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
    install -d -o dserver -m 0755 /var/log/dserver
    install -d -o dserver -m 0755 /var/run/dserver
    install -d -o dserver -m 0755 /var/run/dserver/cache
}

load_rc_config $name
run_rc_command "$1"
