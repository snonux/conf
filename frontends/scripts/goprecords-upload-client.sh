#!/bin/sh
set -e
GOPRECORDS_BASE_URL="${GOPRECORDS_BASE_URL:-https://goprecords.f3s.buetow.org}"
GOPRECORDS_HOST="${GOPRECORDS_HOST:?set GOPRECORDS_HOST (e.g. f0, pi0, earth)}"
PATH="/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:${PATH}"

_default_token_file() {
	if [ "$(id -u)" = "0" ]; then
		printf '/etc/goprecords-upload.token'
	else
		config="${XDG_CONFIG_HOME:-${HOME}/.config}"
		printf '%s/goprecords-upload-%s/token' "$config" "$GOPRECORDS_HOST"
	fi
}

GOPRECORDS_TOKEN_FILE="${GOPRECORDS_TOKEN_FILE:-$(_default_token_file)}"

if ! test -r "$GOPRECORDS_TOKEN_FILE"; then
	echo "goprecords-upload-client: cannot read $GOPRECORDS_TOKEN_FILE" >&2
	exit 1
fi
TOKEN=$(tr -d '\n\r' <"$GOPRECORDS_TOKEN_FILE")

upload() {
	kind=$1
	file=$2
	if ! test -f "$file"; then
		echo "goprecords-upload-client: skip $kind (no $file)" >&2
		return 0
	fi
	curl -fsS -X PUT --data-binary "@${file}" \
		-H "Authorization: Bearer ${TOKEN}" \
		"${GOPRECORDS_BASE_URL}/upload/${GOPRECORDS_HOST}/${kind}"
}

_find_records() {
	for p in \
		/var/spool/uptimed/records \
		/var/db/uptimed/records \
		/usr/local/var/uptimed/records; do
		if test -f "$p"; then
			printf '%s' "$p"
			return 0
		fi
	done
	echo "goprecords-upload-client: no uptimed records file found" >&2
	exit 1
}

records_path=$(_find_records)

tmp=$(mktemp)
trap 'rm -f "$tmp"' 0 INT TERM HUP

upload records "$records_path"

if command -v uprecords >/dev/null 2>&1; then
	uprecords -a -m 100 >"$tmp"
	upload txt "$tmp"
	uprecords -a | grep '^->' >"$tmp" || true
	if test -s "$tmp"; then
		upload cur.txt "$tmp"
	fi
fi

if test -r /etc/os-release; then
	upload os.txt /etc/os-release
elif test -r /var/run/dmesg.boot; then
	upload os.txt /var/run/dmesg.boot
else
	uname -a >"$tmp"
	upload os.txt "$tmp"
fi

if test -r /proc/cpuinfo; then
	upload cpuinfo.txt /proc/cpuinfo
elif test -r /var/run/dmesg.boot; then
	upload cpuinfo.txt /var/run/dmesg.boot
else
	sysctl hw.model hw.ncpu hw.machine >"$tmp" 2>/dev/null || uname -a >"$tmp"
	upload cpuinfo.txt "$tmp"
fi
