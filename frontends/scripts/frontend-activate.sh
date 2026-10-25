#!/bin/sh
# OpenBSD base sh: no dependency on a separately installed Bash.
# A failed activation must remain retryable after the managed file converges.
set -eu
umask 077

operation=${1:?expected check or apply}
service=${2:?expected pf, httpd or inetd}
case "$service" in
    pf) set -- /etc/pf.conf "$0" ;;
    httpd) set -- /etc/httpd.conf "$0" ;;
    inetd) set -- /etc/inetd.conf /etc/login.conf.d/inetd "$0" ;;
    *) echo 'unsupported frontend service' >&2; exit 2 ;;
esac

# Read every input before hashing; do not hide a failed read in a pipeline.
inputs=$(sha256 -q "$@")
flags=
if [ "$service" != pf ]; then
    flags=$(rcctl get "$service" flags)
fi
digest=$(printf '%s\n%s\n' "$inputs" "$flags" | sha256 -q)
stamp=/var/db/gonf-frontend-$service.sha256

_pf_enabled() {
    status=$(pfctl -si) || return 1
    printf '%s\n' "$status" | grep -q '^Status: Enabled'
}

case "$operation" in
    check)
        [ -f "$stamp" ] && [ "$digest" = "$(cat "$stamp")" ] || exit 1
        if [ "$service" = pf ]; then
            _pf_enabled
        else
            rcctl check "$service" >/dev/null
        fi
        ;;
    apply)
        case "$service" in
            pf)
                pfctl -nf /etc/pf.conf
                # No state flush: established SSH and production sessions stay.
                pfctl -f /etc/pf.conf
                if ! _pf_enabled; then
                    pfctl -e
                fi
                ;;
            httpd)
                httpd -n -f /etc/httpd.conf
                rcctl restart httpd
                rcctl check httpd
                ;;
            inetd)
                # inetd has no parse-only mode; -d would start another daemon.
                rcctl restart inetd
                rcctl check inetd
                ;;
        esac
        # Save success only after the parser, loader and service checks pass.
        printf '%s\n' "$digest" > "$stamp"
        ;;
    *) echo 'expected check or apply' >&2; exit 2 ;;
esac
