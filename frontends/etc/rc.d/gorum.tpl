#!/bin/ksh

daemon="/usr/local/bin/gorum"
daemon_flags="-cfg /etc/gorum.json"
daemon_user="_gorum"
daemon_logger="daemon.info"

. /etc/rc.d/rc.subr

rc_reload=NO

rc_pre() {
    install -d -o _gorum /var/log/gorum
}

rc_cmd $1 &
