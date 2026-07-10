#!/bin/ksh

daemon="/usr/local/bin/dserver"
daemon_flags="-cfg /etc/dserver/dtail.json"
daemon_user="_dserver"

. /etc/rc.d/rc.subr

rc_reload=NO
# dserver does not daemonize itself — let rc.subr start it in the background
# (rc_bg) instead of backgrounding the whole rc_cmd invocation, which would
# hide rc_pre failures and the start result from rcctl.
rc_bg=YES

rc_pre() {
    # /var/run is wiped by /etc/rc at boot — recreate the runtime dirs and
    # repopulate the SSH key cache on every service start (a daily cron job
    # keeps it fresh afterwards). The SSH host key lives in persistent
    # /var/db/dserver so it survives reboots (a regenerated host key would
    # break clients' known_hosts).
    # install -d applies -o only to the final directory, so create the
    # parent /var/run/dserver (the _dserver home dir) explicitly with the
    # right ownership instead of letting it appear implicitly as root:wheel.
    install -d -o _dserver /var/log/dserver
    install -d -o _dserver /var/run/dserver
    install -d -o _dserver /var/run/dserver/cache
    install -d -o _dserver -m 0700 /var/db/dserver
    if [ -x /usr/local/bin/dserver-update-key-cache.sh ]; then
        /usr/local/bin/dserver-update-key-cache.sh
    fi
}

rc_cmd "$1"
