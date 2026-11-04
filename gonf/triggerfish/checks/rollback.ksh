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
 '-nvf '*) print 'set skip on { lo }'; print 'pass all';;
 '-s Anchors'|'-s Tables') :;;
 '-s info') print 'Status: Enabled';;
 '-s Interfaces -v') print 'lo (skip)'; print 'lo0 (skip)';;
 '-sr') print 'pass all';;
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
 up|down) return 0;;
 inet|inet6)
  [[ $* == -autoconf || $* == autoconf ]] && return 0
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
 'check '*) return 1;;
 'start unbound ntpd') [[ ! -f $BASE/fail-activate ]];;
 *) return 0;;
 esac
}
crontab() { print -- "* * * * * $SELF tick"; }
netstat() { cat "$BASE/sockets"; }
sync() { :; }
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
print 'pkg_scripts=uptimed' > "$fixture/etc/rc.conf.local"
print 'daemon_flags=' > "$fixture/var/run/rc.d/ntpd"
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
"$fixture/run" stage trial0001 shared-igc0 180
# Produce exact reviewed candidate binding as a controller would, then verify
# both missing readiness and missing keys block arm before any network writes.
expect_fail() {
 if "$fixture/run" "$@" > "$fixture/failure.log" 2>&1; then
  print -u2 -- "unexpected success: $*"; exit 1
 fi
}
expect_fail arm trial0001 shared-igc0 180
# The fixture root is mode 700 and the review is owned by the invoking root.
{
 print -- "candidate-sha256=$(cat "$base/router-state/digest")"
 print 'provisioning-clients=reviewed-none-unlisted'
 print 'dependencies=r14-reviewed-none-unlisted'
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
"$fixture/run" restore
[[ -f $base/router-owner && ! -f $base/router-state/pending ]]
rm "$base/fail-activate"
"$fixture/run" stage trial0002 shared-igc0 180
"$fixture/run" arm trial0002 shared-igc0 180
"$fixture/run" activate trial0002
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
 dhcpd_flags=NO; unbound_flags=NO; ntpd_flags=NO; rad_flags=NO; pf=YES
 [[ ! -f "$fixture/etc/rc.conf.local" ]] || . "$fixture/etc/rc.conf.local"
}
unbound_flags=active-cached-value
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
print '[[ $unbound_flags == NO && $pf == YES ]]' >> "$fixture/boot-cache"
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
 print 'provisioning-clients=reviewed-none-unlisted'
 print 'dependencies=r14-reviewed-none-unlisted'
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
print 'rollback fixture: passed (mocked mutations; real native config parsers)'
