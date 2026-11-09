#!/bin/ksh
# Destructive operations are mocked; all filesystem paths are rewritten into a
# disposable root. Run on OpenBSD: ksh rollback.ksh CONTROLLER PROFILE_DIRECTORY.
set -eu
set -o pipefail
[[ $# == 2 && $(uname -s) == OpenBSD ]] || exit 2
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
   if grep -Eq '^pass in quick on [$]up6_if inet6 proto icmp6 from fe80::/10 .*icmp6-type routeradv$' "$2"; then
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
 [[ $* == '-xf /sbin/resolvd' && -f $BASE/native-resolvd ]] || return 1
 print 12345
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
review() {
 {
  print -- "candidate-sha256=$(cat "$base/router-state/digest")"
  print 'provisioning-clients=captured-inventory-no-known-clients'
  print 'dependencies=r14-reviewed-current-stage-only'
  print 'wg2=provisioned-and-tested'
  print 'old-management=fresh-authenticated'
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
sed 's/^inet6 -autoconf -temporary$/inet6 autoconf -temporary/' "$fixture/wifi-first-igc0.saved" > "$base/router-profiles/wifi-first/hostname.igc0"
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
grep -qx 'inet6 -autoconf -temporary' "$fixture/etc/hostname.igc0"
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
# Bootstrap must not alter physical interfaces, PF, routes or service state.
# It must not alter physical interfaces, PF, routes or service state.
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
print 'rollback fixture: passed (mocked mutations; real native config parsers)'
