#!/bin/sh
# Validate only the four peer certificates in approved private staging.
# Never run on production private keys during preparation.
set -eu
PATH=/bin:/usr/bin:/sbin:/usr/sbin:/usr/local/bin:/usr/local/sbin
export PATH
umask 077

fail() {
    printf 'Peer certificate validation failed: %s\n' "$1" >&2
    exit 1
}

[ "$#" -eq 5 ] || {
    printf 'Usage: %s name fullchain key expected-leaf-sha256 ca-file\n' "$0" >&2
    exit 2
}
name=$1
chain=$2
key=$3
expected=$4
ca_file=$5
case "$name" in
    gogios.buetow.org) wanted='DNS:gogios.buetow.org,DNS:www.gogios.buetow.org' ;;
    standby.ecat.buetow.org|standby.alt.irregular.ninja|standby.gogios.buetow.org)
        wanted=DNS:$name ;;
    *) fail 'unexpected certificate name' ;;
esac
case "$expected" in
    *[!0-9a-f]*|'') fail 'invalid public leaf pin' ;;
esac
[ "${#expected}" -eq 64 ] || fail 'invalid public leaf pin'
for file in "$chain" "$key"; do
    [ -f "$file" ] && [ ! -L "$file" ] ||
        fail 'missing or symlinked staged file'
done
[ -f "$ca_file" ] || fail 'missing CA file'

# The expected pin comes from an independent strict hostname/CA-verified
# public TLS connection to the DNS owner's numeric address, before transfer.
work=$(mktemp -d "${TMPDIR:-/tmp}/peer-cert-check.XXXXXXXXXX") ||
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
openssl x509 -in "$chain" -text -noout >"$work/leaf.txt" 2>/dev/null ||
    fail 'cannot inspect certificate names'
sans=$(awk '/X509v3 Subject Alternative Name:/ { getline; gsub(/[[:space:]]/, ""); print }' "$work/leaf.txt")
[ "$sans" = "$wanted" ] || fail 'unexpected certificate SANs'
openssl x509 -in "$chain" -checkend 2592000 -noout >/dev/null 2>&1 ||
    fail 'candidate expires within 30 days'
openssl verify -CAfile "$ca_file" -untrusted "$chain" \
    -purpose sslserver "$chain" >/dev/null 2>&1 ||
    fail 'candidate certificate chain is not trusted'

# Only public-key derivatives are written to the temporary directory. An
# empty passphrase prevents malformed/encrypted keys from prompting.
openssl x509 -in "$chain" -pubkey -noout >"$work/cert.pub" 2>/dev/null ||
    fail 'cannot read certificate public key'
openssl pkey -pubin -in "$work/cert.pub" -outform DER \
    >"$work/cert.der" 2>/dev/null || fail 'invalid certificate public key'
openssl pkey -in "$key" -passin pass: -pubout -outform DER \
    >"$work/key.der" 2>/dev/null || fail 'cannot read candidate public key'
cmp -s "$work/cert.der" "$work/key.der" ||
    fail 'certificate and private key do not match'
printf 'Peer certificate pin, exact SANs, trust, lifetime and key match: OK\n'
