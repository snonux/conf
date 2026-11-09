#!/bin/ksh
# Destructive operations are mocked; all filesystem paths are rewritten into a
# disposable root. Run on OpenBSD:
#   ksh rollback.ksh CONTROLLER PROFILE_DIRECTORY NAT66_EGRESS_HELPER
set -eu
set -o pipefail
[[ $# == 3 && $(uname -s) == OpenBSD ]] || exit 2
fixture=$(mktemp -d /tmp/router-rollback-fixture.XXXXXXXX)
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/etc/gonf" "$fixture/var/unbound/etc" "$fixture/var/run/rc.d"
# No production test-mode switch: source only the function section and rewrite
# absolute state paths. This harness never executes the production dispatch.
sed '/^# Dispatch:/,$d' "$1" | sed "s@/var/unbound/etc/@__UNBOUND__@g; s@/etc/@$fixture/etc/@g; s@/var/@$fixture/var/@g; s@__UNBOUND__@$fixture/var/unbound/etc/@g; s@/bsd.upgrade@$fixture/bsd.upgrade@g" > "$fixture/functions"
cp -Rp "$2" "$fixture/etc/gonf/router-profiles"
cat > "$fixture/mocks" <<'MOCKS'
# Real parser validation remains safe; mutation calls below only update records.
pfctl() {
 case $* in
 '-nf '*) /sbin/pfctl "$@";;
 # Candidate validation reads the real expansion (optimizer off).
 '-o none -nvf '*) /sbin/pfctl "$@";;
 '-nvf '*) print 'set skip on { lo }'; awk '/^table / {print}' "$2"; print 'pass all';;
 '-s Anchors') :;;
 '-s Tables')
  for table in "$BASE/pf-tables/"*; do
   [[ ! -f $table ]] || print -- "${table##*/}"
  done
  return 0;;
 '-s info') print 'Status: Enabled';;
 '-s Interfaces -v') print 'lo (skip)'; print 'lo0 (skip)';;
 '-sr')
  print 'pass all'
  if [[ -f $BASE/pf-tables/__automatic_aabb_0 && ! -f $BASE/unreferenced-auto ]]; then
   print 'pass in inet from <__automatic_aabb_0> to any'
  fi;;
 '-vvs Tables')
  for table in "$BASE/pf-tables/"*; do
   [[ -f $table ]] || continue
   if [[ ${table##*/} == __automatic_* && ! -f $BASE/mutable-auto ]]; then
    print -- "c-a-r-- ${table##*/}"
   else print -- "-pa-r-- ${table##*/}"; fi
  done;;
 '-f '*)
  print -- "pfctl $*" >> "$BASE/operations"
  if [[ $2 == "$STATE/candidate/pf.conf" ]]; then
   # Local-only candidates deny RA/RS; the o34 uplink stages pass the CPE's.
   if grep -Eq '^pass in quick on [$]up6_if inet6 proto icmp6 from fe80::1 .*icmp6-type routeradv$' "$2"; then
    rm -f "$BASE/pf-blocks-ra"
   else touch "$BASE/pf-blocks-ra"; fi
   mkdir -p "$BASE/pf-tables"
   touch "$BASE/pf-tables/recorders" "$BASE/pf-tables/camera_updates"
   if grep -qx 'table <nat66_egress> persist' "$2"; then touch "$BASE/pf-tables/nat66_egress"; fi
  else rm -f "$BASE/pf-blocks-ra"; fi;;
 '-t '*)
  case $4 in
  show) cat "$BASE/pf-tables/$2";;
  kill) rm "$BASE/pf-tables/$2";;
  replace) cp "$6" "$BASE/pf-tables/$2";;
  *) return 1;;
  esac;;
 *) print -- "pfctl $*" >> "$BASE/operations";;
 esac
}
ifconfig() {
 typeset file=$BASE/interfaces/$1
 if [[ $# == 1 ]]; then cat "$file"; return; fi
 print -- "ifconfig $*" >> "$BASE/operations"
 typeset i=$1 family=$2
 shift 2
 case $family in
 create)
  [[ ! -f $file ]] || return 1
  print -- "$i: flags=<UP>" > "$file"
  [[ ${1:-} != description ]] || print -- " description: $2" >> "$file"
  return 0;;
 destroy) rm -f "$file"; return 0;;
 up|down) return 0;;
 -inet6)
  awk '$1!="inet6"' "$file" > "$file.next"; mv "$file.next" "$file"
  return 0;;
 inet|inet6)
  if [[ $1 == -autoconf || $1 == autoconf || $1 == -temporary || $1 == temporary ]]; then
   typeset opt flag
   for opt in "$@"; do
    case $opt in *temporary) flag=AUTOCONF6TEMP;; *) flag=AUTOCONF6;; esac
    sed "s/$flag, UP/UP/; s/$flag,//g; s/,$flag>/>/g" "$file" > "$file.next"
    mv "$file.next" "$file"
    if [[ $opt != -* ]]; then
     sed "s/flags=</flags=<$flag,/" "$file" > "$file.next"; mv "$file.next" "$file"
    fi
   done
   return 0
  fi
  typeset address=$1
  [[ $1 != alias ]] || { shift; address=$1; }
  if [[ $* == *" delete" ]]; then
   awk -v a="$address" '$2!=a' "$file" > "$file.next"
  elif [[ $family == inet ]]; then
   awk -v a="$address" '$2!=a' "$file" > "$file.next"
   print -- " inet $address netmask 0xffffff00" >> "$file.next"
  else
   awk -v a="$address" '$2!=a' "$file" > "$file.next"
   print -- " inet6 $address prefixlen 64" >> "$file.next"
  fi
  mv "$file.next" "$file";;
 esac
}
route() {
 if [[ $* == '-n show -inet' ]]; then
  if [[ ! -f $BASE/route-failure && ( ! -f $BASE/route-race || -f $BASE/route-added ) ]]; then
   print 'default 192.168.1.1 UGS 0 0 - 8 igc0'
  fi
  return 0
 fi
 if [[ $* == '-n show -inet6' ]]; then return; fi
 if [[ $* == '-n add '* && -f $BASE/route-race ]]; then touch "$BASE/route-added"; return 1; fi
 if [[ $* == '-n add '* && -f $BASE/route-failure ]]; then return 1; fi
 print -- "route $*" >> "$BASE/operations"
}
sysctl() {
 case $* in
 '-n net.inet.ip.forwarding') print 1;;
 '-n net.inet6.ip6.forwarding') print 0;;
 *) print -- "sysctl $*" >> "$BASE/operations";;
 esac
}
rcctl() {
 print -- "rcctl $*" >> "$BASE/operations"
 case $* in
 'check cron'|'check ntpd') return 0;;
 'check resolvd') [[ ! -f $BASE/resolvd-stopped ]];;
 'check '*) return 1;;
 'start unbound') [[ ! -f $BASE/fail-activate ]];;
 'start ntpd') grep -qx 'nameserver 127.0.0.1' "${BASE%/gonf}/resolv.conf";;
 '-f start resolvd') print 'nameserver 192.0.2.99' > "${BASE%/gonf}/resolv.conf";;
 *) return 0;;
 esac
}
crontab() {
 print -- "* * * * * $SELF tick"
 [[ -f $BASE/no-nat66-cron ]] || print -- "* * * * * $BASE/nat66-egress"
}
pgrep() {
 if [[ $* == '-x (sysupgrade|syspatch|pkg_add)' && -f $BASE/upgrade-running ]]; then print 12346; return 0; fi
 if [[ $* == '-x dhcpd' ]]; then [[ ! -f $BASE/dhcpd-down ]] && print 4242; return; fi
 [[ $* == '-xf /sbin/resolvd' && -f $BASE/native-resolvd ]] || return 1
 print 12345
}
# dhcpd holds one bpf device per served port. dhcpd-dropped-port simulates
# c24 review B1: a flagged port without a subnet for its first address.
fstat() {
 typeset n
 n=$(($(sed -n 's/^dhcpd_interfaces=//p' "$STATE/candidate/services.conf" | wc -w)))
 [[ ! -f $BASE/dhcpd-dropped-port ]] || n=$((n - 1))
 print 'USER     CMD          PID   FD MOUNT        INUM MODE         R/W    SZ|DV'
 while (( n > 0 )); do print "_dhcp    dhcpd       4242    $((n + 3)) clone       1 crw-------     rw     bpf$n"; n=$((n - 1)); done
}
shutdown() {
 [[ $* == '-r now' ]] || return 1
 print 'shutdown requested' > "$BASE/reboot-requested"
}
netstat() { cat "$BASE/sockets"; }
sh() {
 [[ $1 == "${BASE%/gonf}/netstart" && $2 == wg2 ]] || return 1
 print ' inet 10.99.0.1 netmask 0xffffff00' >> "$BASE/interfaces/wg2"
}
sync() { :; }
slaacctl() {
 print -- "slaacctl $*" >> "$BASE/operations"
 [[ $1 == send && $2 == solicitation ]] || return 1
 # Real candidate PF denies RA/RS. An early solicitation cannot recover SLAAC.
 [[ ! -f $BASE/pf-blocks-ra ]] || return 1
 [[ ! -f $BASE/slaac-fail ]] || return 0
 [[ ! -f $BASE/slaac-addresses ]] || cat "$BASE/slaac-addresses" >> "$BASE/interfaces/$3"
}
MOCKS
# Each invocation is a separate native shell with errexit enabled, so a parent
# conditional cannot accidentally disable failure propagation in the worker.
cat > "$fixture/run" <<EOF2
#!/bin/ksh
. "$fixture/functions"
. "$fixture/mocks"
"\$@"
EOF2
chmod 700 "$fixture/run"
base=$fixture/etc/gonf
mkdir -p "$base/interfaces" "$base/router-readiness"
for i in igc0 igc1 igc2 igc3; do print -- "$i: flags=<UP,BROADCAST>" > "$base/interfaces/$i"; done
print ' inet 192.168.1.2 netmask 0xffffff00' >> "$base/interfaces/igc0"
# Deliberate persistent/runtime drift: .11.2 is persisted but absent live.
print 'inet 192.168.1.2 255.255.255.0\ninet alias 192.168.11.2 255.255.255.0' > "$fixture/etc/hostname.igc0"
for i in igc1 igc2 igc3; do print up > "$fixture/etc/hostname.$i"; done
print 192.168.1.1 > "$fixture/etc/mygate"
print 'set state-policy floating\npass all' > "$fixture/etc/pf.conf"
print 'net.inet.ip.forwarding=1\nnet.inet6.ip6.forwarding=0\nkern.maxfiles=12345' > "$fixture/etc/sysctl.conf"
print 'pkg_scripts=uptimed\nresolvd_flags=""' > "$fixture/etc/rc.conf.local"
print 'nameserver fe80::1%igc0\nnameserver fd22:c702:acb7::1' > "$fixture/etc/resolv.conf"
chmod 644 "$fixture/etc/resolv.conf"
cp -p "$fixture/etc/resolv.conf" "$fixture/resolv.expected"
print 'daemon_flags=' > "$fixture/var/run/rc.d/ntpd"
print 'daemon_flags=-a fixture-runtime-drift' > "$fixture/var/run/rc.d/resolvd"
print 'wg2: flags=<UP>\n inet 10.99.0.1 netmask 0xffffff00' > "$base/interfaces/wg2"
# Not keys: this synthetic fixture tests only the fail-closed presence gate.
{
 # A deterministic invalid cryptographic fixture, never a provisioned key.
 printf 'wgkey %043d=\n' 0
 printf 'wgpeer %043d= wgaip 10.99.0.2/32\n' 0
 printf 'wgpeer %043d= wgaip 10.99.0.3/32\n' 0
} > "$fixture/etc/hostname.wg2"
print '' > "$base/sockets"
cat > "$fixture/etc/rc" <<'RC'
#!/bin/ksh
mount -uw /
rootdisk_nodes
sysctl_conf
sh /etc/netstart
RC
# install_boot is deliberately exercised only against the rewritten fixture rc.
"$fixture/run" install_boot
/bin/ksh -n "$fixture/etc/rc"
# Existing bootstrap hooks are upgraded only when their legacy form matches.
sed 's/ rad_flags resolvd_flags pf$/ rad_flags pf/' "$fixture/etc/rc" > "$fixture/etc/rc.next"
mv "$fixture/etc/rc.next" "$fixture/etc/rc"
"$fixture/run" install_boot
grep -qx ' unset dhcpd_flags unbound_flags ntpd_flags rad_flags resolvd_flags pf' "$fixture/etc/rc"
"$fixture/run" stage trial0001 shared-igc0 180
# Produce exact reviewed candidate binding as a controller would, then verify
# both missing readiness and missing keys block arm before any network writes.
expect_fail() {
 if "$fixture/run" "$@" > "$fixture/failure.log" 2>&1; then
  print -u2 -- "unexpected success: $*"; exit 1
 fi
}
# A correct gateway may appear between query and add; an actual failed add
# without that exact route must still fail instead of reporting recovery.
touch "$base/route-race"
"$fixture/run" restore_gateway_route inet default 192.168.1.1 UGS 8
[[ -f $base/route-added ]]
rm "$base/route-race" "$base/route-added"
touch "$base/route-failure"
expect_fail restore_gateway_route inet default 192.168.1.1 UGS 8
rm "$base/route-failure"
# A corrupt cached-state reparse must not be silently accepted or upgraded.
cp -p "$fixture/etc/rc" "$fixture/rc.expected"
sed '/^ _rc_parse_conf$/d' "$fixture/etc/rc" > "$fixture/etc/rc.next"
mv "$fixture/etc/rc.next" "$fixture/etc/rc"
expect_fail install_boot
cp -p "$fixture/rc.expected" "$fixture/etc/rc"
expect_fail arm trial0001 shared-igc0 180
# The fixture root is mode 700 and the review is owned by the invoking root.
{
 print -- "candidate-sha256=$(cat "$base/router-state/digest")"
 print 'provisioning-clients=captured-inventory-no-known-clients'
 print 'dependencies=r14-reviewed-current-stage-only'
 print 'wg2=provisioned-and-tested'
 print 'old-management=fresh-authenticated'
 print 'current-cabling=shared-igc0'
} > "$base/router-readiness/shared-igc0"
chmod 600 "$base/router-readiness/shared-igc0"
mv "$fixture/etc/hostname.wg2" "$fixture/etc/wg2.saved"
expect_fail arm trial0001 shared-igc0 180
mv "$fixture/etc/wg2.saved" "$fixture/etc/hostname.wg2"
"$fixture/run" arm trial0001 shared-igc0 180
[[ -f $base/router-owner && -f $base/router-state/pending ]]
# Failure after addresses/files changed leaves a durable pending marker; the
# next OS timer invocation restores runtime drift, files and unrelated sysctls.
touch "$base/fail-activate"
expect_fail activate trial0001
[[ $(cat "$base/router-state/status") == activating ]]
print 0 > "$base/router-state/deadline"
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored ]]
if grep -q '192.168.11.2' "$base/interfaces/igc0"; then exit 1; fi
grep -q '192.168.11.2' "$fixture/etc/hostname.igc0"
grep -qx 'kern.maxfiles=12345' "$fixture/etc/sysctl.conf"
cmp -s "$fixture/resolv.expected" "$fixture/etc/resolv.conf"
[[ $(stat -f %Lp "$fixture/etc/resolv.conf") == 644 ]]
grep -qx 'resolvd_flags=""' "$fixture/etc/rc.conf.local"
grep -qx 'rcctl set resolvd flags -a fixture-runtime-drift' "$base/operations"
grep -qx 'rcctl -f start resolvd' "$base/operations"
"$fixture/run" restore
cmp -s "$fixture/resolv.expected" "$fixture/etc/resolv.conf"
[[ -f $base/router-owner && ! -f $base/router-state/pending ]]
rm "$base/fail-activate"
# Persistent PF tables survive loading old rules; restore must remove only
# candidate-created names and retain the captured table and its contents.
print 'table <existing> persist' >> "$fixture/etc/pf.conf"
print '192.0.2.10' > "$base/pf-tables/existing"
"$fixture/run" stage trialtbl1 shared-igc0 180
"$fixture/run" arm trialtbl1 shared-igc0 180
"$fixture/run" activate trialtbl1
print '198.51.100.10' > "$base/pf-tables/existing"
touch "$base/pf-tables/foreign"
expect_fail restore
[[ -f $base/router-state/pending && -f $base/pf-tables/foreign ]]
rm "$base/pf-tables/foreign"
"$fixture/run" restore
grep -qx '192.0.2.10' "$base/pf-tables/existing"
[[ ! -e $base/pf-tables/recorders && ! -e $base/pf-tables/camera_updates ]]
# A refused later stage must not strand the last restored snapshot. Recovery
# checks its token/integrity and archives the inactive stage before replaying.
"$fixture/run" stage trialtbl2 shared-igc0 180
expect_fail recover_previous wrongtoken
"$fixture/run" recover_previous trialtbl1
[[ $(cat "$base/router-state/token") == trialtbl1 ]]
[[ -d $base/router-rejected-trialtbl2 && ! -f $base/router-state/pending ]]
rm "$base/pf-tables/existing"
sed '/^table <existing> persist$/d' "$fixture/etc/pf.conf" > "$fixture/etc/pf.next"
mv "$fixture/etc/pf.next" "$fixture/etc/pf.conf"
# Optimizer-created const tables are part of the running policy. Unknown,
# mutable or unreferenced runtime-only tables still refuse before arming.
print '192.0.2.0/24' > "$base/pf-tables/__automatic_aabb_0"
"$fixture/run" stage trialauto1 shared-igc0 180
touch "$base/mutable-auto"
expect_fail arm trialauto1 shared-igc0 180
grep -q 'mutable optimizer PF table' "$fixture/failure.log"
[[ ! -f $base/router-state/pending ]]
rm "$base/mutable-auto"
touch "$base/unreferenced-auto"
expect_fail arm trialauto1 shared-igc0 180
grep -q 'unreferenced optimizer PF table' "$fixture/failure.log"
rm "$base/unreferenced-auto"
"$fixture/run" arm trialauto1 shared-igc0 180
grep -q '^table <__automatic_aabb_0> const' "$base/router-state/saved/pf.runtime"
/sbin/pfctl -nf "$base/router-state/saved/pf.runtime"
"$fixture/run" activate trialauto1
"$fixture/run" restore
grep -qx '192.0.2.0/24' "$base/pf-tables/__automatic_aabb_0"
rm "$base/pf-tables/__automatic_aabb_0"
# Unsupported resolver symlinks refuse before pending authorizes activation.
"$fixture/run" stage trialdns1 shared-igc0 180
mv "$fixture/etc/resolv.conf" "$fixture/resolv.saved"
ln -s "$fixture/resolv.saved" "$fixture/etc/resolv.conf"
expect_fail arm trialdns1 shared-igc0 180
[[ ! -f $base/router-state/pending ]]
rm "$fixture/etc/resolv.conf"
ln -s "$fixture/absent-resolver-target" "$fixture/etc/resolv.conf"
expect_fail arm trialdns1 shared-igc0 180
[[ ! -f $base/router-state/pending ]]
rm "$fixture/etc/resolv.conf"
mv "$fixture/resolv.saved" "$fixture/etc/resolv.conf"
# Native boot can start resolvd without an rc.d runtime cache. Unknown process
# arguments refuse, while the exact no-argument daemon captures empty flags.
mv "$fixture/var/run/rc.d/resolvd" "$fixture/resolvd.runtime.saved"
expect_fail arm trialdns1 shared-igc0 180
[[ ! -f $base/router-state/pending ]]
touch "$base/native-resolvd"
"$fixture/run" arm trialdns1 shared-igc0 180
[[ ! -s $base/router-state/saved/services/resolvd.flags || $(cat "$base/router-state/saved/services/resolvd.flags") == '' ]]
"$fixture/run" restore
rm "$base/native-resolvd"
mv "$fixture/resolvd.runtime.saved" "$fixture/var/run/rc.d/resolvd"
# An absent file and a stopped resolver must round-trip without starting it.
touch "$base/resolvd-stopped"
rm "$fixture/etc/resolv.conf"
"$fixture/run" stage trialdns2 shared-igc0 180
"$fixture/run" arm trialdns2 shared-igc0 180
"$fixture/run" activate trialdns2
: > "$base/operations"
"$fixture/run" restore
[[ ! -e $fixture/etc/resolv.conf ]]
if grep -qx 'rcctl -f start resolvd' "$base/operations"; then exit 1; fi
rm "$base/resolvd-stopped"
cp -p "$fixture/resolv.expected" "$fixture/etc/resolv.conf"
"$fixture/run" stage trial0002 shared-igc0 180
"$fixture/run" arm trial0002 shared-igc0 180
"$fixture/run" activate trial0002
grep -qx 'nameserver 127.0.0.1' "$fixture/etc/resolv.conf"
grep -qx 'rcctl stop resolvd' "$base/operations"
grep -qx 'rcctl disable resolvd' "$base/operations"
expect_fail confirm trial0002 services-checked
# Existing SSH tuple must be rejected, even when its socket is still present.
print '192.168.1.236 23456 192.168.1.2 22' > "$base/router-state/preexisting-connections"
print 'tcp 0 0 192.168.1.2.22 192.168.1.236.23456 ESTABLISHED' > "$base/sockets"
expect_fail proof old trial0002 '192.168.1.236 23456 192.168.1.2 22'
print 'tcp 0 0 192.168.1.2.22 192.168.1.236.34567 ESTABLISHED' > "$base/sockets"
"$fixture/run" proof old trial0002 '192.168.1.236 34567 192.168.1.2 22'
print 'tcp 0 0 10.20.0.1.22 10.20.0.250.34568 ESTABLISHED' > "$base/sockets"
"$fixture/run" proof new trial0002 '10.20.0.250 34568 10.20.0.1 22'
expect_fail confirm trial0002
"$fixture/run" confirm trial0002 services-checked
expect_fail restore
# A reboot while pending restores files BEFORE the native rc caller reparses
# defaults. No rc.local/firsttime late-hook or previous-boot lock is sufficient.
"$fixture/run" stage trial0003 shared-igc0 180
"$fixture/run" arm trial0003 shared-igc0 180
"$fixture/run" activate trial0003
"$fixture/run" boot_restore
[[ ! -f $base/router-state/pending ]]
[[ $(cat "$base/router-state/status") == restored-at-boot ]]
# Execute the actual inserted hook with a cached value that the restored
# rc.conf.local omits. Resetting only persisted keys would leave it enabled.
cat > "$fixture/boot-cache" <<EOF2
#!/bin/ksh
. "$fixture/functions"
. "$fixture/mocks"
_rc_parse_conf() {
 dhcpd_flags=NO; unbound_flags=NO; ntpd_flags=NO; rad_flags=NO; resolvd_flags=; pf=YES
 [[ ! -f "$fixture/etc/rc.conf.local" ]] || . "$fixture/etc/rc.conf.local"
}
unbound_flags=active-cached-value
resolvd_flags=NO
EOF2
# Re-arm the pending marker and invoke the genuine inserted block in isolation.
touch "$base/router-state/pending"
# Redirect the hook's helper to this fixture dispatcher, retaining its logic.
cat > "$base/router-rollback" <<EOF2
#!/bin/ksh
. "$fixture/functions"
. "$fixture/mocks"
case \$1 in boot-needed) pending;; boot-restore) boot_restore;; esac
EOF2
chmod 700 "$base/router-rollback"
sed -n '/^# BEGIN gonf router recovery$/,/^# END gonf router recovery$/p' "$fixture/etc/rc" >> "$fixture/boot-cache"
# The child shell, rather than this harness, expands these variables.
# shellcheck disable=SC2016
print '[[ $unbound_flags == NO && $resolvd_flags == "" && $pf == YES ]]' >> "$fixture/boot-cache"
/bin/ksh "$fixture/boot-cache"
# Missing interface placements, template activation, changed frozen bytes and
# unlisted dependency readiness must refuse without applying anything.
"$fixture/run" stage trial0004 shared-igc0 180
print 'extra=unknown' >> "$base/router-readiness/shared-igc0"
expect_fail arm trial0004 shared-igc0 180
sed '$d' "$base/router-readiness/shared-igc0" > "$base/review.next"
mv "$base/review.next" "$base/router-readiness/shared-igc0"
chmod 600 "$base/router-readiness/shared-igc0"
print '# changed after freeze' >> "$base/router-state/candidate/pf.conf"
expect_fail arm trial0004 shared-igc0 180
"$fixture/run" stage trial0005 shared-igc0 180
print 'TEMPLATE' >> "$fixture/etc/hostname.wg2"
expect_fail arm trial0005 shared-igc0 180
sed '$d' "$fixture/etc/hostname.wg2" > "$fixture/etc/wg2.next"
mv "$fixture/etc/wg2.next" "$fixture/etc/hostname.wg2"
# On post-rack rollback, recovery is never deleted or moved to upstream. The
# snapshot is the last accepted shared profile with recovery already on igc1.
"$fixture/run" stage trial0006 post-rack 180
{
 print -- "candidate-sha256=$(cat "$base/router-state/digest")"
 print 'provisioning-clients=captured-inventory-no-known-clients'
 print 'dependencies=r14-reviewed-current-stage-only'
 print 'wg2=provisioned-and-tested'
 print 'old-management=fresh-authenticated'
 print 'current-cabling=post-rack'
} > "$base/router-readiness/post-rack"
chmod 600 "$base/router-readiness/post-rack"
"$fixture/run" arm trial0006 post-rack 180
"$fixture/run" activate trial0006
# Discard older fixture operations, then inspect the whole restore action.
: > "$base/operations"
"$fixture/run" restore
if grep -E 'ifconfig igc1 inet 10[.]98[.]0[.]1 delete|ifconfig igc[023].*10[.]98[.]0[.]1' "$base/operations"; then exit 1; fi
grep -q 'inet 10.98.0.1 ' "$base/interfaces/igc1"
# Native IPv6 ownership: restore static aliases, relearn managed prefixes and
# remove a link-local address from a family absent in the captured baseline.
print 'igc0: flags=<AUTOCONF6,AUTOCONF6TEMP,UP,BROADCAST>' > "$base/interfaces/igc0"
{
 print ' inet 192.168.1.2 netmask 0xffffff00'
 print ' inet6 fe80::1 prefixlen 64'
 print ' inet6 2001:db8:1:0::1234 prefixlen 64 autoconf temporary pltime 100 vltime 200'
 print ' inet6 fd02:ca0b:f1cb:20::99 prefixlen 64'
} >> "$base/interfaces/igc0"
print ' inet6 2001:db8:1::5678 prefixlen 64 autoconf temporary pltime 100 vltime 200' > "$base/slaac-addresses"
"$fixture/run" stage ipv6trial shared-igc0 180
"$fixture/run" arm ipv6trial shared-igc0 180
"$fixture/run" activate ipv6trial
# An address proposal surviving a late event refuses acceptance.
print ' inet6 2001:db8:2::bad prefixlen 64' >> "$base/interfaces/igc0"
expect_fail check_candidate_ipv6
print ' inet6 fe80::2 prefixlen 64' >> "$base/interfaces/igc2"
touch "$base/slaac-fail"
expect_fail restore
[[ -f $base/router-state/pending ]]
rm "$base/slaac-fail"
: > "$base/operations"
"$fixture/run" restore
grep -q 'inet6 fd02:ca0b:f1cb:20::99 prefixlen 64' "$base/interfaces/igc0"
grep -q 'inet6 2001:db8:1::5678.*autoconf temporary' "$base/interfaces/igc0"
if grep -q '2001:db8:1:0::1234' "$base/interfaces/igc0"; then exit 1; fi
if grep -q 'inet6 ' "$base/interfaces/igc2"; then exit 1; fi
grep -q AUTOCONF6TEMP "$base/interfaces/igc0"
grep -q 'AUTOCONF6,' "$base/interfaces/igc0"
# Even replay of a restored journal must recover independently on failure.
[[ ! -f $base/router-state/pending ]]
touch "$base/slaac-fail"
expect_fail restore
[[ -f $base/router-state/pending ]]
rm "$base/slaac-fail"
print 0 > "$base/router-state/deadline"
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored && ! -f $base/router-state/pending ]]
# Router reboot canaries share the durable journal and early hook.
# The reboot action requires a genuinely changed fixed canary, a long live
# deadline, no upgrade and three currently established management sockets.
print 'inet alias 10.20.0.254 255.255.255.0' >> "$base/router-profiles/shared-igc0/hostname.igc0"
print '# gonf boot canary: 10.20.0.254' >> "$base/router-profiles/shared-igc0/ntpd.conf"
"$fixture/run" stage boottrial1 shared-igc0 1800
sed "1s/.*/candidate-sha256=$(cat "$base/router-state/digest")/" "$base/router-readiness/shared-igc0" > "$base/review.next"
mv "$base/review.next" "$base/router-readiness/shared-igc0"
chmod 600 "$base/router-readiness/shared-igc0"
"$fixture/run" arm boottrial1 shared-igc0 1800
"$fixture/run" activate boottrial1
expect_fail reboot_canary wrongtoken
touch "$fixture/bsd.upgrade"
expect_fail reboot_canary boottrial1
grep -q 'upgrade kernel is queued' "$fixture/failure.log"
rm "$fixture/bsd.upgrade"
touch "$base/upgrade-running"
expect_fail reboot_canary boottrial1
grep -q 'unattended upgrade is running' "$fixture/failure.log"
rm "$base/upgrade-running"
print "$(($(date +%s) + 10))" > "$base/router-state/deadline"
expect_fail reboot_canary boottrial1
grep -q 'insufficient reboot recovery window' "$fixture/failure.log"
print "$(($(date +%s) + 1800))" > "$base/router-state/deadline"
expect_fail reboot_canary boottrial1
grep -q 'management proofs missing' "$fixture/failure.log"
[[ ! -f $base/reboot-requested ]]
print '' > "$base/router-state/preexisting-connections"
cat > "$base/sockets" <<'SOCKETS'
tcp 0 0 192.168.1.2.22 192.168.1.236.45681 ESTABLISHED
tcp 0 0 10.20.0.1.22 10.99.0.2.45682 ESTABLISHED
tcp 0 0 10.99.0.1.22 10.99.0.2.45683 ESTABLISHED
SOCKETS
"$fixture/run" proof old boottrial1 '192.168.1.236 45681 192.168.1.2 22'
"$fixture/run" proof new boottrial1 '10.99.0.2 45682 10.20.0.1 22'
"$fixture/run" proof vpn boottrial1 '10.99.0.2 45683 10.99.0.1 22'
cp -p "$fixture/etc/ntpd.conf" "$fixture/canary-ntpd.expected"
print '# external edit' >> "$fixture/etc/ntpd.conf"
expect_fail reboot_canary boottrial1
grep -q 'live boot canary ntpd file changed' "$fixture/failure.log"
cp -p "$fixture/canary-ntpd.expected" "$fixture/etc/ntpd.conf"
cp -p "$base/sockets" "$fixture/boot-sockets.expected"
sed '/10\.99\.0\.1\.22/d' "$base/sockets" > "$base/sockets.next"
mv "$base/sockets.next" "$base/sockets"
expect_fail reboot_canary boottrial1
grep -q 'fresh connection socket not found' "$fixture/failure.log"
[[ ! -f $base/reboot-requested ]]
cp -p "$fixture/boot-sockets.expected" "$base/sockets"
"$fixture/run" reboot_canary boottrial1
[[ -f $base/reboot-requested && -f $base/router-state/pending ]]
"$fixture/run" boot_restore
if grep -q '10.20.0.254' "$fixture/etc/hostname.igc0" "$fixture/etc/ntpd.conf"; then exit 1; fi
[[ $(cat "$base/router-state/status") == restored-at-boot && ! -f $base/router-state/pending ]]
# o34 IPv6 stages: uplink SLAAC first, then NAT66 on top of the confirmed
# uplink. Start from a wifi-first runtime and persistent baseline.
# The optional second argument is the old-management value; c24's lab-igc1
# records isp-uplink-only (after b24 no host reaches 192.168.1.2).
review() {
 {
  print -- "candidate-sha256=$(cat "$base/router-state/digest")"
  print 'provisioning-clients=captured-inventory-no-known-clients'
  print 'dependencies=r14-reviewed-current-stage-only'
  print 'wg2=provisioned-and-tested'
  print -- "old-management=${2:-fresh-authenticated}"
  print -- "current-cabling=$1"
 } > "$base/router-readiness/$1"
 chmod 600 "$base/router-readiness/$1"
}
confirm_fresh() {
 print '' > "$base/router-state/preexisting-connections"
 print "tcp 0 0 192.168.1.2.22 192.168.1.236.$2 ESTABLISHED" > "$base/sockets"
 "$fixture/run" proof old "$1" "192.168.1.236 $2 192.168.1.2 22"
 print "tcp 0 0 10.20.0.1.22 10.99.0.2.$3 ESTABLISHED" > "$base/sockets"
 "$fixture/run" proof new "$1" "10.99.0.2 $3 10.20.0.1 22"
 "$fixture/run" confirm "$1" services-checked
}
for i in igc0 igc1 igc2 igc3; do print -- "$i: flags=<UP,BROADCAST>" > "$base/interfaces/$i"; done
{
 print ' inet 192.168.1.2 netmask 0xffffff00'
 print ' inet 10.20.0.1 netmask 0xffffff00'
 print ' inet6 fe80::6662:66ff:fe25:ced9%igc0 prefixlen 64 scopeid 0x1'
 print ' inet6 fd02:ca0b:f1cb:20::1 prefixlen 64'
} >> "$base/interfaces/igc0"
print ' inet 10.98.0.1 netmask 0xffffff00' >> "$base/interfaces/igc1"
print ' inet 10.30.0.1 netmask 0xffffff00' >> "$base/interfaces/igc2"
for i in igc0 igc1 igc2 igc3; do cp "$base/router-profiles/wifi-first/hostname.$i" "$fixture/etc/hostname.$i"; done
rm -f "$base/reboot-requested" "$base/nat66-egress"
up=$base/router-profiles/wifi-ipv6-uplink
nat=$base/router-profiles/wifi-ipv6-nat66
# Every misplaced o34 element refuses at stage, before any journal change.
cp -Rp "$nat" "$fixture/nat66.saved"
cp -Rp "$up" "$fixture/uplink.saved"
cp -p "$base/router-profiles/wifi-first/hostname.igc0" "$fixture/wifi-first-igc0.saved"
o34_refuses() {
 expect_fail stage o34bad01 "$1" 180
 grep -q "$2" "$fixture/failure.log" || { print -u2 -- "wrong refusal for $1: $2"; cat "$fixture/failure.log" >&2; exit 1; }
 rm -rf "$nat" "$up"
 cp -Rp "$fixture/nat66.saved" "$nat"
 cp -Rp "$fixture/uplink.saved" "$up"
}
print 'interface igc0' >> "$nat/rad.conf"
o34_refuses wifi-ipv6-nat66 'rad must serve only igc2'
sed '/^table <nat66_egress> persist$/d' "$nat/pf.conf" > "$nat/pf.next"; mv "$nat/pf.next" "$nat/pf.conf"
o34_refuses wifi-ipv6-nat66 'NAT66 egress table missing'
# A NAT match after the quick non-global egress block never translates.
awk '/nat-to <nat66_egress>$/ {held=$0; next} {print} /^pass in quick on \$wifi_if inet6 from \$wifi6/ {print held}' "$nat/pf.conf" > "$nat/pf.next"; mv "$nat/pf.next" "$nat/pf.conf"
grep -q 'nat-to <nat66_egress>$' "$nat/pf.conf"
o34_refuses wifi-ipv6-nat66 'NAT66 translation must precede the quick uplink rules'
sed '/f1cb:30::1/d' "$nat/hostname.igc2" > "$nat/igc2.next"; mv "$nat/igc2.next" "$nat/hostname.igc2"
print 'inet6 alias fd02:ca0b:f1cb:30::1 64' >> "$nat/hostname.igc0"
o34_refuses wifi-ipv6-nat66 'Wi-Fi ULA only on igc2'
sed '/^inet6 autoconf -temporary$/d' "$nat/hostname.igc0" > "$nat/igc0.next"; mv "$nat/igc0.next" "$nat/hostname.igc0"
print 'inet6 autoconf -temporary' >> "$nat/hostname.igc2"
o34_refuses wifi-ipv6-nat66 'IPv6 autoconfiguration only on the igc0 uplink'
print 'net.inet.ip.forwarding=1\nnet.inet6.ip6.forwarding=0' > "$nat/sysctl.conf.fragment"
o34_refuses wifi-ipv6-nat66 'unexpected sysctl fragment'
print 'net.inet.ip.forwarding=1\nnet.inet6.ip6.forwarding=1' > "$up/sysctl.conf.fragment"
o34_refuses wifi-ipv6-uplink 'unexpected sysctl fragment'
cp -p "$nat/rad.conf" "$up/rad.conf"
o34_refuses wifi-ipv6-uplink 'rad configuration outside the NAT66 profile'
print 'inet6 alias fd02:ca0b:f1cb:30::1 64' >> "$up/hostname.igc2"
o34_refuses wifi-ipv6-uplink 'unexpected Wi-Fi ULA'
sed -e 's/^inet6 -autoconf$/inet6 autoconf -temporary/' -e '/^inet6 -temporary$/d' "$fixture/wifi-first-igc0.saved" > "$base/router-profiles/wifi-first/hostname.igc0"
o34_refuses wifi-first 'unexpected IPv6 autoconfiguration line'
cp -p "$fixture/wifi-first-igc0.saved" "$base/router-profiles/wifi-first/hostname.igc0"
# A temporary SLAAC address after activation refuses acceptance; the timer
# then restores the local-only baseline, including the persistent policy.
print ' inet6 2001:db8:5::77 prefixlen 64 autoconf temporary pltime 100 vltime 200' > "$base/slaac-addresses"
"$fixture/run" stage o34upl01 wifi-ipv6-uplink 180
review wifi-ipv6-uplink
"$fixture/run" arm o34upl01 wifi-ipv6-uplink 180
expect_fail activate o34upl01
grep -q 'unexpected physical IPv6 address' "$fixture/failure.log"
[[ $(cat "$base/router-state/status") == activating ]]
print 0 > "$base/router-state/deadline"
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored && ! -f $base/router-state/pending ]]
if grep -q -e AUTOCONF6 -e 2001:db8:5 "$base/interfaces/igc0"; then exit 1; fi
grep -q 'inet6 fd02:ca0b:f1cb:20::1 prefixlen 64' "$base/interfaces/igc0"
grep -qx 'inet6 -autoconf' "$fixture/etc/hostname.igc0"
grep -qx 'inet6 -temporary' "$fixture/etc/hostname.igc0"
# Uplink stage: stable SLAAC address and solicitation only after its PF.
print ' inet6 2001:db8:5::7 prefixlen 64 autoconf pltime 100 vltime 200' > "$base/slaac-addresses"
"$fixture/run" stage o34upl02 wifi-ipv6-uplink 180
review wifi-ipv6-uplink
"$fixture/run" arm o34upl02 wifi-ipv6-uplink 180
: > "$base/operations"
"$fixture/run" activate o34upl02
grep -qx 'ifconfig igc0 inet6 autoconf -temporary' "$base/operations"
grep -qx 'sysctl net.inet.ip.forwarding=1 net.inet6.ip6.forwarding=0' "$base/operations"
grep -qx 'rcctl disable rad' "$base/operations"
if grep -q -e 'rcctl start rad' -e 'slaacctl send solicitation igc[123]' "$base/operations"; then exit 1; fi
awk '/^pfctl -f .*candidate\/pf.conf$/ {pf=NR} /^slaacctl send solicitation igc0$/ {rs=NR} END {exit !(pf && rs > pf)}' "$base/operations"
grep -q 'AUTOCONF6,' "$base/interfaces/igc0"
grep -q 'inet6 2001:db8:5::7 .*autoconf' "$base/interfaces/igc0"
grep -qx 'inet6 autoconf -temporary' "$fixture/etc/hostname.igc0"
[[ ! -e $fixture/etc/rad.conf ]]
cp -p "$base/interfaces/igc0" "$fixture/igc0.active"
print ' inet6 2001:db8:5::99 prefixlen 64 autoconf temporary pltime 100 vltime 200' >> "$base/interfaces/igc0"
expect_fail check_candidate_ipv6
cp -p "$fixture/igc0.active" "$base/interfaces/igc0"
sed 's/flags=</flags=<AUTOCONF6TEMP,/' "$fixture/igc0.active" > "$base/interfaces/igc0"
expect_fail check_candidate_ipv6
cp -p "$fixture/igc0.active" "$base/interfaces/igc0"
sed 's/flags=</flags=<AUTOCONF6,/' "$base/interfaces/igc2" > "$fixture/igc2.next"
cp -p "$base/interfaces/igc2" "$fixture/igc2.active"
mv "$fixture/igc2.next" "$base/interfaces/igc2"
expect_fail check_candidate_ipv6
cp -p "$fixture/igc2.active" "$base/interfaces/igc2"
"$fixture/run" check_candidate_ipv6
expect_fail reboot_canary o34upl02
grep -q 'no active boot canary' "$fixture/failure.log"
confirm_fresh o34upl02 46001 46002
# NAT66 refuses to arm without its egress helper and timer.
"$fixture/run" stage o34nat01 wifi-ipv6-nat66 180
review wifi-ipv6-nat66
expect_fail arm o34nat01 wifi-ipv6-nat66 180
grep -q 'NAT66 egress helper not installed' "$fixture/failure.log"
print '#!/bin/ksh' > "$base/nat66-egress"
chmod 700 "$base/nat66-egress"
touch "$base/no-nat66-cron"
expect_fail arm o34nat01 wifi-ipv6-nat66 180
grep -q 'NAT66 egress timer not installed' "$fixture/failure.log"
[[ ! -f $base/router-state/pending ]]
rm "$base/no-nat66-cron"
"$fixture/run" arm o34nat01 wifi-ipv6-nat66 180
: > "$base/operations"
"$fixture/run" activate o34nat01
grep -qx 'rcctl enable rad' "$base/operations"
cmp -s "$nat/rad.conf" "$fixture/etc/rad.conf"
grep -q 'inet6 fd02:ca0b:f1cb:30::1 prefixlen 64' "$base/interfaces/igc2"
grep -qx 'inet6 alias fd02:ca0b:f1cb:30::1 64' "$fixture/etc/hostname.igc2"
grep -qx 'net.inet6.ip6.forwarding=1' "$fixture/etc/sysctl.conf"
grep -qx 'kern.maxfiles=12345' "$fixture/etc/sysctl.conf"
[[ -f $base/pf-tables/nat66_egress ]]
# Forwarding only after the candidate PF, rad only after forwarding.
awk '/^pfctl -f .*candidate\/pf.conf$/ {pf=NR}
 /^sysctl net.inet.ip.forwarding=1 net.inet6.ip6.forwarding=1$/ {fw=NR}
 /^rcctl start rad$/ {rad=NR}
 END {exit !(pf && fw > pf && rad > fw)}' "$base/operations"
# Timed restore returns to the confirmed uplink stage: rad, forwarding, Wi-Fi
# IPv6, rad.conf and the NAT table go; the uplink SLAAC prefix is relearned.
print 0 > "$base/router-state/deadline"
: > "$base/operations"
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored && ! -f $base/router-state/pending ]]
[[ ! -e $fixture/etc/rad.conf && ! -e $base/pf-tables/nat66_egress ]]
grep -qx 'rcctl stop rad' "$base/operations"
grep -qx 'sysctl net.inet.ip.forwarding=1 net.inet6.ip6.forwarding=0' "$base/operations"
grep -qx 'ifconfig igc2 -inet6' "$base/operations"
if grep -q -e 'inet6 ' "$base/interfaces/igc2"; then exit 1; fi
if grep -q 'rcctl -f start rad' "$base/operations"; then exit 1; fi
grep -qx 'slaacctl send solicitation igc0' "$base/operations"
grep -q 'AUTOCONF6,' "$base/interfaces/igc0"
grep -q 'inet6 2001:db8:5::7 .*autoconf' "$base/interfaces/igc0"
grep -qx 'inet6 autoconf -temporary' "$fixture/etc/hostname.igc0"
if grep -q 'f1cb:30' "$fixture/etc/hostname.igc2"; then exit 1; fi
grep -qx 'net.inet6.ip6.forwarding=0' "$fixture/etc/sysctl.conf"
grep -qx 'kern.maxfiles=12345' "$fixture/etc/sysctl.conf"
# Boot restoration of an unconfirmed NAT66 trial restores the persistent
# uplink files before netstart, rc.conf.local and sysctl included.
"$fixture/run" stage o34nat02 wifi-ipv6-nat66 180
review wifi-ipv6-nat66
"$fixture/run" arm o34nat02 wifi-ipv6-nat66 180
"$fixture/run" activate o34nat02
"$fixture/run" boot_restore
[[ $(cat "$base/router-state/status") == restored-at-boot && ! -f $base/router-state/pending ]]
[[ ! -e $fixture/etc/rad.conf ]]
grep -qx 'inet6 autoconf -temporary' "$fixture/etc/hostname.igc0"
if grep -q 'f1cb:30' "$fixture/etc/hostname.igc2"; then exit 1; fi
grep -qx 'net.inet6.ip6.forwarding=0' "$fixture/etc/sysctl.conf"
cmp -s "$base/router-state/saved/files$fixture/etc/rc.conf.local" "$fixture/etc/rc.conf.local"
# A real reboot drops runtime-only state; the mocks keep it, so reset it.
simulated_reboot() {
 cp -p "$fixture/igc0.active" "$base/interfaces/igc0"
 cp -p "$fixture/igc2.active" "$base/interfaces/igc2"
 rm -f "$base/pf-tables/nat66_egress"
}
simulated_reboot
# The reboot canary is accepted for NAT66 and removed again by boot recovery.
print 'inet alias 10.20.0.254 255.255.255.0' >> "$nat/hostname.igc0"
print '# gonf boot canary: 10.20.0.254' >> "$nat/ntpd.conf"
"$fixture/run" stage o34boot1 wifi-ipv6-nat66 1800
review wifi-ipv6-nat66
"$fixture/run" arm o34boot1 wifi-ipv6-nat66 1800
"$fixture/run" activate o34boot1
print '' > "$base/router-state/preexisting-connections"
cat > "$base/sockets" <<'SOCKETS'
tcp 0 0 192.168.1.2.22 192.168.1.236.46011 ESTABLISHED
tcp 0 0 10.20.0.1.22 10.99.0.2.46012 ESTABLISHED
tcp 0 0 10.99.0.1.22 10.99.0.2.46013 ESTABLISHED
SOCKETS
"$fixture/run" proof old o34boot1 '192.168.1.236 46011 192.168.1.2 22'
"$fixture/run" proof new o34boot1 '10.99.0.2 46012 10.20.0.1 22'
"$fixture/run" proof vpn o34boot1 '10.99.0.2 46013 10.99.0.1 22'
"$fixture/run" reboot_canary o34boot1
[[ -f $base/reboot-requested && -f $base/router-state/pending ]]
"$fixture/run" boot_restore
if grep -q '10.20.0.254' "$fixture/etc/hostname.igc0" "$fixture/etc/ntpd.conf"; then exit 1; fi
[[ ! -e $fixture/etc/rad.conf ]]
rm -rf "$nat" && cp -Rp "$fixture/nat66.saved" "$nat"
rm "$base/reboot-requested"
simulated_reboot
# Journals snapshotted before rad.conf ownership omit it from the manifest:
# a local-only or uplink candidate still restores, a NAT66 candidate refuses.
strip_rad_manifest() {
 # rad.conf may be the only absent entry; an empty remainder is valid.
 grep -vxF "$fixture/etc/rad.conf" "$base/router-state/saved/absent" > "$fixture/absent.next" || :
 mv "$fixture/absent.next" "$base/router-state/saved/absent"
 "$fixture/run" atomic "$base/router-state/snapshot-digest" "$("$fixture/run" checksum "$base/router-state/saved")"
}
"$fixture/run" stage o34leg01 wifi-ipv6-uplink 180
review wifi-ipv6-uplink
"$fixture/run" arm o34leg01 wifi-ipv6-uplink 180
grep -qxF "$fixture/etc/rad.conf" "$base/router-state/saved/absent"
strip_rad_manifest
"$fixture/run" activate o34leg01
"$fixture/run" restore
"$fixture/run" stage o34leg02 wifi-ipv6-nat66 180
review wifi-ipv6-nat66
"$fixture/run" arm o34leg02 wifi-ipv6-nat66 180
"$fixture/run" activate o34leg02
strip_rad_manifest
expect_fail restore
grep -q 'incomplete saved file manifest' "$fixture/failure.log"
[[ -f $base/router-state/pending && -e $fixture/etc/rad.conf ]]
print -r -- "$fixture/etc/rad.conf" >> "$base/router-state/saved/absent"
"$fixture/run" atomic "$base/router-state/snapshot-digest" "$("$fixture/run" checksum "$base/router-state/saved")"
"$fixture/run" restore
[[ ! -e $fixture/etc/rad.conf && ! -f $base/router-state/pending ]]
# A persist table survives a reload that does not declare it. Forward
# activation from a confirmed NAT66 stage to one without NAT66 must retire
# <nat66_egress>; otherwise the next snapshot refuses the orphan.
"$fixture/run" stage o34fwd01 wifi-ipv6-nat66 180
review wifi-ipv6-nat66
"$fixture/run" arm o34fwd01 wifi-ipv6-nat66 180
"$fixture/run" activate o34fwd01
confirm_fresh o34fwd01 46021 46022
[[ -f $base/pf-tables/nat66_egress ]]
print '2001:db8:5::7' > "$base/pf-tables/nat66_egress"
"$fixture/run" stage o34fwd02 wifi-ipv6-uplink 180
review wifi-ipv6-uplink
"$fixture/run" arm o34fwd02 wifi-ipv6-uplink 180
grep -qx '2001:db8:5::7' "$base/router-state/saved/tables/nat66_egress"
: > "$base/operations"
"$fixture/run" activate o34fwd02
[[ ! -e $base/pf-tables/nat66_egress ]]
# Restoring the NAT66 baseline brings the table back with its saved address.
"$fixture/run" restore
grep -qx '2001:db8:5::7' "$base/pf-tables/nat66_egress"
"$fixture/run" stage o34fwd03 wifi-ipv6-uplink 180
review wifi-ipv6-uplink
"$fixture/run" arm o34fwd03 wifi-ipv6-uplink 180
"$fixture/run" activate o34fwd03
confirm_fresh o34fwd03 46031 46032
[[ ! -e $base/pf-tables/nat66_egress ]]
# Without retirement the orphan would block the next arm: show the refusal.
print '2001:db8:5::7' > "$base/pf-tables/nat66_egress"
"$fixture/run" stage o34fwd04 wifi-ipv6-uplink 180
review wifi-ipv6-uplink
expect_fail arm o34fwd04 wifi-ipv6-uplink 180
grep -q 'runtime-only PF table' "$fixture/failure.log"
[[ ! -f $base/router-state/pending ]]
rm "$base/pf-tables/nat66_egress"
# The journal lock waits briefly for a live short holder (the egress helper)
# and still refuses a holder that does not let go.
mkdir "$base/router-lock"
sleep 2 &
print -- $! > "$base/router-lock/pid"
"$fixture/run" lock
[[ ! -d $base/router-lock ]]
mkdir "$base/router-lock"
sleep 30 &
holder=$!
print -- "$holder" > "$base/router-lock/pid"
expect_fail lock
grep -q 'journal is busy' "$fixture/failure.log"
kill "$holder"
rm -rf "$base/router-lock"
# The cron tick must restore an expired trial even while another process
# (the per-minute egress helper) briefly holds the lock: it waits, it never
# skips. Only a live "$SELF activate" holder is terminated.
expired_trial() {
 "$fixture/run" stage "$1" wifi-ipv6-uplink 180
 review wifi-ipv6-uplink
 "$fixture/run" arm "$1" wifi-ipv6-uplink 180
 "$fixture/run" activate "$1"
 print 0 > "$base/router-state/deadline"
}
expired_trial o34tick1
mkdir "$base/router-lock"
sleep 2 &
print -- $! > "$base/router-lock/pid"
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored && ! -f $base/router-state/pending && ! -d $base/router-lock ]]
# A pidless lock (a holder between mkdir and its pid write, or a pid file
# gone mid-read) neither aborts tick nor gets deleted; here it is released.
expired_trial o34tick2
mkdir "$base/router-lock"
( sleep 2; rm -rf "$base/router-lock" ) &
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored && ! -f $base/router-state/pending ]]
# A young pidless lock that stays is busy, not stale: tick fails (cron
# retries next minute) and the lock survives untouched.
expired_trial o34tick3
mkdir "$base/router-lock"
expect_fail tick
grep -q 'journal is busy' "$fixture/failure.log"
[[ -d $base/router-lock && -f $base/router-state/pending ]]
# Over a minute old: its owner died before writing the pid; take over.
touch -t 202601010000 "$base/router-lock"
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored && ! -d $base/router-lock ]]
# g24 camera-igc3: the NAT66 profile with 10.40.0.1/24 moved from igc0 to the
# uncabled igc3. Start from a confirmed NAT66 baseline, as on the live router.
"$fixture/run" stage g24nat01 wifi-ipv6-nat66 180
review wifi-ipv6-nat66
"$fixture/run" arm g24nat01 wifi-ipv6-nat66 180
"$fixture/run" activate g24nat01
confirm_fresh g24nat01 47001 47002
grep -q ' inet 10.40.0.1 ' "$base/interfaces/igc0"
if grep -q '10.40.0.1' "$base/interfaces/igc3" "$fixture/etc/hostname.igc3"; then exit 1; fi
cam=$base/router-profiles/camera-igc3
cp -Rp "$cam" "$fixture/camera.saved"
# Every misplaced camera element refuses at stage, before any journal change.
camera_refuses() {
 expect_fail stage g24bad01 "$1" 180
 grep -q "$2" "$fixture/failure.log" || { print -u2 -- "wrong refusal for $1: $2"; cat "$fixture/failure.log" >&2; exit 1; }
 [[ $(cat "$base/router-state/token") == g24nat01 ]]
 rm -rf "$cam" "$nat"
 cp -Rp "$fixture/camera.saved" "$cam"
 cp -Rp "$fixture/nat66.saved" "$nat"
}
# Edit one rendered file in place (sed has no portable -i on OpenBSD).
edit() { sed "$1" "$2" > "$2.next"; mv "$2.next" "$2"; }
edit '/10[.]40[.]0[.]1/d' "$cam/hostname.igc3"
print 'inet alias 10.40.0.1 255.255.255.0' >> "$cam/hostname.igc0"
camera_refuses camera-igc3 'camera zone on the wrong port'
edit 's/^cam_if = "igc3"$/cam_if = "igc0"/' "$cam/pf.conf"
grep -qx 'cam_if = "igc0"' "$cam/pf.conf"
camera_refuses camera-igc3 'camera zone on the wrong port'
# On both ports the connected prefix would be duplicated.
print 'inet alias 10.40.0.1 255.255.255.0' >> "$cam/hostname.igc0"
camera_refuses camera-igc3 'duplicate or missing zone prefix'
# NAT66 cannot move the cameras behind its PF's back either.
edit '/10[.]40[.]0[.]1/d' "$nat/hostname.igc0"
print 'inet alias 10.40.0.1 255.255.255.0' >> "$nat/hostname.igc3"
camera_refuses wifi-ipv6-nat66 'camera zone on the wrong port'
# No permit reaches the zone: recorder, router service, update exception.
# The PF macros are literal rule text here, not shell expansions.
# shellcheck disable=SC2016
print 'pass in on $lab_if inet proto tcp from <recorders> to <outdoor> port 554 tag gonf_transit keep state' >> "$cam/pf.conf"
camera_refuses camera-igc3 'camera zone must stay default-deny'
print 'pass in on igc3 inet proto udp to 10.40.0.1 port 123 keep state' >> "$cam/pf.conf"
camera_refuses camera-igc3 'camera zone must stay default-deny'
print 'pass in on { igc2 igc3 } inet proto udp to port 53 keep state' >> "$cam/pf.conf"
camera_refuses camera-igc3 'camera zone must stay default-deny'
# shellcheck disable=SC2016
print 'pass in on $cam_if inet from <camera_updates> to ! <nonpublic> tag gonf_transit keep state' >> "$cam/pf.conf"
camera_refuses camera-igc3 'camera zone must stay default-deny'
# No translation for the zone, neither as a source nor via the update table.
edit 's@^match out on igc0 inet from { 10.20.0.0/24, 10.30.0.0/24 }@match out on igc0 inet from { 10.20.0.0/24, 10.30.0.0/24, 10.40.0.0/24 }@' "$cam/pf.conf"
grep -q '10.40.0.0/24 } to ! <nonpublic> nat-to' "$cam/pf.conf"
camera_refuses camera-igc3 'no NAT for the camera zone'
print 'match out on igc0 inet from <camera_updates> to ! <nonpublic> nat-to 192.168.1.2' >> "$cam/pf.conf"
camera_refuses camera-igc3 'no NAT for the camera zone'
# The explicit, logged deny is mandatory for camera-igc3.
edit '/^block drop out log quick on [$]cam_if$/d' "$cam/pf.conf"
camera_refuses camera-igc3 'camera zone default deny missing'
# No IPv6 on the camera port: no address line, no router advertisement, no DHCP.
print 'inet6 -autoconf' >> "$cam/hostname.igc3"
camera_refuses camera-igc3 'camera port must carry no IPv6'
print 'interface igc3' >> "$cam/rad.conf"
camera_refuses camera-igc3 'rad must serve only igc2'
edit 's/^dhcpd_interfaces=igc2$/dhcpd_interfaces=igc2 igc3/' "$cam/services.conf"
camera_refuses camera-igc3 'wrong Wi-Fi DHCP binding'
# post-ap also puts the cameras on igc3; it stages without the explicit deny.
"$fixture/run" stage g24pap01 post-ap 180
# Activation moves the prefix: drained from igc0 before it is added to igc3,
# any stray IPv6 on the camera port removed. Nothing else changes.
"$fixture/run" stage g24cam01 camera-igc3 180
review camera-igc3
"$fixture/run" arm g24cam01 camera-igc3 180
print ' inet6 fe80::6662:66ff:fe25:cedc%igc3 prefixlen 64 scopeid 0x4' >> "$base/interfaces/igc3"
: > "$base/operations"
"$fixture/run" activate g24cam01
grep -q ' inet 10.40.0.1 ' "$base/interfaces/igc3"
if grep -q '10.40.0.1' "$base/interfaces/igc0" "$fixture/etc/hostname.igc0"; then exit 1; fi
if grep -q 'inet6 ' "$base/interfaces/igc3"; then exit 1; fi
grep -qx 'ifconfig igc3 -inet6' "$base/operations"
awk '$0=="ifconfig igc0 inet 10.40.0.1 delete" {del=NR}
 $0=="ifconfig igc3 inet alias 10.40.0.1 netmask 255.255.255.0" {add=NR}
 END {exit !(del && add > del)}' "$base/operations"
cmp -s "$cam/hostname.igc3" "$fixture/etc/hostname.igc3"
cmp -s "$cam/pf.conf" "$fixture/etc/pf.conf"
grep -qx 'rcctl set dhcpd flags igc2' "$base/operations"
grep -qx 'sysctl net.inet.ip.forwarding=1 net.inet6.ip6.forwarding=1' "$base/operations"
grep -qx 'rcctl start rad' "$base/operations"
cmp -s "$cam/rad.conf" "$fixture/etc/rad.conf"
# IPv6 appearing on the camera port afterwards refuses acceptance.
cp -p "$base/interfaces/igc3" "$fixture/igc3.camera"
print ' inet6 fe80::6662:66ff:fe25:cedc%igc3 prefixlen 64 scopeid 0x4' >> "$base/interfaces/igc3"
expect_fail check_candidate_ipv6
grep -q 'camera port carries IPv6' "$fixture/failure.log"
cp -p "$fixture/igc3.camera" "$base/interfaces/igc3"
"$fixture/run" check_candidate_ipv6
# Timed restore returns the zone to igc0, files included; NAT66 stays.
print 0 > "$base/router-state/deadline"
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored && ! -f $base/router-state/pending ]]
grep -q ' inet 10.40.0.1 ' "$base/interfaces/igc0"
if grep -q '10.40.0.1' "$base/interfaces/igc3"; then exit 1; fi
for f in hostname.igc0 hostname.igc3 pf.conf rad.conf; do cmp -s "$nat/$f" "$fixture/etc/$f"; done
grep -qx 'net.inet6.ip6.forwarding=1' "$fixture/etc/sysctl.conf"
# Boot restoration of an unconfirmed camera trial restores the NAT66 files.
cp -p "$base/interfaces/igc0" "$fixture/igc0.nat66"
cp -p "$base/interfaces/igc3" "$fixture/igc3.nat66"
"$fixture/run" stage g24cam02 camera-igc3 180
review camera-igc3
"$fixture/run" arm g24cam02 camera-igc3 180
"$fixture/run" activate g24cam02
"$fixture/run" boot_restore
[[ $(cat "$base/router-state/status") == restored-at-boot && ! -f $base/router-state/pending ]]
for f in hostname.igc0 hostname.igc3 pf.conf rad.conf; do cmp -s "$nat/$f" "$fixture/etc/$f"; done
# A real reboot drops the runtime state; the mocks keep it, so reset it.
cp -p "$fixture/igc0.nat66" "$base/interfaces/igc0"
cp -p "$fixture/igc3.nat66" "$base/interfaces/igc3"
# Confirmed camera profile, then its reboot canary: early boot restores the
# confirmed camera files, so the zone stays on igc3 across the reboot.
"$fixture/run" stage g24cam03 camera-igc3 180
review camera-igc3
"$fixture/run" arm g24cam03 camera-igc3 180
"$fixture/run" activate g24cam03
confirm_fresh g24cam03 47011 47012
cp -p "$base/interfaces/igc0" "$fixture/igc0.camera"
print 'inet alias 10.20.0.254 255.255.255.0' >> "$cam/hostname.igc0"
print '# gonf boot canary: 10.20.0.254' >> "$cam/ntpd.conf"
"$fixture/run" stage g24boot1 camera-igc3 1800
review camera-igc3
"$fixture/run" arm g24boot1 camera-igc3 1800
"$fixture/run" activate g24boot1
print '' > "$base/router-state/preexisting-connections"
cat > "$base/sockets" <<'SOCKETS'
tcp 0 0 192.168.1.2.22 192.168.1.236.47021 ESTABLISHED
tcp 0 0 10.20.0.1.22 10.99.0.2.47022 ESTABLISHED
tcp 0 0 10.99.0.1.22 10.99.0.2.47023 ESTABLISHED
SOCKETS
"$fixture/run" proof old g24boot1 '192.168.1.236 47021 192.168.1.2 22'
"$fixture/run" proof new g24boot1 '10.99.0.2 47022 10.20.0.1 22'
"$fixture/run" proof vpn g24boot1 '10.99.0.2 47023 10.99.0.1 22'
"$fixture/run" reboot_canary g24boot1
[[ -f $base/reboot-requested && -f $base/router-state/pending ]]
"$fixture/run" boot_restore
if grep -q '10.20.0.254' "$fixture/etc/hostname.igc0" "$fixture/etc/ntpd.conf"; then exit 1; fi
for f in hostname.igc0 hostname.igc3 pf.conf rad.conf ntpd.conf; do cmp -s "$fixture/camera.saved/$f" "$fixture/etc/$f"; done
rm -rf "$cam" && cp -Rp "$fixture/camera.saved" "$cam"
rm "$base/reboot-requested"
cp -p "$fixture/igc0.camera" "$base/interfaces/igc0"
cp -p "$fixture/igc3.camera" "$base/interfaces/igc3"
# A later NAT66 restage moves the zone back to igc0 and leaves igc0's IPv6;
# restoring it returns the confirmed camera placement.
"$fixture/run" stage g24fwd01 wifi-ipv6-nat66 180
review wifi-ipv6-nat66
"$fixture/run" arm g24fwd01 wifi-ipv6-nat66 180
: > "$base/operations"
"$fixture/run" activate g24fwd01
grep -q ' inet 10.40.0.1 ' "$base/interfaces/igc0"
if grep -q '10.40.0.1' "$base/interfaces/igc3" "$fixture/etc/hostname.igc3"; then exit 1; fi
if grep -q 'ifconfig igc0 -inet6' "$base/operations"; then exit 1; fi
grep -q 'inet6 fd02:ca0b:f1cb:20::1 ' "$base/interfaces/igc0"
"$fixture/run" restore
grep -q ' inet 10.40.0.1 ' "$base/interfaces/igc3"
if grep -q '10.40.0.1' "$base/interfaces/igc0"; then exit 1; fi
for f in hostname.igc0 hostname.igc3 pf.conf; do cmp -s "$cam/$f" "$fixture/etc/$f"; done
# i44: port wiring is a closed table. Every stageable physical profile has
# one; any other name has none, so no port is ever guessed (g24 N6). c24:
# loop over every rendered profile directory, not a hand-written list, so a
# newly rendered profile cannot miss profile_ok or the wiring table. final is
# rendered for review only: it has no wiring and needs the retirement gate.
rendered=0
for d in "$base"/router-profiles/*/; do
 p=${d%/}
 p=${p##*/}
 [[ $p != final ]] || continue
 "$fixture/run" profile_ok "$p"
 "$fixture/run" wiring "$p" > /dev/null
 (( rendered += 1 ))
done
# The nine stageable physical profiles of the c24 render (lab-igc1 included).
(( rendered >= 9 ))
[[ -d $base/router-profiles/lab-igc1 ]]
[[ $("$fixture/run" wiring lab-igc1) == 'igc1 igc2 igc3' && $("$fixture/run" camera_port lab-igc1) == igc3 ]]
[[ $("$fixture/run" camera_port camera-igc3) == igc3 && $("$fixture/run" camera_port post-ap) == igc3 ]]
[[ $("$fixture/run" camera_port wifi-first) == igc0 && $("$fixture/run" camera_port post-rack) == igc0 ]]
for p in final vpn-bootstrap camera-igc4 ''; do
 expect_fail camera_port "$p"
 [[ ! -s $fixture/failure.log ]]
done
# A non-NAT66 bundle under an unlisted name reaches the wiring check.
expect_fail validate "$base/router-profiles/post-ap" post-ap2
grep -q 'profile has no reviewed port wiring' "$fixture/failure.log"
# Every rendered stageable profile passes the tightened validator unchanged.
for d in "$base"/router-profiles/*/; do
 p=${d%/}
 p=${p##*/}
 [[ $p != final ]] || continue
 "$fixture/run" validate "$base/router-profiles/$p" "$p"
done
[[ ! -e $base/router-pf-expanded ]]
# Each refusal leaves the journal (restored g24fwd01) and all profiles as they were.
cp -Rp "$base/router-profiles" "$fixture/profiles.saved"
i44_refuses() {
 expect_fail stage i44bad01 "$1" 180
 grep -qF -- "$2" "$fixture/failure.log" || { print -u2 -- "wrong refusal for $1: $2"; cat "$fixture/failure.log" >&2; exit 1; }
 [[ $(cat "$base/router-state/token") == g24fwd01 ]]
 rm -rf "$base/router-profiles"
 cp -Rp "$fixture/profiles.saved" "$base/router-profiles"
}
wf=$base/router-profiles/wifi-first
wp=$base/router-profiles/wifi-prepare
# 10.30.0.1 is bound to igc2 by address and by the PF macro, not merely once.
edit '/^inet alias 10[.]30[.]0[.]1 /d' "$wf/hostname.igc2"
print 'inet alias 10.30.0.1 255.255.255.0' >> "$wf/hostname.igc0"
i44_refuses wifi-first 'Wi-Fi zone on the wrong port'
edit 's/^wifi_if = "igc2"$/wifi_if = "igc0"/' "$wf/pf.conf"
i44_refuses wifi-first 'Wi-Fi zone on the wrong port'
edit '/^inet alias 10[.]30[.]0[.]1 /d' "$wp/hostname.igc2"
print 'inet alias 10.30.0.1 255.255.255.0' >> "$wp/hostname.igc3"
i44_refuses wifi-prepare 'Wi-Fi zone on the wrong port'
edit '/^inet alias 10[.]30[.]0[.]1 /d' "$cam/hostname.igc2"
print 'inet alias 10.30.0.1 255.255.255.0' >> "$cam/hostname.igc0"
i44_refuses camera-igc3 'Wi-Fi zone on the wrong port'
edit '/^inet alias 10[.]20[.]0[.]1 /d' "$wf/hostname.igc0"
print 'inet alias 10.20.0.1 255.255.255.0' >> "$wf/hostname.igc2"
i44_refuses wifi-first 'lab zone on the wrong port'
edit '/^inet6 /d' "$wf/hostname.igc0"
print 'inet6 alias fd02:ca0b:f1cb:20::1 64\ninet6 -autoconf\ninet6 -temporary' >> "$wf/hostname.igc2"
i44_refuses wifi-first 'lab ULA on the wrong port'
# DHCP follows the stage: none in wifi-prepare, igc2 only in wifi-first.
edit 's/^dhcpd_interfaces=disabled$/dhcpd_interfaces=igc2/' "$wp/services.conf"
i44_refuses wifi-prepare 'DHCP forbidden on shared wire'
edit 's/^dhcpd_interfaces=igc2$/dhcpd_interfaces=disabled/' "$wf/services.conf"
i44_refuses wifi-first 'wrong Wi-Fi DHCP binding'
# netstart turns a third word into a prefix length: the joined policy line refuses.
edit '/^inet6 -temporary$/d' "$wf/hostname.igc0"
edit 's/^inet6 -autoconf$/inet6 -autoconf -temporary/' "$wf/hostname.igc0"
i44_refuses wifi-first 'netstart misparses'
edit '/^inet6 -temporary$/d' "$wf/hostname.igc0"
i44_refuses wifi-first 'persistent IPv6 policy missing or duplicated'
print 'inet6 -autoconf' >> "$wf/hostname.igc2"
i44_refuses wifi-first 'persistent IPv6 policy missing or duplicated'
edit '/^inet6 -/d' "$wf/hostname.igc0"
print 'inet6 -autoconf\ninet6 -temporary' >> "$wf/hostname.igc2"
i44_refuses wifi-first 'IPv6 policy must accompany the lab address'
print 'inet6 -temporary' >> "$up/hostname.igc0"
i44_refuses wifi-ipv6-uplink 'persistent IPv6 policy missing or duplicated'
# g24 N1: the camera checks read pfctl's expansion, so every spelling of the
# port or zone refuses, and the deny must come before any permit that can
# match the port. Literal PF text below, not shell expansions.
cam_before_deny() { edit "/^# g24 camera zone on/i\\
$1
" "$cam/pf.conf"; }
print 'pass in quick on igc3' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone must stay default-deny'
print 'pass in on {igc2,igc3} inet proto udp to port 53' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone must stay default-deny'
print 'pass in on igc2 inet from (igc3:network) to any' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone must stay default-deny'
print 'pass out on igc0 inet from (igc3) to any' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone must stay default-deny'
# shellcheck disable=SC2016
print 'cams = "igc3"\npass in on $cams inet proto tcp to port 22' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone must stay default-deny'
print 'match out on igc0 inet from (igc3:network) to ! <nonpublic> nat-to 192.168.1.2' >> "$cam/pf.conf"
i44_refuses camera-igc3 'no NAT for the camera zone'
cam_before_deny 'pass in quick inet proto tcp to port 8080'
i44_refuses camera-igc3 'camera zone deny must precede every permit'
cam_before_deny 'pass in quick on ! igc0 inet proto tcp to port 8080'
i44_refuses camera-igc3 'camera zone deny must precede every permit'
cam_before_deny 'pass in quick on igc inet proto tcp to port 8080'
i44_refuses camera-igc3 'camera zone deny must precede every permit'
cam_before_deny 'pass in quick on egress inet proto tcp to port 8080'
i44_refuses camera-igc3 'camera zone deny must precede every permit'
# A deny after the interface-less transit permit no longer shields the port.
edit '/^block drop [a-z]* log quick on [$]cam_if$/d' "$cam/pf.conf"
# shellcheck disable=SC2016
print 'block drop in log quick on $cam_if\nblock drop out log quick on $cam_if' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone deny must precede every permit'
# Controls: a concrete other port before the deny, and a public 110.40.0.0/16
# address after it, are not camera rules.
cam_before_deny 'pass in quick on igc2 inet proto tcp from 10.30.0.0/24 to 10.30.0.1 port 8080'
print 'pass in on wg2 inet proto tcp from 10.99.0.2 to 110.40.0.1 port 22' >> "$cam/pf.conf"
"$fixture/run" validate "$cam" camera-igc3
rm -rf "$base/router-profiles"
cp -Rp "$fixture/profiles.saved" "$base/router-profiles"
# i44: wifi-prepare and wifi-first through the journal, from the confirmed
# camera-igc3 placement the g24 section left behind.
"$fixture/run" stage i44wfp01 wifi-prepare 180
review wifi-prepare
"$fixture/run" arm i44wfp01 wifi-prepare 180
: > "$base/operations"
"$fixture/run" activate i44wfp01
# DHCP stays off, the Wi-Fi gateway moves to igc2, cameras return to igc0.
grep -qx 'rcctl disable dhcpd' "$base/operations"
if grep -q 'rcctl start dhcpd' "$base/operations"; then exit 1; fi
grep -qx 'rcctl disable rad' "$base/operations"
grep -qx 'sysctl net.inet.ip.forwarding=1 net.inet6.ip6.forwarding=0' "$base/operations"
grep -q ' inet 10.30.0.1 ' "$base/interfaces/igc2"
grep -q ' inet 10.40.0.1 ' "$base/interfaces/igc0"
if grep -q '10.40.0.1' "$base/interfaces/igc3"; then exit 1; fi
# The split policy reaches ifconfig as netstart would issue it at boot.
grep -qx 'ifconfig igc0 inet6 -autoconf' "$base/operations"
grep -qx 'ifconfig igc0 inet6 -temporary' "$base/operations"
if grep -q prefixlen.-temporary "$base/operations"; then exit 1; fi
for f in hostname.igc0 hostname.igc2 hostname.igc3 pf.conf dhcpd.conf; do cmp -s "$wp/$f" "$fixture/etc/$f"; done
[[ ! -e $fixture/etc/rad.conf ]]
"$fixture/run" check_candidate_ipv6
# An unlisted profile name in the journal fails closed instead of using igc0.
print camera-igc4 > "$base/router-state/profile"
expect_fail check_candidate_ipv6
grep -q 'profile has no reviewed camera port' "$fixture/failure.log"
print wifi-prepare > "$base/router-state/profile"
# The lab stays on the shared igc0: old/new receipts confirm, no igc1 receipt.
confirm_fresh i44wfp01 47101 47102
[[ $(cat "$base/router-state/status") == confirmed && ! -f $base/router-state/pending ]]
cp -p "$base/interfaces/igc0" "$fixture/igc0.prepare"
cp -p "$base/interfaces/igc2" "$fixture/igc2.prepare"
# wifi-first starts dhcpd on igc2 only; timed expiry returns wifi-prepare.
"$fixture/run" stage i44wfa01 wifi-first 180
review wifi-first
"$fixture/run" arm i44wfa01 wifi-first 180
: > "$base/operations"
"$fixture/run" activate i44wfa01
grep -qx 'rcctl enable dhcpd' "$base/operations"
grep -qx 'rcctl set dhcpd flags igc2' "$base/operations"
grep -qx 'rcctl start dhcpd' "$base/operations"
for f in hostname.igc0 hostname.igc2 pf.conf dhcpd.conf; do cmp -s "$wf/$f" "$fixture/etc/$f"; done
print 0 > "$base/router-state/deadline"
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored && ! -f $base/router-state/pending ]]
for f in hostname.igc0 hostname.igc2 pf.conf dhcpd.conf; do cmp -s "$wp/$f" "$fixture/etc/$f"; done
grep -q ' inet 10.30.0.1 ' "$base/interfaces/igc2"
# Boot restoration of an unconfirmed wifi-first trial restores wifi-prepare.
"$fixture/run" stage i44wfa02 wifi-first 180
review wifi-first
"$fixture/run" arm i44wfa02 wifi-first 180
"$fixture/run" activate i44wfa02
"$fixture/run" boot_restore
[[ $(cat "$base/router-state/status") == restored-at-boot && ! -f $base/router-state/pending ]]
for f in hostname.igc0 hostname.igc2 pf.conf dhcpd.conf; do cmp -s "$wp/$f" "$fixture/etc/$f"; done
cp -p "$fixture/igc0.prepare" "$base/interfaces/igc0"
cp -p "$fixture/igc2.prepare" "$base/interfaces/igc2"
# wifi-first confirms on old/new receipts as well.
"$fixture/run" stage i44wfa03 wifi-first 180
review wifi-first
"$fixture/run" arm i44wfa03 wifi-first 180
"$fixture/run" activate i44wfa03
confirm_fresh i44wfa03 47111 47112
[[ $(cat "$base/router-state/status") == confirmed ]]
# Once the lab moves to igc1 (post-rack), confirmation needs the fresh igc1
# recovery receipt; a recovery tuple for the wrong address is refused.
"$fixture/run" stage i44rack1 post-rack 180
review post-rack
"$fixture/run" arm i44rack1 post-rack 180
"$fixture/run" activate i44rack1
print '' > "$base/router-state/preexisting-connections"
print 'tcp 0 0 192.168.1.2.22 192.168.1.236.47121 ESTABLISHED' > "$base/sockets"
"$fixture/run" proof old i44rack1 '192.168.1.236 47121 192.168.1.2 22'
print 'tcp 0 0 10.20.0.1.22 10.99.0.2.47122 ESTABLISHED' > "$base/sockets"
"$fixture/run" proof new i44rack1 '10.99.0.2 47122 10.20.0.1 22'
expect_fail confirm i44rack1 services-checked
grep -q 'fresh igc1 recovery receipt missing' "$fixture/failure.log"
print 'tcp 0 0 10.20.0.1.22 10.98.0.250.47123 ESTABLISHED' > "$base/sockets"
expect_fail proof recovery i44rack1 '10.98.0.250 47123 10.20.0.1 22'
grep -q 'wrong management address' "$fixture/failure.log"
print 'tcp 0 0 10.98.0.1.22 10.98.0.250.47124 ESTABLISHED' > "$base/sockets"
"$fixture/run" proof recovery i44rack1 '10.98.0.250 47124 10.98.0.1 22'
"$fixture/run" confirm i44rack1 services-checked
[[ $(cat "$base/router-state/status") == confirmed && ! -f $base/router-state/pending ]]
# c24 lab-igc1: camera-igc3 with 10.20.0.1/24, the lab ULA and their routes
# moved from igc0 to igc1 after the b24 cable move. Start from a confirmed
# camera-igc3 baseline, as on the live router.
"$fixture/run" stage c24cam01 camera-igc3 180
review camera-igc3
"$fixture/run" arm c24cam01 camera-igc3 180
"$fixture/run" activate c24cam01
confirm_fresh c24cam01 47201 47202
grep -q ' inet 10.20.0.1 ' "$base/interfaces/igc0"
grep -q 'inet6 fd02:ca0b:f1cb:20::1 ' "$base/interfaces/igc0"
lab=$base/router-profiles/lab-igc1
"$fixture/run" validate "$lab" lab-igc1
grep -qx 'pass in on igc1 inet proto tcp from 10.98.0.0/24 to 10.98.0.1 port 22 keep state' "$lab/pf.conf"
if grep -q 'from 10.98.0.0/24 to 10.98.0.1' "$cam/pf.conf"; then exit 1; fi
cp -Rp "$lab" "$fixture/lab.saved"
cp -Rp "$cam" "$fixture/camera.c24"
# Every misplaced lab element refuses at stage; the confirmed journal stays.
c24_refuses() {
 expect_fail stage c24bad01 lab-igc1 180
 grep -qF -- "$1" "$fixture/failure.log" || { print -u2 -- "wrong refusal: $1"; cat "$fixture/failure.log" >&2; exit 1; }
 [[ $(cat "$base/router-state/token") == c24cam01 && $(cat "$base/router-state/status") == confirmed ]]
 rm -rf "$lab"
 cp -Rp "$fixture/lab.saved" "$lab"
}
# The lab gateway on both ports duplicates the connected prefix.
print 'inet alias 10.20.0.1 255.255.255.0' >> "$lab/hostname.igc0"
c24_refuses 'duplicate or missing zone prefix'
# A lab, Wi-Fi or camera address beside another zone's port does as well.
print 'inet alias 10.20.0.99 255.255.255.0' >> "$lab/hostname.igc2"
c24_refuses 'lab prefix on a second port'
# Address and PF macro must agree on igc1.
edit 's/^lab_if = "igc1"$/lab_if = "igc0"/' "$lab/pf.conf"
c24_refuses 'lab zone on the wrong port'
edit '/^inet alias 10[.]20[.]0[.]1 /d' "$lab/hostname.igc1"
print 'inet alias 10.20.0.1 255.255.255.0' >> "$lab/hostname.igc0"
c24_refuses 'lab zone on the wrong port'
# The ULA follows the lab; on the uplink it would sit beside SLAAC.
edit '/^inet6 alias fd02:ca0b:f1cb:20::1 64$/d' "$lab/hostname.igc1"
print 'inet6 alias fd02:ca0b:f1cb:20::1 64' >> "$lab/hostname.igc0"
c24_refuses 'lab ULA on the wrong port'
# The igc0 boot canary would put the lab prefix on a second port.
print 'inet alias 10.20.0.254 255.255.255.0' >> "$lab/hostname.igc0"
c24_refuses 'lab prefix on a second port'
# So would any Wi-Fi or camera address away from its own port.
print 'inet alias 10.30.0.99 255.255.255.0' >> "$lab/hostname.igc1"
c24_refuses 'Wi-Fi prefix on a second port'
print 'inet alias 10.40.0.99 255.255.255.0' >> "$lab/hostname.igc0"
c24_refuses 'camera prefix on a second port'
# The lab port keeps both local-only policy lines; SLAAC stays on igc0.
edit '/^inet6 -/d' "$lab/hostname.igc1"
c24_refuses 'persistent IPv6 policy missing or duplicated'
edit '/^inet6 autoconf -temporary$/d' "$lab/hostname.igc0"
print 'inet6 autoconf -temporary' >> "$lab/hostname.igc1"
c24_refuses 'IPv6 autoconfiguration only on the igc0 uplink'
# Recovery stays on igc1; DHCP serves the lab and Wi-Fi ports only.
edit '/^inet alias 10[.]98[.]0[.]1 /d' "$lab/hostname.igc1"
c24_refuses 'missing igc1 recovery address'
edit 's/^dhcpd_interfaces=igc1 igc2$/dhcpd_interfaces=igc2/' "$lab/services.conf"
c24_refuses 'wrong DHCP binding'
# c24 review B1: dhcpd binds igc1 by whichever address is first, so both igc1
# prefixes need a subnet in one shared-network; a lab-only subnet (the live
# defect) or two top-level subnets refuse.
grep -q '^shared-network igc1-lab {$' "$lab/dhcpd.conf"
edit '/^shared-network igc1-lab {$/,/^}$/c\
subnet 10.20.0.0 netmask 255.255.255.0 {\
 option routers 10.20.0.1;\
 deny unknown-clients;\
}' "$lab/dhcpd.conf"
c24_refuses 'dhcpd.conf must declare every IPv4 prefix of igc1, several in one shared-network'
edit '/^shared-network igc1-lab {$/,/^}$/c\
subnet 10.98.0.0 netmask 255.255.255.0 {\
 deny unknown-clients;\
}\
subnet 10.20.0.0 netmask 255.255.255.0 {\
 option routers 10.20.0.1;\
 deny unknown-clients;\
}' "$lab/dhcpd.conf"
c24_refuses 'dhcpd.conf must declare every IPv4 prefix of igc1, several in one shared-network'
# The camera zone keeps its explicit deny and no permit, as in camera-igc3.
edit '/^block drop in log quick on [$]cam_if$/d' "$lab/pf.conf"
c24_refuses 'camera zone default deny missing'
print 'pass in on igc1 inet proto tcp from 10.20.0.0/24 to 10.40.0.11 port 554' >> "$lab/pf.conf"
c24_refuses 'camera zone must stay default-deny'
print 'interface igc1' >> "$lab/rad.conf"
c24_refuses 'rad must serve only igc2'
# Readiness records the post-b24 old-management state; the pre-b24 value
# of every other profile refuses before anything is armed.
"$fixture/run" stage c24lab01 lab-igc1 180
review lab-igc1
expect_fail arm c24lab01 lab-igc1 180
grep -q 'missing exact candidate-bound readiness receipt' "$fixture/failure.log"
[[ ! -f $base/router-state/pending ]]
review lab-igc1 isp-uplink-only
"$fixture/run" arm c24lab01 lab-igc1 180
: > "$base/operations"
"$fixture/run" activate c24lab01
# The lab gateway and ULA leave igc0 before they reach igc1; recovery stays.
grep -q ' inet 10.20.0.1 ' "$base/interfaces/igc1"
grep -q 'inet6 fd02:ca0b:f1cb:20::1 ' "$base/interfaces/igc1"
if grep -q -e '10.20.0.1 ' -e 'fd02:ca0b:f1cb:20::1 ' "$base/interfaces/igc0"; then exit 1; fi
grep -q ' inet 10.98.0.1 ' "$base/interfaces/igc1"
grep -q ' inet 192.168.1.2 ' "$base/interfaces/igc0"
if grep -q 'ifconfig igc1 inet 10.98.0.1 delete' "$base/operations"; then exit 1; fi
awk '$0=="ifconfig igc0 inet 10.20.0.1 delete" {del=NR}
 $0=="ifconfig igc1 inet alias 10.20.0.1 netmask 255.255.255.0" {add=NR}
 $0=="ifconfig igc0 inet6 fd02:ca0b:f1cb:20::1 delete" {del6=NR}
 $0=="ifconfig igc1 inet6 alias fd02:ca0b:f1cb:20::1 prefixlen 64" {add6=NR}
 END {exit !(del && add > del && del6 && add6 > del6)}' "$base/operations"
# Local-only policy on the lab port, SLAAC on the uplink, the camera port bare.
grep -qx 'ifconfig igc1 inet6 -autoconf' "$base/operations"
grep -qx 'ifconfig igc1 inet6 -temporary' "$base/operations"
grep -qx 'ifconfig igc0 inet6 autoconf -temporary' "$base/operations"
grep -qx 'ifconfig igc3 -inet6' "$base/operations"
grep -qx 'slaacctl send solicitation igc0' "$base/operations"
if grep -q 'slaacctl send solicitation igc[123]' "$base/operations"; then exit 1; fi
grep -q 'AUTOCONF6,' "$base/interfaces/igc0"
if grep -q AUTOCONF6 "$base/interfaces/igc1"; then exit 1; fi
# dhcpd on the lab and Wi-Fi ports, NAT66 and rad as in camera-igc3.
grep -qx 'rcctl set dhcpd flags igc1 igc2' "$base/operations"
grep -qx 'rcctl start dhcpd' "$base/operations"
grep -qx 'rcctl start rad' "$base/operations"
grep -qx 'sysctl net.inet.ip.forwarding=1 net.inet6.ip6.forwarding=1' "$base/operations"
[[ -f $base/pf-tables/nat66_egress ]]
for f in hostname.igc0 hostname.igc1 hostname.igc2 hostname.igc3 pf.conf dhcpd.conf rad.conf; do cmp -s "$lab/$f" "$fixture/etc/$f"; done
"$fixture/run" check_candidate_ipv6
# SLAAC on the lab port refuses acceptance.
cp -p "$base/interfaces/igc1" "$fixture/igc1.lab"
sed 's/flags=</flags=<AUTOCONF6,/' "$fixture/igc1.lab" > "$base/interfaces/igc1"
expect_fail check_candidate_ipv6
grep -q 'physical IPv6 autoconfiguration remains enabled' "$fixture/failure.log"
cp -p "$fixture/igc1.lab" "$base/interfaces/igc1"
# lab-igc1 is no boot canary profile.
expect_fail reboot_canary c24lab01
grep -q 'no active boot canary' "$fixture/failure.log"
# The new and recovery receipts must come from lab hosts: the laptop's wg2
# sessions to 10.20.0.1 and 10.98.0.1 never cross igc1. old is not required
# after b24; recovery and VPN are.
print '' > "$base/router-state/preexisting-connections"
print 'tcp 0 0 10.20.0.1.22 10.99.0.2.47211 ESTABLISHED' > "$base/sockets"
expect_fail proof new c24lab01 '10.99.0.2 47211 10.20.0.1 22'
grep -q 'lab-igc1 new and recovery receipts must come from a lab host' "$fixture/failure.log"
[[ ! -e $base/router-state/proof.new ]]
print 'tcp 0 0 10.20.0.1.22 10.20.0.125.47212 ESTABLISHED' > "$base/sockets"
"$fixture/run" proof new c24lab01 '10.20.0.125 47212 10.20.0.1 22'
expect_fail confirm c24lab01 services-checked
grep -q 'fresh igc1 recovery receipt missing' "$fixture/failure.log"
print 'tcp 0 0 10.98.0.1.22 10.99.0.2.47213 ESTABLISHED' > "$base/sockets"
expect_fail proof recovery c24lab01 '10.99.0.2 47213 10.98.0.1 22'
grep -q 'lab-igc1 new and recovery receipts must come from a lab host' "$fixture/failure.log"
[[ ! -e $base/router-state/proof.recovery ]]
print 'tcp 0 0 10.98.0.1.22 10.98.0.125.47214 ESTABLISHED' > "$base/sockets"
"$fixture/run" proof recovery c24lab01 '10.98.0.125 47214 10.98.0.1 22'
expect_fail confirm c24lab01 services-checked
grep -q 'fresh VPN receipt missing' "$fixture/failure.log"
[[ $(cat "$base/router-state/status") == active-unconfirmed && -f $base/router-state/pending ]]
# Timed restore after the cable move returns camera-igc3: the lab gateway
# goes back to igc0, recovery never leaves igc1 (current-cabling recovery).
print 0 > "$base/router-state/deadline"
: > "$base/operations"
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored && ! -f $base/router-state/pending ]]
grep -q ' inet 10.20.0.1 ' "$base/interfaces/igc0"
grep -q 'inet6 fd02:ca0b:f1cb:20::1 ' "$base/interfaces/igc0"
if grep -q -e '10.20.0.1 ' -e 'fd02:ca0b:f1cb:20::1 ' "$base/interfaces/igc1"; then exit 1; fi
grep -q ' inet 10.98.0.1 ' "$base/interfaces/igc1"
if grep -E 'ifconfig igc1 inet 10[.]98[.]0[.]1 delete|ifconfig igc[023].*10[.]98[.]0[.]1' "$base/operations"; then exit 1; fi
for f in hostname.igc0 hostname.igc1 hostname.igc2 hostname.igc3 pf.conf dhcpd.conf rad.conf; do cmp -s "$fixture/camera.c24/$f" "$fixture/etc/$f"; done
cp -p "$base/interfaces/igc0" "$fixture/igc0.c24"
cp -p "$base/interfaces/igc1" "$fixture/igc1.c24"
# Boot restoration of an unconfirmed lab-igc1 trial restores camera-igc3.
"$fixture/run" stage c24lab02 lab-igc1 180
review lab-igc1 isp-uplink-only
"$fixture/run" arm c24lab02 lab-igc1 180
"$fixture/run" activate c24lab02
"$fixture/run" boot_restore
[[ $(cat "$base/router-state/status") == restored-at-boot && ! -f $base/router-state/pending ]]
for f in hostname.igc0 hostname.igc1 hostname.igc2 hostname.igc3 pf.conf dhcpd.conf rad.conf; do cmp -s "$fixture/camera.c24/$f" "$fixture/etc/$f"; done
cp -p "$fixture/igc0.c24" "$base/interfaces/igc0"
cp -p "$fixture/igc1.c24" "$base/interfaces/igc1"
# Confirmation with new (lab host), recovery and VPN receipts; no old one.
"$fixture/run" stage c24lab03 lab-igc1 180
review lab-igc1 isp-uplink-only
"$fixture/run" arm c24lab03 lab-igc1 180
"$fixture/run" activate c24lab03
print '' > "$base/router-state/preexisting-connections"
cat > "$base/sockets" <<'SOCKETS'
tcp 0 0 10.20.0.1.22 10.20.0.125.47221 ESTABLISHED
tcp 0 0 10.98.0.1.22 10.98.0.125.47222 ESTABLISHED
tcp 0 0 10.99.0.1.22 10.99.0.2.47223 ESTABLISHED
SOCKETS
"$fixture/run" proof new c24lab03 '10.20.0.125 47221 10.20.0.1 22'
"$fixture/run" proof recovery c24lab03 '10.98.0.125 47222 10.98.0.1 22'
"$fixture/run" proof vpn c24lab03 '10.99.0.2 47223 10.99.0.1 22'
[[ ! -e $base/router-state/proof.old ]]
# Flags alone never confirm: dhcpd must hold a bpf device for igc1 and igc2.
touch "$base/dhcpd-dropped-port"
expect_fail confirm c24lab03 services-checked
grep -q 'dhcpd does not listen on every DHCP port of the candidate' "$fixture/failure.log"
rm "$base/dhcpd-dropped-port"
touch "$base/dhcpd-down"
expect_fail confirm c24lab03 services-checked
rm "$base/dhcpd-down"
[[ $(cat "$base/router-state/status") == active-unconfirmed && -f $base/router-state/pending ]]
"$fixture/run" confirm c24lab03 services-checked
[[ $(cat "$base/router-state/status") == confirmed && ! -f $base/router-state/pending ]]
expect_fail restore
rm -rf "$fixture/lab.saved" "$fixture/camera.c24"
# Bootstrap must not alter physical interfaces, PF, routes or service state.
rm "$fixture/etc/hostname.wg2" "$base/interfaces/wg2"
mkdir "$base/router-profiles/vpn-bootstrap"
{
 print 'inet 10.99.0.1 255.255.255.0'
 print 'wgport 51822'
 printf 'wgkey %043d=\n' 1
 printf 'wgpeer %043d= wgaip 10.99.0.2/32\n' 2
 printf 'wgpeer %043d= wgaip 10.99.0.3/32\n' 3
 print up
} > "$base/router-profiles/vpn-bootstrap/hostname.wg2"
chmod 600 "$base/router-profiles/vpn-bootstrap/hostname.wg2"
"$fixture/run" stage vpntrial1 vpn-bootstrap 180
touch "$base/router-profiles/vpn-bootstrap/unknown-input"
expect_fail stage vpntrial1 vpn-bootstrap 180
rm "$base/router-profiles/vpn-bootstrap/unknown-input"
touch "$fixture/etc/hostname.wg2"
expect_fail arm vpntrial1 vpn-bootstrap 180
rm "$fixture/etc/hostname.wg2"
print 'wg2: flags=<UP>' > "$base/interfaces/wg2"
expect_fail arm vpntrial1 vpn-bootstrap 180
rm "$base/interfaces/wg2"
"$fixture/run" arm vpntrial1 vpn-bootstrap 180
: > "$base/operations"
"$fixture/run" activate vpntrial1
print 0 > "$base/router-state/deadline"
"$fixture/run" tick
[[ ! -e $fixture/etc/hostname.wg2 && ! -e $base/interfaces/wg2 ]]
if grep -Ev '^(ifconfig wg2 (create description gonf-vpn-vpntrial1|destroy)|rcctl check cron)$' "$base/operations"; then exit 1; fi
"$fixture/run" restore
"$fixture/run" stage vpntrial2 vpn-bootstrap 180
"$fixture/run" arm vpntrial2 vpn-bootstrap 180
"$fixture/run" activate vpntrial2
expect_fail confirm vpntrial2 services-checked
print '' > "$base/router-state/preexisting-connections"
print 'tcp 0 0 192.168.1.2.22 192.168.1.236.45671 ESTABLISHED' > "$base/sockets"
"$fixture/run" proof old vpntrial2 '192.168.1.236 45671 192.168.1.2 22'
print 'tcp 0 0 10.99.0.1.22 10.99.0.2.45672 ESTABLISHED' > "$base/sockets"
expect_fail proof vpn vpntrial2 '192.168.1.236 45672 10.99.0.1 22'
"$fixture/run" proof vpn vpntrial2 '10.99.0.2 45672 10.99.0.1 22'
"$fixture/run" confirm vpntrial2 services-checked
expect_fail restore
# Boot file restoration is exercised without booting or touching native state.
rm "$fixture/etc/hostname.wg2" "$base/interfaces/wg2"
"$fixture/run" stage vpntrial3 vpn-bootstrap 180
"$fixture/run" arm vpntrial3 vpn-bootstrap 180
"$fixture/run" activate vpntrial3
"$fixture/run" boot_restore
[[ ! -e $fixture/etc/hostname.wg2 && ! -e $base/router-state/pending ]]
# An externally replaced interface must never be destroyed by this journal.
rm "$base/interfaces/wg2"
"$fixture/run" stage vpntrial4 vpn-bootstrap 180
"$fixture/run" arm vpntrial4 vpn-bootstrap 180
"$fixture/run" activate vpntrial4
print 'wg2: flags=<UP>\n description: external-owner' > "$base/interfaces/wg2"
expect_fail restore
[[ -f $base/interfaces/wg2 && -f $base/router-state/pending ]]
# NAT66 egress helper: exactly one address, only while the loaded rules use
# the table, only under the journal lock, failures logged. Mocked commands
# record into files; the helper's own dispatch runs unchanged.
egress=$fixture/egress
mkdir -p "$egress"
sed "s@/etc/@$fixture/etc/@g" "$3" > "$egress/helper"
cat > "$egress/mocks" <<EOF2
ifconfig() { [[ \$1 == igc0 ]] && cat "$egress/igc0"; }
logger() { print -r -- "logger \$*" >> "$egress/log"; }
pfctl() {
 case \$* in
 -sr) cat "$egress/rules";;
 '-t nat66_egress -T show') [[ -f $egress/table ]] && cat "$egress/table";;
 '-t nat66_egress -T flush') : > "$egress/table";;
 '-t nat66_egress -T replace -f '*) [[ ! -f $egress/replace-fails ]] && cp "\$6" "$egress/table";;
 *) return 1;;
 esac
}
EOF2
cat > "$egress/run" <<EOF2
#!/bin/ksh
. "$egress/mocks"
. "$egress/helper"
EOF2
chmod 700 "$egress/run"
nat_rule='match out on igc0 inet6 from fd02:ca0b:f1cb:30::/64 to ! <nonglobal6> nat-to <nat66_egress> round-robin'
lladdr=' inet6 fe80::6662:66ff:fe25:ced9%igc0 prefixlen 64 scopeid 0x1'
lab=' inet6 fd02:ca0b:f1cb:20::1 prefixlen 64'
old=' inet6 2001:db8:a::10 prefixlen 64 autoconf pltime 9000 vltime 12000'
new=' inet6 2001:db8:b::10 prefixlen 64 autoconf pltime 10800 vltime 14400'
egress_case() {
 rm -f "$egress/log" "$egress/replace-fails"
 print -r -- "$1" > "$egress/rules"
 print -r -- "$2" > "$egress/igc0"
}
# No NAT66 rule loaded: nothing happens, an absent table stays absent.
rm -f "$egress/table"
egress_case 'pass all' "$lladdr
$lab
$new"
"$egress/run"
[[ ! -e $egress/table && ! -e $egress/log ]]
# Table exists but the loaded rules no longer use it: no refill.
print '2001:db8:a::10' > "$egress/table"
"$egress/run"
grep -qx '2001:db8:a::10' "$egress/table"
# One stable address; the lab ULA and link-local are never chosen.
rm -f "$egress/table"
egress_case "$nat_rule" "$lladdr
$lab
$old"
"$egress/run"
[[ $(cat "$egress/table") == 2001:db8:a::10 ]]
grep -q 'set to: 2001:db8:a::10' "$egress/log"
# Unchanged choice: no rewrite, no log line.
egress_case "$nat_rule" "$lladdr
$lab
$old"
"$egress/run"
[[ ! -e $egress/log ]]
# Flash renumbering without deprecation: both preferred, the refreshed new
# prefix has the longer valid lifetime and is the only table entry.
egress_case "$nat_rule" "$lladdr
$lab
$old
$new"
"$egress/run"
[[ $(cat "$egress/table") == 2001:db8:b::10 ]]
# Deprecated old plus new: the new one.
egress_case "$nat_rule" "$lladdr
 inet6 2001:db8:a::10 prefixlen 64 deprecated autoconf pltime 0 vltime 13000
$new"
"$egress/run"
[[ $(cat "$egress/table") == 2001:db8:b::10 ]]
# Equal lifetimes: the lowest address, deterministically.
egress_case "$nat_rule" " inet6 2001:db8:c::20 prefixlen 64 autoconf pltime 10800 vltime 14400
$new"
"$egress/run"
[[ $(cat "$egress/table") == 2001:db8:b::10 ]]
# Only temporary addresses: the table is flushed (fails closed).
egress_case "$nat_rule" "$lladdr
 inet6 2001:db8:b::99 prefixlen 64 autoconf temporary pltime 10800 vltime 14400"
"$egress/run"
[[ -e $egress/table && ! -s $egress/table ]]
grep -q 'set to: empty' "$egress/log"
# No global address at all: empty as well.
print '2001:db8:b::10' > "$egress/table"
egress_case "$nat_rule" "$lladdr
$lab"
"$egress/run"
[[ ! -s $egress/table ]]
# A rollback operation holds the journal lock: skip, change nothing.
egress_case "$nat_rule" "$new"
mkdir "$base/router-lock"
sleep 30 &
holder=$!
print -- "$holder" > "$base/router-lock/pid"
"$egress/run"
[[ ! -s $egress/table && -d $base/router-lock ]]
kill "$holder"
# A dead owner is stale: take over, update, release.
sleep 0 &
dead=$!
wait "$dead"
print -- "$dead" > "$base/router-lock/pid"
"$egress/run"
[[ $(cat "$egress/table") == 2001:db8:b::10 && ! -d $base/router-lock ]]
# A failed update exits non-zero and reaches syslog.
egress_case "$nat_rule" "$old"
touch "$egress/replace-fails"
if "$egress/run" 2>/dev/null; then print -u2 'egress failure not reported'; exit 1; fi
grep -q 'logger -p daemon.err -t nat66-egress cannot set <nat66_egress> to 2001:db8:a::10' "$egress/log"
[[ ! -d $base/router-lock ]]
# A young pidless lock is a starting holder: skip without deleting it. Over
# a minute old, its owner died before writing the pid: take over.
egress_case "$nat_rule" "$old"
mkdir "$base/router-lock"
"$egress/run"
[[ -d $base/router-lock && $(cat "$egress/table") == 2001:db8:b::10 ]]
touch -t 202601010000 "$base/router-lock"
"$egress/run"
[[ ! -d $base/router-lock && $(cat "$egress/table") == 2001:db8:a::10 ]]
print 'rollback fixture: passed (mocked mutations; real native config parsers)'
