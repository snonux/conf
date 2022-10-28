#!/bin/ksh

daemon="/usr/local/bin/dserver"
daemon_flags="-cfg /etc/dserver/dtail.json"
daemon_user="_dserver"

. /etc/rc.d/rc.subr

rc_reload=NO

rc_pre() {
    install -d -o _dserver /var/log/dserver
    install -d -o _dserver /var/run/dserver/cache
}

rc_cmd $1 &
