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
   # Local-only candidates would deny RA/RS; the NAT66 stages pass the CPE's
   # (lab-final with its l24 if-bound state).
   if grep -Eq '^pass in quick on [$]up6_if inet6 proto icmp6 from fe80::1 .*icmp6-type routeradv( keep state [(]if-bound[)])?$' "$2"; then
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
for i in igc2 igc3; do print up > "$fixture/etc/hostname.$i"; done
# The generic journal checks below run on wifi-ipv6-nat66, the oldest
# stageable profile since g74 pruned the shared-wire stages. Arming it needs
# the current-cabling recovery on igc1 (live and persistent) and the NAT66
# egress helper with its timer (the crontab mock lists it).
print ' inet 10.98.0.1 netmask 0xffffff00' >> "$base/interfaces/igc1"
print 'up\ninet alias 10.98.0.1 255.255.255.0' > "$fixture/etc/hostname.igc1"
print '#!/bin/ksh' > "$base/nat66-egress"
chmod 700 "$base/nat66-egress"
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
"$fixture/run" stage trial0001 wifi-ipv6-nat66 180
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
expect_fail arm trial0001 wifi-ipv6-nat66 180
# The fixture root is mode 700 and the review is owned by the invoking root.
{
 print -- "candidate-sha256=$(cat "$base/router-state/digest")"
 print 'provisioning-clients=captured-inventory-no-known-clients'
 print 'dependencies=r14-reviewed-current-stage-only'
 print 'wg2=provisioned-and-tested'
 print 'old-management=fresh-authenticated'
 print 'current-cabling=wifi-ipv6-nat66'
} > "$base/router-readiness/wifi-ipv6-nat66"
chmod 600 "$base/router-readiness/wifi-ipv6-nat66"
mv "$fixture/etc/hostname.wg2" "$fixture/etc/wg2.saved"
expect_fail arm trial0001 wifi-ipv6-nat66 180
mv "$fixture/etc/wg2.saved" "$fixture/etc/hostname.wg2"
"$fixture/run" arm trial0001 wifi-ipv6-nat66 180
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
"$fixture/run" stage trialtbl1 wifi-ipv6-nat66 180
"$fixture/run" arm trialtbl1 wifi-ipv6-nat66 180
"$fixture/run" activate trialtbl1
print '198.51.100.10' > "$base/pf-tables/existing"
touch "$base/pf-tables/foreign"
expect_fail restore
[[ -f $base/router-state/pending && -f $base/pf-tables/foreign ]]
rm "$base/pf-tables/foreign"
"$fixture/run" restore
grep -qx '192.0.2.10' "$base/pf-tables/existing"
[[ ! -e $base/pf-tables/recorders && ! -e $base/pf-tables/camera_updates && ! -e $base/pf-tables/nat66_egress ]]
# A refused later stage must not strand the last restored snapshot. Recovery
# checks its token/integrity and archives the inactive stage before replaying.
"$fixture/run" stage trialtbl2 wifi-ipv6-nat66 180
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
"$fixture/run" stage trialauto1 wifi-ipv6-nat66 180
touch "$base/mutable-auto"
expect_fail arm trialauto1 wifi-ipv6-nat66 180
grep -q 'mutable optimizer PF table' "$fixture/failure.log"
[[ ! -f $base/router-state/pending ]]
rm "$base/mutable-auto"
touch "$base/unreferenced-auto"
expect_fail arm trialauto1 wifi-ipv6-nat66 180
grep -q 'unreferenced optimizer PF table' "$fixture/failure.log"
rm "$base/unreferenced-auto"
"$fixture/run" arm trialauto1 wifi-ipv6-nat66 180
grep -q '^table <__automatic_aabb_0> const' "$base/router-state/saved/pf.runtime"
/sbin/pfctl -nf "$base/router-state/saved/pf.runtime"
"$fixture/run" activate trialauto1
"$fixture/run" restore
grep -qx '192.0.2.0/24' "$base/pf-tables/__automatic_aabb_0"
rm "$base/pf-tables/__automatic_aabb_0"
# Unsupported resolver symlinks refuse before pending authorizes activation.
"$fixture/run" stage trialdns1 wifi-ipv6-nat66 180
mv "$fixture/etc/resolv.conf" "$fixture/resolv.saved"
ln -s "$fixture/resolv.saved" "$fixture/etc/resolv.conf"
expect_fail arm trialdns1 wifi-ipv6-nat66 180
[[ ! -f $base/router-state/pending ]]
rm "$fixture/etc/resolv.conf"
ln -s "$fixture/absent-resolver-target" "$fixture/etc/resolv.conf"
expect_fail arm trialdns1 wifi-ipv6-nat66 180
[[ ! -f $base/router-state/pending ]]
rm "$fixture/etc/resolv.conf"
mv "$fixture/resolv.saved" "$fixture/etc/resolv.conf"
# Native boot can start resolvd without an rc.d runtime cache. Unknown process
# arguments refuse, while the exact no-argument daemon captures empty flags.
mv "$fixture/var/run/rc.d/resolvd" "$fixture/resolvd.runtime.saved"
expect_fail arm trialdns1 wifi-ipv6-nat66 180
[[ ! -f $base/router-state/pending ]]
touch "$base/native-resolvd"
"$fixture/run" arm trialdns1 wifi-ipv6-nat66 180
[[ ! -s $base/router-state/saved/services/resolvd.flags || $(cat "$base/router-state/saved/services/resolvd.flags") == '' ]]
"$fixture/run" restore
rm "$base/native-resolvd"
mv "$fixture/resolvd.runtime.saved" "$fixture/var/run/rc.d/resolvd"
# An absent file and a stopped resolver must round-trip without starting it.
touch "$base/resolvd-stopped"
rm "$fixture/etc/resolv.conf"
"$fixture/run" stage trialdns2 wifi-ipv6-nat66 180
"$fixture/run" arm trialdns2 wifi-ipv6-nat66 180
"$fixture/run" activate trialdns2
: > "$base/operations"
"$fixture/run" restore
[[ ! -e $fixture/etc/resolv.conf ]]
if grep -qx 'rcctl -f start resolvd' "$base/operations"; then exit 1; fi
rm "$base/resolvd-stopped"
cp -p "$fixture/resolv.expected" "$fixture/etc/resolv.conf"
"$fixture/run" stage trial0002 wifi-ipv6-nat66 180
"$fixture/run" arm trial0002 wifi-ipv6-nat66 180
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
"$fixture/run" stage trial0003 wifi-ipv6-nat66 180
"$fixture/run" arm trial0003 wifi-ipv6-nat66 180
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
"$fixture/run" stage trial0004 wifi-ipv6-nat66 180
print 'extra=unknown' >> "$base/router-readiness/wifi-ipv6-nat66"
expect_fail arm trial0004 wifi-ipv6-nat66 180
sed '$d' "$base/router-readiness/wifi-ipv6-nat66" > "$base/review.next"
mv "$base/review.next" "$base/router-readiness/wifi-ipv6-nat66"
chmod 600 "$base/router-readiness/wifi-ipv6-nat66"
print '# changed after freeze' >> "$base/router-state/candidate/pf.conf"
expect_fail arm trial0004 wifi-ipv6-nat66 180
"$fixture/run" stage trial0005 wifi-ipv6-nat66 180
print 'TEMPLATE' >> "$fixture/etc/hostname.wg2"
expect_fail arm trial0005 wifi-ipv6-nat66 180
sed '$d' "$fixture/etc/hostname.wg2" > "$fixture/etc/wg2.next"
mv "$fixture/etc/wg2.next" "$fixture/etc/hostname.wg2"
# On a rollback, recovery is never deleted or moved to upstream. The snapshot
# is the last accepted profile with recovery already on igc1.
"$fixture/run" stage trial0006 wifi-ipv6-nat66 180
"$fixture/run" arm trial0006 wifi-ipv6-nat66 180
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
# The baseline Wi-Fi port has no IPv6 family at all.
print 'igc2: flags=<UP,BROADCAST>\n inet 10.30.0.1 netmask 0xffffff00' > "$base/interfaces/igc2"
# NAT66 keeps SLAAC on igc0: the trial learns a stable address (a temporary
# one would refuse acceptance), the restore relearns the temporary baseline.
print ' inet6 2001:db8:1::77 prefixlen 64 autoconf pltime 100 vltime 200' > "$base/slaac-addresses"
"$fixture/run" stage ipv6trial wifi-ipv6-nat66 180
"$fixture/run" arm ipv6trial wifi-ipv6-nat66 180
"$fixture/run" activate ipv6trial
print ' inet6 2001:db8:1::5678 prefixlen 64 autoconf temporary pltime 100 vltime 200' > "$base/slaac-addresses"
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
# No SLAAC address is offered here; the rendered profile is put back after.
rm -f "$base/slaac-addresses"
cp -Rp "$base/router-profiles/wifi-ipv6-nat66" "$fixture/nat66.plain"
print 'inet alias 10.20.0.254 255.255.255.0' >> "$base/router-profiles/wifi-ipv6-nat66/hostname.igc0"
print '# gonf boot canary: 10.20.0.254' >> "$base/router-profiles/wifi-ipv6-nat66/ntpd.conf"
"$fixture/run" stage boottrial1 wifi-ipv6-nat66 1800
sed "1s/.*/candidate-sha256=$(cat "$base/router-state/digest")/" "$base/router-readiness/wifi-ipv6-nat66" > "$base/review.next"
mv "$base/review.next" "$base/router-readiness/wifi-ipv6-nat66"
chmod 600 "$base/router-readiness/wifi-ipv6-nat66"
"$fixture/run" arm boottrial1 wifi-ipv6-nat66 1800
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
rm -rf "$base/router-profiles/wifi-ipv6-nat66" "$base/reboot-requested"
cp -Rp "$fixture/nat66.plain" "$base/router-profiles/wifi-ipv6-nat66"
# o34 NAT66 (wifi-ipv6-nat66): since g74 the oldest stageable profile. Start
# from the local-only wifi-first wiring that preceded it (that profile was
# pruned, so its files are written here): lab and cameras on igc0 with the
# ISP LAN, Wi-Fi on igc2, recovery on igc1, no SLAAC, no Wi-Fi ULA, no rad.
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
{
 print 'inet 192.168.1.2 255.255.255.0'
 print 'inet alias 10.20.0.1 255.255.255.0'
 print 'inet alias 10.40.0.1 255.255.255.0'
 print 'inet6 alias fd02:ca0b:f1cb:20::1 64'
 print 'inet6 -autoconf'
 print 'inet6 -temporary'
} > "$fixture/etc/hostname.igc0"
print 'up\ninet alias 10.98.0.1 255.255.255.0' > "$fixture/etc/hostname.igc1"
print 'up\ninet alias 10.30.0.1 255.255.255.0' > "$fixture/etc/hostname.igc2"
print up > "$fixture/etc/hostname.igc3"
print 'net.inet.ip.forwarding=1\nnet.inet6.ip6.forwarding=0\nkern.maxfiles=12345' > "$fixture/etc/sysctl.conf"
print 'set state-policy floating\npass all' > "$fixture/etc/pf.conf"
rm -rf "$base/pf-tables"
rm -f "$fixture/etc/rad.conf" "$base/reboot-requested" "$base/nat66-egress" "$base/slaac-addresses"
cp -p "$base/interfaces/igc0" "$fixture/igc0.base"
cp -p "$base/interfaces/igc2" "$fixture/igc2.base"
nat=$base/router-profiles/wifi-ipv6-nat66
# Every misplaced o34 element refuses at stage, before any journal change.
cp -Rp "$nat" "$fixture/nat66.saved"
o34_refuses() {
 expect_fail stage o34bad01 "$1" 180
 grep -q "$2" "$fixture/failure.log" || { print -u2 -- "wrong refusal for $1: $2"; cat "$fixture/failure.log" >&2; exit 1; }
 rm -rf "$nat"
 cp -Rp "$fixture/nat66.saved" "$nat"
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
# The local-only policy lines instead of SLAAC on the uplink, or beside it.
awk '$0 == "inet6 autoconf -temporary" {print "inet6 -autoconf"; print "inet6 -temporary"; next} {print}' "$nat/hostname.igc0" > "$nat/igc0.next"; mv "$nat/igc0.next" "$nat/hostname.igc0"
o34_refuses wifi-ipv6-nat66 'unexpected IPv6 autoconfiguration line'
print 'inet6 -temporary' >> "$nat/hostname.igc0"
o34_refuses wifi-ipv6-nat66 'persistent IPv6 policy missing or duplicated'
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
# A temporary SLAAC address after activation refuses acceptance; the timer
# then restores the local-only baseline, including the persistent policy.
print ' inet6 2001:db8:5::77 prefixlen 64 autoconf temporary pltime 100 vltime 200' > "$base/slaac-addresses"
"$fixture/run" arm o34nat01 wifi-ipv6-nat66 180
expect_fail activate o34nat01
grep -q 'unexpected physical IPv6 address' "$fixture/failure.log"
[[ $(cat "$base/router-state/status") == activating ]]
print 0 > "$base/router-state/deadline"
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored && ! -f $base/router-state/pending ]]
if grep -q -e AUTOCONF6 -e 2001:db8:5 "$base/interfaces/igc0"; then exit 1; fi
grep -q 'inet6 fd02:ca0b:f1cb:20::1 prefixlen 64' "$base/interfaces/igc0"
grep -qx 'inet6 -autoconf' "$fixture/etc/hostname.igc0"
grep -qx 'inet6 -temporary' "$fixture/etc/hostname.igc0"
[[ ! -e $fixture/etc/rad.conf ]]
# NAT66: stable SLAAC address and solicitation only after its PF, the Wi-Fi
# ULA, rad, IPv6 forwarding and the egress table.
print ' inet6 2001:db8:5::7 prefixlen 64 autoconf pltime 100 vltime 200' > "$base/slaac-addresses"
"$fixture/run" stage o34nat02 wifi-ipv6-nat66 180
review wifi-ipv6-nat66
"$fixture/run" arm o34nat02 wifi-ipv6-nat66 180
: > "$base/operations"
"$fixture/run" activate o34nat02
grep -qx 'ifconfig igc0 inet6 autoconf -temporary' "$base/operations"
if grep -q 'slaacctl send solicitation igc[123]' "$base/operations"; then exit 1; fi
awk '/^pfctl -f .*candidate\/pf.conf$/ {pf=NR} /^slaacctl send solicitation igc0$/ {rs=NR} END {exit !(pf && rs > pf)}' "$base/operations"
grep -q 'AUTOCONF6,' "$base/interfaces/igc0"
grep -q 'inet6 2001:db8:5::7 .*autoconf' "$base/interfaces/igc0"
grep -qx 'inet6 autoconf -temporary' "$fixture/etc/hostname.igc0"
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
# Acceptance refuses a temporary address or temporary addressing on the
# uplink and autoconfiguration on any other port.
cp -p "$base/interfaces/igc0" "$fixture/igc0.trial"
cp -p "$base/interfaces/igc2" "$fixture/igc2.trial"
print ' inet6 2001:db8:5::99 prefixlen 64 autoconf temporary pltime 100 vltime 200' >> "$base/interfaces/igc0"
expect_fail check_candidate_ipv6
cp -p "$fixture/igc0.trial" "$base/interfaces/igc0"
sed 's/flags=</flags=<AUTOCONF6TEMP,/' "$fixture/igc0.trial" > "$base/interfaces/igc0"
expect_fail check_candidate_ipv6
cp -p "$fixture/igc0.trial" "$base/interfaces/igc0"
sed 's/flags=</flags=<AUTOCONF6,/' "$fixture/igc2.trial" > "$base/interfaces/igc2"
expect_fail check_candidate_ipv6
cp -p "$fixture/igc2.trial" "$base/interfaces/igc2"
"$fixture/run" check_candidate_ipv6
# Timed restore returns to the local-only baseline: rad, forwarding, SLAAC,
# Wi-Fi IPv6, rad.conf and the NAT table go.
print 0 > "$base/router-state/deadline"
: > "$base/operations"
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored && ! -f $base/router-state/pending ]]
[[ ! -e $fixture/etc/rad.conf && ! -e $base/pf-tables/nat66_egress ]]
grep -qx 'rcctl stop rad' "$base/operations"
grep -qx 'sysctl net.inet.ip.forwarding=1 net.inet6.ip6.forwarding=0' "$base/operations"
grep -qx 'ifconfig igc2 -inet6' "$base/operations"
if grep -q -e 'inet6 ' "$base/interfaces/igc2"; then exit 1; fi
if grep -q -e 'rcctl -f start rad' -e 'slaacctl send solicitation' "$base/operations"; then exit 1; fi
if grep -q -e AUTOCONF6 -e 2001:db8:5 "$base/interfaces/igc0"; then exit 1; fi
grep -qx 'inet6 -autoconf' "$fixture/etc/hostname.igc0"
if grep -q 'f1cb:30' "$fixture/etc/hostname.igc2"; then exit 1; fi
grep -qx 'net.inet6.ip6.forwarding=0' "$fixture/etc/sysctl.conf"
grep -qx 'kern.maxfiles=12345' "$fixture/etc/sysctl.conf"
# Boot restoration of an unconfirmed NAT66 trial restores the persistent
# local-only files before netstart, rc.conf.local and sysctl included.
"$fixture/run" stage o34nat03 wifi-ipv6-nat66 180
review wifi-ipv6-nat66
"$fixture/run" arm o34nat03 wifi-ipv6-nat66 180
"$fixture/run" activate o34nat03
"$fixture/run" boot_restore
[[ $(cat "$base/router-state/status") == restored-at-boot && ! -f $base/router-state/pending ]]
[[ ! -e $fixture/etc/rad.conf ]]
grep -qx 'inet6 -autoconf' "$fixture/etc/hostname.igc0"
if grep -q 'f1cb:30' "$fixture/etc/hostname.igc2"; then exit 1; fi
grep -qx 'net.inet6.ip6.forwarding=0' "$fixture/etc/sysctl.conf"
cmp -s "$base/router-state/saved/files$fixture/etc/rc.conf.local" "$fixture/etc/rc.conf.local"
# A real reboot drops runtime-only state; the mocks keep it, so reset it.
cp -p "$fixture/igc0.base" "$base/interfaces/igc0"
cp -p "$fixture/igc2.base" "$base/interfaces/igc2"
rm -rf "$base/pf-tables"
# Journals snapshotted before rad.conf ownership omit it from the manifest:
# a NAT66 candidate (every stageable profile since g74) then refuses.
strip_rad_manifest() {
 # rad.conf may be the only absent entry; an empty remainder is valid.
 grep -vxF "$fixture/etc/rad.conf" "$base/router-state/saved/absent" > "$fixture/absent.next" || :
 mv "$fixture/absent.next" "$base/router-state/saved/absent"
 "$fixture/run" atomic "$base/router-state/snapshot-digest" "$("$fixture/run" checksum "$base/router-state/saved")"
}
"$fixture/run" stage o34leg01 wifi-ipv6-nat66 180
review wifi-ipv6-nat66
"$fixture/run" arm o34leg01 wifi-ipv6-nat66 180
grep -qxF "$fixture/etc/rad.conf" "$base/router-state/saved/absent"
"$fixture/run" activate o34leg01
strip_rad_manifest
expect_fail restore
grep -q 'incomplete saved file manifest' "$fixture/failure.log"
[[ -f $base/router-state/pending && -e $fixture/etc/rad.conf ]]
print -r -- "$fixture/etc/rad.conf" >> "$base/router-state/saved/absent"
"$fixture/run" atomic "$base/router-state/snapshot-digest" "$("$fixture/run" checksum "$base/router-state/saved")"
"$fixture/run" restore
[[ ! -e $fixture/etc/rad.conf && ! -f $base/router-state/pending ]]
# A confirmed NAT66 profile keeps its egress table across a reapply: the
# snapshot captures the helper's address and a restore puts it back.
"$fixture/run" stage o34fwd01 wifi-ipv6-nat66 180
review wifi-ipv6-nat66
"$fixture/run" arm o34fwd01 wifi-ipv6-nat66 180
"$fixture/run" activate o34fwd01
confirm_fresh o34fwd01 46021 46022
[[ -f $base/pf-tables/nat66_egress ]]
print '2001:db8:5::7' > "$base/pf-tables/nat66_egress"
"$fixture/run" stage o34fwd02 wifi-ipv6-nat66 180
review wifi-ipv6-nat66
"$fixture/run" arm o34fwd02 wifi-ipv6-nat66 180
grep -qx '2001:db8:5::7' "$base/router-state/saved/tables/nat66_egress"
"$fixture/run" activate o34fwd02
print '2001:db8:5::8' > "$base/pf-tables/nat66_egress"
"$fixture/run" restore
grep -qx '2001:db8:5::7' "$base/pf-tables/nat66_egress"
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
 "$fixture/run" stage "$1" wifi-ipv6-nat66 180
 review wifi-ipv6-nat66
 "$fixture/run" arm "$1" wifi-ipv6-nat66 180
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
# An unlisted profile name in the journal fails closed instead of using igc0.
print camera-igc4 > "$base/router-state/profile"
expect_fail check_candidate_ipv6
grep -q 'profile has no reviewed camera port' "$fixture/failure.log"
print camera-igc3 > "$base/router-state/profile"
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
# newly rendered profile cannot miss profile_ok or the wiring table.
rendered=0
for d in "$base"/router-profiles/*/; do
 p=${d%/}
 p=${p##*/}
 "$fixture/run" profile_ok "$p"
 "$fixture/run" wiring "$p" > /dev/null
 (( rendered += 1 ))
done
# The four stageable physical profiles (g74 pruned every other one).
(( rendered == 4 ))
[[ -d $base/router-profiles/lab-igc1 && -d $base/router-profiles/lab-final ]]
[[ $("$fixture/run" wiring lab-final) == 'igc1 igc2 igc3' && $("$fixture/run" camera_port lab-final) == igc3 ]]
[[ $("$fixture/run" wiring lab-igc1) == 'igc1 igc2 igc3' && $("$fixture/run" camera_port lab-igc1) == igc3 ]]
[[ $("$fixture/run" wiring camera-igc3) == 'igc0 igc2 igc3' && $("$fixture/run" camera_port camera-igc3) == igc3 ]]
[[ $("$fixture/run" wiring wifi-ipv6-nat66) == 'igc0 igc2 igc0' && $("$fixture/run" camera_port wifi-ipv6-nat66) == igc0 ]]
# The pruned profiles (g74) and any other name are neither stageable nor wired.
for p in shared-igc0 wifi-prepare wifi-first post-rack post-ap final wifi-ipv6-uplink vpn-bootstrap camera-igc4 ''; do
 expect_fail profile_ok "$p"
 expect_fail camera_port "$p"
 [[ ! -s $fixture/failure.log ]]
 expect_fail stage g74gone1 "$p" 180
 grep -q 'invalid stage profile' "$fixture/failure.log"
done
# A bundle under an unlisted name reaches the wiring check. Its sysctl
# fragment is the non-NAT66 one, which an unlisted name expects.
cp -Rp "$cam" "$fixture/unlisted"
print 'net.inet.ip.forwarding=1\nnet.inet6.ip6.forwarding=0' > "$fixture/unlisted/sysctl.conf.fragment"
expect_fail validate "$fixture/unlisted" post-ap
grep -q 'profile has no reviewed port wiring' "$fixture/failure.log"
rm -rf "$fixture/unlisted"
# Every rendered stageable profile passes the tightened validator unchanged.
for d in "$base"/router-profiles/*/; do
 p=${d%/}
 p=${p##*/}
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
# The local-only policy lines live on lab-igc1's lab port igc1.
li=$base/router-profiles/lab-igc1
# 10.30.0.1 is bound to igc2 by address and by the PF macro, not merely once.
edit '/^inet alias 10[.]30[.]0[.]1 /d' "$nat/hostname.igc2"
print 'inet alias 10.30.0.1 255.255.255.0' >> "$nat/hostname.igc0"
i44_refuses wifi-ipv6-nat66 'Wi-Fi zone on the wrong port'
edit 's/^wifi_if = "igc2"$/wifi_if = "igc0"/' "$nat/pf.conf"
i44_refuses wifi-ipv6-nat66 'Wi-Fi zone on the wrong port'
edit '/^inet alias 10[.]30[.]0[.]1 /d' "$nat/hostname.igc2"
print 'inet alias 10.30.0.1 255.255.255.0' >> "$nat/hostname.igc3"
i44_refuses wifi-ipv6-nat66 'Wi-Fi zone on the wrong port'
edit '/^inet alias 10[.]30[.]0[.]1 /d' "$cam/hostname.igc2"
print 'inet alias 10.30.0.1 255.255.255.0' >> "$cam/hostname.igc0"
i44_refuses camera-igc3 'Wi-Fi zone on the wrong port'
edit '/^inet alias 10[.]20[.]0[.]1 /d' "$nat/hostname.igc0"
print 'inet alias 10.20.0.1 255.255.255.0' >> "$nat/hostname.igc2"
i44_refuses wifi-ipv6-nat66 'lab zone on the wrong port'
edit '/^inet6 /d' "$nat/hostname.igc0"
print 'inet6 alias fd02:ca0b:f1cb:20::1 64\ninet6 -autoconf\ninet6 -temporary' >> "$nat/hostname.igc3"
i44_refuses wifi-ipv6-nat66 'lab ULA on the wrong port'
# DHCP follows the stage: igc2 only before b24.
edit 's/^dhcpd_interfaces=igc2$/dhcpd_interfaces=disabled/' "$nat/services.conf"
i44_refuses wifi-ipv6-nat66 'wrong Wi-Fi DHCP binding'
# netstart turns a third word into a prefix length: the joined policy line refuses.
edit '/^inet6 -temporary$/d' "$li/hostname.igc1"
edit 's/^inet6 -autoconf$/inet6 -autoconf -temporary/' "$li/hostname.igc1"
i44_refuses lab-igc1 'netstart misparses'
edit '/^inet6 -temporary$/d' "$li/hostname.igc1"
i44_refuses lab-igc1 'persistent IPv6 policy missing or duplicated'
print 'inet6 -autoconf' >> "$li/hostname.igc2"
i44_refuses lab-igc1 'persistent IPv6 policy missing or duplicated'
edit '/^inet6 -/d' "$li/hostname.igc1"
print 'inet6 -autoconf\ninet6 -temporary' >> "$li/hostname.igc2"
i44_refuses lab-igc1 'IPv6 policy must accompany the lab address'
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
# l24: only the rule header names the interface. Quoted text saying
# "on igc0" (a label or tag) does not make an interface-less permit concrete.
cam_before_deny 'pass in quick inet proto tcp to port 8080 label "x on igc0 y"'
i44_refuses camera-igc3 'camera zone deny must precede every permit'
cam_before_deny 'pass in quick inet proto tcp to port 8080 tag "on igc0"'
i44_refuses camera-igc3 'PF tag names must be single words'
cam_before_deny 'pass in log (all, to pflog0) quick inet proto tcp to port 8080'
i44_refuses camera-igc3 'camera zone deny must precede every permit'
# Anchors refuse anywhere: rules loaded into them at runtime could permit the
# port, empty or not, and this helper does not own their loaders.
cam_before_deny 'pass in quick on igc2 inet proto tcp to port 8080 tag "x no state"'
i44_refuses camera-igc3 'PF tag names must be single words'
cam_before_deny 'pass in quick on igc2 inet proto tcp to port 8080 !tagged "x no state"'
i44_refuses camera-igc3 'PF tag names must be single words'
cam_before_deny "pass in quick on igc2 inet proto tcp to port 8080 tag 'x no state'"
i44_refuses camera-igc3 'PF tag names must be single words'
cam_before_deny 'anchor "l24"'
i44_refuses camera-igc3 'PF anchors are not supported in candidates'
print 'anchor "l24" in on igc2 {\n pass in quick on igc2 inet proto tcp to port 8080\n}' >> "$cam/pf.conf"
i44_refuses camera-igc3 'PF anchors are not supported in candidates'
# Supernets, ranges, admitting negations and tables that name the zone.
print 'pass in on wg2 inet proto tcp from 10.99.0.2 to 10.0.0.0/8 port 22' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone must stay default-deny'
print 'pass in on wg2 inet proto tcp from 10.99.0.2 to 10.39.255.0 - 10.40.0.20 port 22' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone must stay default-deny'
print 'pass in on wg2 inet proto tcp from 10.99.0.2 to ! 10.20.0.0/24 port 22' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone must stay default-deny'
print 'table <l24big> const { 10.0.0.0/8 }\npass in on wg2 inet proto tcp from 10.99.0.2 to <l24big> port 22' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone must stay default-deny'
print 'table <l24dyn> persist\npass in on wg2 inet proto tcp from 10.99.0.2 to <l24dyn> port 22' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone must stay default-deny'
print 'table <l24lab> const { 10.20.0.0/24 }\npass in on wg2 inet proto tcp from 10.99.0.2 to ! <l24lab> port 22' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone must stay default-deny'
print 'table <l24hole> const { 10.0.0.0/8, !10.40.0.0/26 }\npass in on wg2 inet proto tcp from 10.99.0.2 to ! <l24hole> port 22' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone must stay default-deny'
print 'match out on igc0 inet from 10.0.0.0/8 to ! <nonpublic> nat-to 192.168.1.2' >> "$cam/pf.conf"
i44_refuses camera-igc3 'no NAT for the camera zone'
# A deny after the interface-less transit permit no longer shields the port.
edit '/^block drop [a-z]* log quick on [$]cam_if$/d' "$cam/pf.conf"
# shellcheck disable=SC2016
print 'block drop in log quick on $cam_if\nblock drop out log quick on $cam_if' >> "$cam/pf.conf"
i44_refuses camera-igc3 'camera zone deny must precede every permit'
# Controls: a concrete other port before the deny, and a public 110.40.0.0/16
# address after it, are not camera rules.
cam_before_deny 'pass in quick on igc2 inet proto tcp from 10.30.0.0/24 to 10.30.0.1 port 8080'
cam_before_deny 'pass in log (all, to pflog0) quick on igc2 inet proto tcp from 10.30.0.0/24 to 10.30.0.1 port 8081 tag "gonf_transit"'
# l24 controls: quoted zone text, a negated table or prefix that excludes the
# whole zone, and ranges and prefixes beside it are not camera rules.
{
 print 'pass in on wg2 inet proto tcp from 10.99.0.2 to 110.40.0.1 port 22'
 print 'pass in on wg2 inet proto tcp from 10.99.0.2 to 10.20.0.1 port 22 label "10.40.0.11 on igc3"'
 print 'table <l24big> const { 10.0.0.0/8 }\npass in on wg2 inet proto tcp from 10.99.0.2 to ! <l24big> port 22'
 print 'pass in on wg2 inet proto tcp from 10.99.0.2 to ! 10.0.0.0/8 port 22'
 print 'pass in on wg2 inet proto tcp from 10.99.0.2 to 10.41.0.0 - 10.41.0.9 port 22'
 print 'pass in on wg2 inet proto tcp from 10.99.0.2 to 10.41.0.0/16 port 22'
} >> "$cam/pf.conf"
"$fixture/run" validate "$cam" camera-igc3
rm -rf "$base/router-profiles"
cp -Rp "$fixture/profiles.saved" "$base/router-profiles"
# c24 lab-igc1: camera-igc3 with 10.20.0.1/24, the lab ULA and their routes
# moved from igc0 to igc1 after the b24 cable move. Start from a confirmed
# camera-igc3 baseline, as on the live router (the journal holds the restored
# g24fwd01 trial, the camera zone already on igc3).
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
# j24: lab-igc1 gives the ISP LAN no router management; the stages before
# b24 keep both old-management rules. No profile keeps 192.168.11.2 or the
# f0 -> AP migration flow (both retired from the inventory).
if grep -q 'from 192.168.1.0/24' "$lab/pf.conf"; then exit 1; fi
for r in 'pass in on igc0 inet proto tcp from 192.168.1.0/24 to 192.168.1.2 port 22 keep state' \
 'pass in on igc0 inet proto icmp from 192.168.1.0/24 to 192.168.1.2 keep state'; do
 grep -qx -- "$r" "$cam/pf.conf"
done
grep -qx 'pass in on { igc0 $wifi_if } inet proto udp to { 192.168.1.2, 10.30.0.1 } port 51822 keep state' "$lab/pf.conf"
if grep -q '192[.]168[.]11[.]' "$base"/router-profiles/*/hostname.igc0; then exit 1; fi
if grep -Eq 'to 10[.]30[.]0[.]2 port' "$base"/router-profiles/*/pf.conf; then exit 1; fi
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
# The split policy reaches ifconfig as netstart would issue it at boot (i44).
grep -qx 'ifconfig igc1 inet6 -autoconf' "$base/operations"
grep -qx 'ifconfig igc1 inet6 -temporary' "$base/operations"
if grep -q prefixlen.-temporary "$base/operations"; then exit 1; fi
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
grep -q 'lab-port new and recovery receipts must come from a lab host' "$fixture/failure.log"
[[ ! -e $base/router-state/proof.new ]]
print 'tcp 0 0 10.20.0.1.22 10.20.0.125.47212 ESTABLISHED' > "$base/sockets"
"$fixture/run" proof new c24lab01 '10.20.0.125 47212 10.20.0.1 22'
expect_fail confirm c24lab01 services-checked
grep -q 'fresh igc1 recovery receipt missing' "$fixture/failure.log"
# A recovery tuple for the wrong management address is refused.
print 'tcp 0 0 10.20.0.1.22 10.98.0.125.47215 ESTABLISHED' > "$base/sockets"
expect_fail proof recovery c24lab01 '10.98.0.125 47215 10.20.0.1 22'
grep -q 'wrong management address' "$fixture/failure.log"
print 'tcp 0 0 10.98.0.1.22 10.99.0.2.47213 ESTABLISHED' > "$base/sockets"
expect_fail proof recovery c24lab01 '10.99.0.2 47213 10.98.0.1 22'
grep -q 'lab-port new and recovery receipts must come from a lab host' "$fixture/failure.log"
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
# j24 lab-final: lab-igc1 without the igc1 recovery subnet. Any recovery
# leftover (address, PF exception, DHCP subnet) refuses at stage.
fin=$base/router-profiles/lab-final
"$fixture/run" validate "$fin" lab-final
if grep -q '10[.]98[.]' "$fin"/hostname.igc[0-3] "$fin/pf.conf" "$fin/dhcpd.conf"; then exit 1; fi
cp -Rp "$fin" "$fixture/final.saved"
j24_refuses() {
 expect_fail stage j24bad01 lab-final 180
 grep -qF -- "$1" "$fixture/failure.log" || { print -u2 -- "wrong refusal: $1"; cat "$fixture/failure.log" >&2; exit 1; }
 [[ $(cat "$base/router-state/token") == c24lab03 && $(cat "$base/router-state/status") == confirmed ]]
 rm -rf "$fin"; cp -Rp "$fixture/final.saved" "$fin"
}
# l24: every stateful pass of lab-final keeps an if-bound state, so a device
# on the camera port cannot match states created on another port. The global
# policy stays floating (the arm snapshot restores it with the loaded rules).
grep -qx 'set state-policy floating' "$fin/pf.conf"
awk '/^pass / && !/ keep state [(]if-bound[)]$/ {bad=1} END {exit bad}' "$fin/pf.conf"
edit 's/^\(pass out on igc0 inet from 192[.]168[.]1[.]2 to ! <nonpublic>\) keep state (if-bound)$/\1 keep state/' "$fin/pf.conf"
grep -qx 'pass out on igc0 inet from 192.168.1.2 to ! <nonpublic> keep state' "$fin/pf.conf"
j24_refuses 'every stateful pass of this profile must keep if-bound states'
# PF's default is keep state: a pass without a state keyword is floating too.
print 'pass in on wg2 inet proto tcp from 10.99.0.2 to 10.20.0.1 port 22' >> "$fin/pf.conf"
j24_refuses 'every stateful pass of this profile must keep if-bound states'
print 'pass in on wg2 inet proto tcp from 10.99.0.2 to 10.20.0.1 port 22 label "keep state (if-bound)"' >> "$fin/pf.conf"
j24_refuses 'every stateful pass of this profile must keep if-bound states'
print 'pass in on wg2 inet proto tcp from 10.99.0.2 to 10.20.0.1 port 22 keep state (floating)' >> "$fin/pf.conf"
j24_refuses 'every stateful pass of this profile must keep if-bound states'
# Controls: stateless passes and if-bound beside other state options.
print 'pass in on wg2 inet proto udp from 10.99.0.2 to 10.20.0.1 port 53 no state' >> "$fin/pf.conf"
print 'pass in on wg2 inet proto tcp from 10.99.0.2 to 10.20.0.1 port 22 keep state (max 100, if-bound)' >> "$fin/pf.conf"
"$fixture/run" validate "$fin" lab-final
rm -rf "$fin"; cp -Rp "$fixture/final.saved" "$fin"
# The older profiles keep their accepted floating states.
if grep -q 'if-bound' "$base/router-profiles/lab-igc1/pf.conf" "$base/router-profiles/camera-igc3/pf.conf"; then exit 1; fi
print 'inet alias 10.98.0.1 255.255.255.0' >> "$fin/hostname.igc1"
j24_refuses 'final profile keeps a recovery leftover'
print 'pass in on wg2 inet proto tcp from 10.99.0.2 to 10.98.0.0/24 port 22 keep state (if-bound)' >> "$fin/pf.conf"
j24_refuses 'final profile keeps a recovery leftover'
print 'subnet 10.98.0.0 netmask 255.255.255.0 { deny unknown-clients; }' >> "$fin/dhcpd.conf"
j24_refuses 'final profile keeps a recovery leftover'
"$fixture/run" stage j24fin01 lab-final 180
review lab-final isp-uplink-only
"$fixture/run" arm j24fin01 lab-final 180
: > "$base/operations"
"$fixture/run" activate j24fin01
# The recovery address leaves igc1; the lab gateway, ULA and upstream stay.
if grep -q ' inet 10.98.0.1 ' "$base/interfaces/igc1"; then exit 1; fi
grep -qx 'ifconfig igc1 inet 10.98.0.1 delete' "$base/operations"
grep -q ' inet 10.20.0.1 ' "$base/interfaces/igc1"
grep -q 'inet6 fd02:ca0b:f1cb:20::1 ' "$base/interfaces/igc1"
grep -q ' inet 192.168.1.2 ' "$base/interfaces/igc0"
grep -qx 'rcctl set dhcpd flags igc1 igc2' "$base/operations"
for f in hostname.igc0 hostname.igc1 hostname.igc2 hostname.igc3 pf.conf dhcpd.conf rad.conf; do cmp -s "$fin/$f" "$fixture/etc/$f"; done
# Confirmation needs a lab-host new receipt and the VPN receipt, no recovery one.
print '' > "$base/router-state/preexisting-connections"
print 'tcp 0 0 10.20.0.1.22 10.99.0.2.47231 ESTABLISHED' > "$base/sockets"
expect_fail proof new j24fin01 '10.99.0.2 47231 10.20.0.1 22'
grep -q 'lab-port new and recovery receipts must come from a lab host' "$fixture/failure.log"
cat > "$base/sockets" <<'SOCKETS'
tcp 0 0 10.20.0.1.22 10.20.0.125.47232 ESTABLISHED
tcp 0 0 10.99.0.1.22 10.99.0.2.47233 ESTABLISHED
SOCKETS
"$fixture/run" proof new j24fin01 '10.20.0.125 47232 10.20.0.1 22'
expect_fail confirm j24fin01 services-checked
grep -q 'fresh VPN receipt missing' "$fixture/failure.log"
# Timed restore returns lab-igc1, with the recovery address back on igc1.
print 0 > "$base/router-state/deadline"
"$fixture/run" tick
[[ $(cat "$base/router-state/status") == restored && ! -f $base/router-state/pending ]]
grep -q ' inet 10.98.0.1 ' "$base/interfaces/igc1"
grep -q ' inet 10.20.0.1 ' "$base/interfaces/igc1"
for f in hostname.igc0 hostname.igc1 hostname.igc2 hostname.igc3 pf.conf dhcpd.conf rad.conf; do cmp -s "$fixture/lab.saved/$f" "$fixture/etc/$f"; done
# Confirmed with new and VPN receipts.
"$fixture/run" stage j24fin02 lab-final 180
review lab-final isp-uplink-only
"$fixture/run" arm j24fin02 lab-final 180
"$fixture/run" activate j24fin02
print '' > "$base/router-state/preexisting-connections"
cat > "$base/sockets" <<'SOCKETS'
tcp 0 0 10.20.0.1.22 10.20.0.125.47241 ESTABLISHED
tcp 0 0 10.99.0.1.22 10.99.0.2.47242 ESTABLISHED
SOCKETS
# Neither receipt alone confirms; a recovery receipt is refused outright.
"$fixture/run" proof vpn j24fin02 '10.99.0.2 47242 10.99.0.1 22'
expect_fail confirm j24fin02 services-checked
grep -q 'fresh old/new receipts missing' "$fixture/failure.log"
print 'tcp 0 0 10.98.0.1.22 10.98.0.125.47243 ESTABLISHED' >> "$base/sockets"
expect_fail proof recovery j24fin02 '10.98.0.125 47243 10.98.0.1 22'
grep -q 'profile has no recovery address; no recovery receipt' "$fixture/failure.log"
[[ ! -e $base/router-state/proof.recovery ]]
"$fixture/run" proof new j24fin02 '10.20.0.125 47241 10.20.0.1 22'
"$fixture/run" confirm j24fin02 services-checked
[[ $(cat "$base/router-state/status") == confirmed && ! -f $base/router-state/pending ]]
if grep -q ' inet 10.98.0.1 ' "$base/interfaces/igc1"; then exit 1; fi
# Without live recovery, lab-final re-arms (reapply and boot test) while a
# recovery profile refuses: it would need the current-cabling recovery.
"$fixture/run" stage j24back1 lab-igc1 180
review lab-igc1 isp-uplink-only
expect_fail arm j24back1 lab-igc1 180
grep -q 'current-cabling recovery not active' "$fixture/failure.log"
"$fixture/run" stage j24fin03 lab-final 180
review lab-final isp-uplink-only
"$fixture/run" arm j24fin03 lab-final 180
"$fixture/run" activate j24fin03
"$fixture/run" boot_restore
[[ $(cat "$base/router-state/status") == restored-at-boot && ! -f $base/router-state/pending ]]
for f in hostname.igc0 hostname.igc1 hostname.igc2 hostname.igc3 pf.conf dhcpd.conf rad.conf; do cmp -s "$fixture/final.saved/$f" "$fixture/etc/$f"; done
rm -rf "$fixture/final.saved"
rm -rf "$fixture/lab.saved" "$fixture/camera.c24"
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
