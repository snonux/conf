#!/bin/ksh

daemon="/usr/local/bin/gorum"
daemon_flags="-cfg /etc/gorum.json"
daemon_user="_gorum"

. /etc/rc.d/rc.subr

rc_reload=NO

rc_pre() {
    install -d -o _gorum /var/log/gorum
    install -d -o _gorum /var/run/gorum/cache
}

rc_cmd $1 &
