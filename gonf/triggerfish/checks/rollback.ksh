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
sed '/^# Dispatch:/,$d' "$1" | sed "s@/var/unbound/etc/@__UNBOUND__@g; s@/etc/@$fixture/etc/@g; s@/var/@$fixture/var/@g; s@__UNBOUND__@$fixture/var/unbound/etc/@g" > "$fixture/functions"
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
 '-sr') print 'pass all';;
 '-f '*)
  print -- "pfctl $*" >> "$BASE/operations"
  if [[ $2 == "$STATE/candidate/pf.conf" ]]; then
   touch "$BASE/pf-blocks-ra"
   mkdir -p "$BASE/pf-tables"
   touch "$BASE/pf-tables/recorders" "$BASE/pf-tables/camera_updates"
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
 if [[ $* == '-n show -inet' ]]; then print 'default 192.168.1.1 UGS 0 0 - 8 igc0'; return; fi
 if [[ $* == '-n show -inet6' ]]; then return; fi
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
crontab() { print -- "* * * * * $SELF tick"; }
pgrep() {
 [[ $* == '-xf /sbin/resolvd' && -f $BASE/native-resolvd ]] || return 1
 print 12345
}
netstat() { cat "$BASE/sockets"; }
sh() {
 [[ $1 == "${BASE%/gonf}/netstart" && $2 == wg2 ]] || return 1
 print ' inet 10.99.0.1 netmask 0xffffff00' >> "$BASE/interfaces/wg2"
}
sync() { :; }
slaacctl() {
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
# wg2 bootstrap shares only the durable journal and hook with physical trials.
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
