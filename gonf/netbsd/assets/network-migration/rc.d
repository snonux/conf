#!/bin/sh
# PROVIDE: gonf_network_migration
# REQUIRE: network
# BEFORE: npf NETWORKING
# rc.subr consumes the callback variables below.
# shellcheck disable=SC2034

# shellcheck source=/dev/null
. /etc/rc.subr
name="gonf_network_migration"
rcvar=$name
start_cmd="/usr/local/sbin/gonf-network-migration boot"
stop_cmd=":"
load_rc_config "$name"
run_rc_command "$1"
