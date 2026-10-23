#!/bin/sh
# Validate staged certificate material before the approved repair procedure.
# The expected leaf pin must come from a hostname-verified TLS connection to
# the active frontend, not from the unvalidated incoming archive.
set -eu
PATH=/bin:/usr/bin:/sbin:/usr/sbin:/usr/local/bin:/usr/local/sbin
export PATH
umask 077

fail() {
    printf 'Certificate validation failed: %s\n' "$1" >&2
    exit 1
}

if [ "$#" -ne 4 ]; then
    printf 'Usage: %s fullchain key expected-leaf-sha256 ca-file\n' "$0" >&2
    exit 2
fi
chain=$1
key=$2
expected=$3
ca_file=$4

case "$expected" in
    *[!0-9a-f]*|'') fail 'invalid public leaf pin' ;;
esac
[ "${#expected}" -eq 64 ] || fail 'invalid public leaf pin'
[ -f "$chain" ] && [ -f "$key" ] && [ -f "$ca_file" ] ||
    fail 'missing input file'

# These temporary files contain public certificate/public-key data only.
# The private key remains at the supplied path and is never copied or logged.
work=$(mktemp -d "${TMPDIR:-/tmp}/code-cert-check.XXXXXXXXXX") ||
    fail 'cannot create private working directory'
trap 'rm -rf "$work"' 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

openssl x509 -in "$chain" -outform DER >"$work/leaf.der" 2>/dev/null ||
    fail 'cannot decode certificate'
digest=$(openssl dgst -sha256 "$work/leaf.der" 2>/dev/null) ||
    fail 'cannot fingerprint public certificate'
digest=${digest##* }
[ "$digest" = "$expected" ] || fail 'public leaf pin mismatch'

# Do not replace a nearly expired standby certificate with another short-lived
# candidate. Hostname/identity is bound by the independently validated leaf pin.
openssl x509 -in "$chain" -checkend 2592000 -noout >/dev/null 2>&1 ||
    fail 'candidate expires within 30 days'
openssl verify -CAfile "$ca_file" -untrusted "$chain" \
    -purpose sslserver "$chain" >/dev/null 2>&1 ||
    fail 'candidate certificate chain is not trusted'

openssl x509 -in "$chain" -pubkey -noout >"$work/cert.pub" 2>/dev/null ||
    fail 'cannot read certificate public key'
openssl pkey -pubin -in "$work/cert.pub" -outform DER \
    >"$work/cert.der" 2>/dev/null || fail 'invalid certificate public key'
# An empty passphrase prevents an encrypted or malformed input from prompting.
openssl pkey -in "$key" -passin pass: -pubout -outform DER \
    >"$work/key.der" 2>/dev/null || fail 'cannot read candidate public key'
cmp -s "$work/cert.der" "$work/key.der" ||
    fail 'certificate and private key do not match'

printf 'Certificate pin, chain, lifetime and key match: OK\n'
