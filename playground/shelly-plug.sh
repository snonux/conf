#!/bin/sh
# Control and query a Shelly Plug M Gen 3 via its HTTP RPC API.
#
# Usage:
#   shelly-plug.sh [host] <command>
#
# Commands:
#   status   Show power, voltage, current, energy and temperature
#   on       Turn the plug on
#   off      Turn the plug off
#   toggle   Toggle the plug
#   info     Show device info (model, firmware, mac)
#
# The host defaults to 192.168.1.28 and can be overridden by the first
# argument or the SHELLY_HOST environment variable.
#
# If the device has authentication enabled, the password is read from the
# SHELLY_PASS environment variable, or from ~/.shelly_plug if that is unset
# (the username is always "admin" on Shelly Gen3). Digest auth is used
# automatically when a password is available.

set -eu

DEFAULT_HOST="192.168.1.28"
PASS_FILE="${HOME}/.shelly_plug"

# Argument forms:
#   (none)            -> host=default, command=status
#   <command>         -> host=default, command=<command>
#   <host> <command>  -> explicit host and command
if [ "$#" -ge 2 ]; then
	HOST="$1"
	CMD="$2"
elif [ "$#" -eq 1 ]; then
	HOST="${SHELLY_HOST:-$DEFAULT_HOST}"
	CMD="$1"
else
	HOST="${SHELLY_HOST:-$DEFAULT_HOST}"
	CMD="status"
fi

BASE="http://${HOST}"

# Resolve the password: SHELLY_PASS wins, otherwise read it from PASS_FILE.
PASS="${SHELLY_PASS:-}"
if [ -z "$PASS" ] && [ -r "$PASS_FILE" ]; then
	PASS="$(head -n 1 "$PASS_FILE" | tr -d '\r\n')"
fi

# Build optional digest-auth args when a password is available.
AUTH=""
if [ -n "$PASS" ]; then
	AUTH="--digest -u admin:${PASS}"
fi

rpc() {
	# shellcheck disable=SC2086
	curl -s --max-time 5 $AUTH "${BASE}/rpc/$1"
}

case "$CMD" in
	status)
		rpc "Switch.GetStatus?id=0"
		echo
		;;
	on)
		rpc "Switch.Set?id=0&on=true"
		echo
		;;
	off)
		rpc "Switch.Set?id=0&on=false"
		echo
		;;
	toggle)
		rpc "Switch.Toggle?id=0"
		echo
		;;
	info)
		curl -s --max-time 5 "${BASE}/shelly"
		echo
		;;
	*)
		echo "Unknown command: $CMD" >&2
		echo "Use one of: status, on, off, toggle, info" >&2
		exit 1
		;;
esac
