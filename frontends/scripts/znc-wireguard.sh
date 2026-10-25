#!/bin/sh
# Mutate private ZNC state on the destination only; never emit its contents.
set -eu
umask 077
operation=${1:?expected check or apply}
address=${2:?expected WireGuard IPv4 address}
case "$address" in
    192.168.2.111) ;;
    *) echo 'unexpected ZNC WireGuard address' >&2; exit 2 ;;
esac
config=/var/znc/configs/znc.conf
transform=/usr/local/libexec/znc-wg-listener.awk

_bound() {
    awk -v address="$address" -v check=1 -f "$transform" "$config" >/dev/null || return 1
    rcctl check znc >/dev/null || return 1
    # The listener must actually have adopted the managed bind.
    sockets=$(netstat -an -f inet) || return 1
    printf '%s\n' "$sockets" | awk -v endpoint="$address.4242" '
        $NF == "LISTEN" && $4 ~ /[.]4242$/ {
            count++; if ($4 != endpoint) bad = 1
        }
        END { exit (count != 1 || bad) }
    ' || return 1
    sockets=$(netstat -an -f inet6) || return 1
    printf '%s\n' "$sockets" | awk '
        $NF == "LISTEN" && $4 ~ /[.]4242$/ { bad = 1 }
        END { exit bad }
    '
}

case "$operation" in
    check) _bound; exit ;;
    apply) ;;
    *) echo 'expected check or apply' >&2; exit 2 ;;
esac
_bound && exit 0
# Refuse an unfamiliar layout before interrupting the daemon.
awk -v address="$address" -f "$transform" "$config" >/dev/null
rcctl check znc >/dev/null
backup=$(mktemp -d /var/znc/configs/.wg-backup.XXXXXXXX)
cp -p "$config" "$backup/znc.conf"
stopped=no
committed=no
_cleanup() {
    status=$?
    trap - EXIT HUP INT TERM
    if [ "$stopped" = yes ] && [ "$committed" = no ]; then
        # A started daemon must stop before restoration or its exit write
        # could overwrite the restored config.
        if rcctl stop znc; then
            cp -p "$backup/znc.conf" "$config"
            rcctl start znc || status=1
            echo 'ZNC bind change failed; restored private backup' >&2
        else
            echo "ZNC rollback stop failed; private backup: $backup" >&2
            status=1
        fi
    fi
    rm -f "$backup/candidate"
    exit "$status"
}
trap _cleanup EXIT
trap 'exit 1' HUP INT TERM
# ZNC saves configuration on exit. Edit only after that write completes.
stopped=yes
rcctl stop znc
cp -p "$config" "$backup/znc.conf"
awk -v address="$address" -f "$transform" "$config" > "$backup/candidate"
chown _znc:_znc "$backup/candidate"
chmod 600 "$backup/candidate"
mv "$backup/candidate" "$config"
rcctl start znc
_bound
committed=yes
# Root-private backup remains on-host for the explicitly approved rollback.
echo "ZNC WireGuard listener active; private backup: $backup"
