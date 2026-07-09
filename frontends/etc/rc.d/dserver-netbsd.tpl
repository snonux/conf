#!/bin/sh
#
# PROVIDE: dserver
# REQUIRE: DAEMON LOGIN
# KEYWORD: shutdown

$_rc_subr_loaded . /etc/rc.subr

name="dserver"
rcvar=$name
command="/usr/local/bin/dserver"

# dserver does not daemonize itself and NetBSD has no daemon(8) like FreeBSD,
# so background it from the shell; rc.subr finds it again via check_process.
# Logger "Fout" writes the real logs to /var/log/dserver, stdout is minimal.
command_args="-cfg /etc/dserver/dtail.json >> /var/log/dserver/dserver.log 2>&1 &"
dserver_user="dserver"

start_precmd="dserver_precmd"

dserver_precmd()
{
	# /var/run is volatile on NetBSD — recreate the runtime dirs and
	# repopulate the SSH key cache on every service start.
	install -d -o dserver -m 0755 /var/log/dserver
	install -d -o dserver -m 0755 /var/run/dserver
	install -d -o dserver -m 0755 /var/run/dserver/cache
	if [ -x /usr/local/bin/dserver-update-key-cache.sh ]; then
		/usr/local/bin/dserver-update-key-cache.sh
	fi
}

load_rc_config $name
run_rc_command "$1"
