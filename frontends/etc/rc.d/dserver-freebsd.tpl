#!/bin/sh
#
# PROVIDE: dserver
# REQUIRE: LOGIN
# KEYWORD: shutdown

. /etc/rc.subr

name="dserver"
rcvar="dserver_enable"
desc="DTail distributed log server"

# Use daemon(8) explicitly: rc.subr's dserver_user wraps with su -m but does
# not fork, so the service command blocks. daemon(8) -u runs as the target
# user and properly detaches from the terminal.
procname="/usr/local/bin/dserver"
command="/usr/sbin/daemon"
command_args="-u dserver -o /var/log/dserver/dserver.log -- /usr/local/bin/dserver -cfg /usr/local/etc/dserver/dtail.json"

start_precmd="dserver_precmd"

dserver_precmd()
{
    # /var/run is volatile on FreeBSD (cleanvar purges it at boot) — recreate
    # the runtime dirs and repopulate the SSH key cache on every service
    # start (a daily periodic job keeps it fresh afterwards). The SSH host
    # key lives in persistent /var/db/dserver so it survives reboots (a
    # regenerated host key would break clients' known_hosts).
    install -d -o dserver -m 0755 /var/log/dserver
    install -d -o dserver -m 0755 /var/run/dserver
    install -d -o dserver -m 0755 /var/run/dserver/cache
    install -d -o dserver -m 0700 /var/db/dserver
    if [ -x /usr/local/bin/dserver-update-key-cache.sh ]; then
        /usr/local/bin/dserver-update-key-cache.sh
    fi
}

load_rc_config $name
run_rc_command "$1"
