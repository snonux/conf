#!/bin/ksh
# Exercise the Gonf ACME renewal script (gonf/frontends/assets/acme.sh.tmpl,
# task m52) without an OpenBSD host or a certificate authority: no real
# certificate is ever requested. The template's certificate list is replaced
# by a fixed one, its /etc and /usr/sbin paths are rewritten into a scratch
# tree, and ifconfig, timeout, host, acme-client and rcctl are faked. The
# fake acme-client acts per host as told by $root/acme.<host>: "renew"
# writes new certificate bytes and exits 0, "same" exits 0 without changing
# them, "valid" exits 2 (still valid), "fail" exits 1. Hosts resolve to the
# address in $root/dns.<host> (no file: lookup fails). Requires a POSIX sh
# for the script and ksh for this harness (OpenBSD's or ksh93).
#
# Covers: an up-to-date run reloads nothing; a renewed site certificate
# reloads relayd only; a renewed host certificate also restarts smtpd; exit 0
# without new bytes is unchanged; a host served by the other frontend is
# skipped, unless its placeholder had to be created; a failed lookup is a
# skip, not an update; a failure still reloads for other changes and makes
# the run exit 1; legacy symlinked standby files are replaced; relayd is
# restarted when it is not running; the certificate list drives the requests.

set -eu

script_dir=$(cd "$(dirname "$0")/../../.." && pwd)
template=$script_dir/gonf/frontends/assets/acme.sh.tmpl
work=$(mktemp -d "${TMPDIR:-/tmp}/acme-test.XXXXXX")
trap 'rm -rf "$work"' EXIT

fake=$work/bin
root=$work/root
log=$work/actions.log
mkdir -p "$fake" "$root/etc/ssl/private"

readonly MY_IP=192.0.2.10 OTHER_IP=192.0.2.20

# The rendered list: one site with its standby twin, a second site, and the
# frontend's own host certificate (the order acmeData produces).
cat >"$work/certificates" <<'EOF'
example.org site
standby.example.org standby
other.example.org site
blowfish.example.org host
EOF

sed \
    -e '/^{{- range .Certificates}}$/,/^{{- end}}$/d' \
    -e "s|/etc/|$root/etc/|g" \
    -e "s|/usr/sbin/|$fake/|g" \
    "$template" >"$work/acme.sh.in"
awk -v list="$work/certificates" '
    { print }
    /^done <<.CERTIFICATES.$/ { while ((getline line < list) > 0) print line }
' "$work/acme.sh.in" >"$work/acme.sh"
if grep -q '{{' "$work/acme.sh"; then
    print "template directives left after substitution" >&2
    exit 1
fi

cat >"$fake/ifconfig" <<EOF
#!/bin/sh
echo "vio0: flags=8843<UP>"
echo "	inet $MY_IP netmask 0xffffff00"
EOF

cat >"$fake/timeout" <<'EOF'
#!/bin/sh
shift
exec "$@"
EOF

cat >"$fake/host" <<EOF
#!/bin/sh
[ -f "$root/dns.\$1" ] || { echo "Host \$1 not found: 3(NXDOMAIN)"; exit 1; }
echo "\$1 has address \$(cat "$root/dns.\$1")"
EOF

cat >"$fake/acme-client" <<EOF
#!/bin/sh
host=\$2
echo "acme-client \$*" >>"$log"
case \$(cat "$root/acme.\$host" 2>/dev/null || echo valid) in
renew)
    echo "renewed \$(date +%s) \$\$" >"$root/etc/ssl/\$host.fullchain.pem"
    exit 0
    ;;
same) exit 0 ;;
fail) exit 1 ;;
*) exit 2 ;;
esac
EOF

cat >"$fake/rcctl" <<EOF
#!/bin/sh
case \$1 in
check) [ ! -f "$root/relayd.stopped" ] ;;
*) echo "rcctl \$*" >>"$log" ;;
esac
EOF
chmod +x "$fake"/*
export PATH="$fake:$PATH"

failures=0

# check name command...: record a failure unless the command succeeds.
check() {
    name=$1
    shift
    if "$@"; then
        print "ok   $name"
    else
        print "FAIL $name"
        failures=$((failures + 1))
    fi
}

# cert name: install certificate files for name (the placeholder source
# foo.example is installed by reset).
cert() {
    print "cert $1" >"$root/etc/ssl/$1.fullchain.pem"
    print "cert $1" >"$root/etc/ssl/$1.crt"
    print "key $1" >"$root/etc/ssl/private/$1.key"
}

# reset: every name resolves to this frontend, is configured in httpd, has
# certificates and is still valid; no actions are logged.
reset() {
    rm -rf "$root"
    mkdir -p "$root/etc/ssl/private"
    : >"$root/etc/httpd.conf"
    for name in foo.zone example.org standby.example.org other.example.org blowfish.example.org; do
        cert "$name"
        print "$MY_IP" >"$root/dns.$name"
        print "server \"$name\" {" >>"$root/etc/httpd.conf"
    done
    : >"$log"
}

run() {
    set +e
    sh "$work/acme.sh" >"$work/out" 2>&1
    status=$?
    set -e
}

logged() { grep -qx "$1" "$log"; }
not_logged() { ! grep -q "$1" "$log"; }
said() { grep -qx "$1" "$work/out"; }

reset
run
check "up to date: exit 0" [ "$status" -eq 0 ]
check "up to date: every certificate requested" \
    [ "$(grep -c '^acme-client' "$log")" -eq 4 ]
check "up to date: requests follow the list" \
    [ "$(awk '/^acme-client/ { print $3 }' "$log" | tr '\n' ' ')" = \
        "example.org standby.example.org other.example.org blowfish.example.org " ]
check "up to date: no reload" not_logged rcctl
check "up to date: reported unchanged" said "Certificate example.org: unchanged"

reset
print renew >"$root/acme.other.example.org"
run
check "site renewed: exit 0" [ "$status" -eq 0 ]
check "site renewed: relayd reloaded" logged "rcctl reload relayd"
check "site renewed: smtpd left alone" not_logged "restart smtpd"
check "site renewed: reported changed" said "Certificate other.example.org: changed"

reset
print renew >"$root/acme.blowfish.example.org"
run
check "host renewed: relayd reloaded" logged "rcctl reload relayd"
check "host renewed: smtpd restarted" logged "rcctl restart smtpd"

reset
print same >"$root/acme.example.org"
run
check "exit 0, same bytes: no reload" not_logged rcctl
check "exit 0, same bytes: unchanged" said "Certificate example.org: unchanged"

reset
print "$OTHER_IP" >"$root/dns.other.example.org"
run
check "served elsewhere: not requested" not_logged "acme-client -v other.example.org"
check "served elsewhere: skipped" said "Certificate other.example.org: skipped"
check "served elsewhere: no reload" not_logged rcctl

reset
print "$OTHER_IP" >"$root/dns.other.example.org"
rm "$root/etc/ssl/other.example.org.crt" "$root/etc/ssl/other.example.org.fullchain.pem" \
    "$root/etc/ssl/private/other.example.org.key"
run
check "placeholder created: copied from foo.zone" \
    cmp -s "$root/etc/ssl/foo.zone.fullchain.pem" "$root/etc/ssl/other.example.org.fullchain.pem"
check "placeholder created: relayd reloaded" logged "rcctl reload relayd"
check "placeholder created: reported changed" said "Certificate other.example.org: changed"

reset
rm "$root/dns.example.org"
run
check "lookup failed: exit 0" [ "$status" -eq 0 ]
check "lookup failed: skipped" said "Certificate example.org: skipped"
check "lookup failed: no reload" not_logged rcctl

reset
sed -i.bak '/"other.example.org"/d' "$root/etc/httpd.conf"
run
check "not in httpd: skipped" said "Certificate other.example.org: skipped"
check "not in httpd: not requested" not_logged "acme-client -v other.example.org"

reset
print fail >"$root/acme.example.org"
print renew >"$root/acme.blowfish.example.org"
run
check "failure: exit 1" [ "$status" -eq 1 ]
check "failure: reported failed" said "Certificate example.org: failed"
check "failure: later certificates still requested" logged "acme-client -v blowfish.example.org"
check "failure: other changes still reload" logged "rcctl restart smtpd"

reset
rm "$root/etc/ssl/standby.example.org.fullchain.pem" "$root/etc/ssl/private/standby.example.org.key"
ln -s "$root/etc/ssl/example.org.fullchain.pem" "$root/etc/ssl/standby.example.org.fullchain.pem"
ln -s "$root/etc/ssl/private/example.org.key" "$root/etc/ssl/private/standby.example.org.key"
run
check "legacy standby: symlink replaced by a file" \
    [ ! -L "$root/etc/ssl/standby.example.org.fullchain.pem" ]
check "legacy standby: relayd reloaded" logged "rcctl reload relayd"

reset
print renew >"$root/acme.example.org"
: >"$root/relayd.stopped"
run
check "relayd stopped: restarted instead" logged "rcctl restart relayd"
check "relayd stopped: not reloaded" not_logged "reload relayd"

if [ "$failures" -ne 0 ]; then
    print "$failures check(s) failed"
    exit 1
fi
print "all checks passed"
