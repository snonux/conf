#!/bin/sh
set -e
HOST="<%= $goprecords_host %>"
BASE_URL="https://goprecords.f3s.buetow.org"
TOKEN="<%= $goprecords_token %>"
PATH="/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:${PATH}"

upload() {
	kind=$1
	file=$2
	if [ ! -f "$file" ]; then
		echo "goprecords-upload: skip $kind (no $file)" >&2
		return 0
	fi
	curl -fsS -X PUT --data-binary "@${file}" \
		-H "Authorization: Bearer ${TOKEN}" \
		"${BASE_URL}/upload/${HOST}/${kind}"
}

records_path=/var/db/uptimed/records
if [ -f /var/spool/uptimed/records ]; then
	records_path=/var/spool/uptimed/records
fi

tmp=$(mktemp)
trap 'rm -f "$tmp"' 0 INT TERM HUP

upload records "$records_path"

if command -v uprecords >/dev/null 2>&1; then
	uprecords -a -m 100 >"$tmp"
	upload txt "$tmp"
	uprecords -a | grep '^->' >"$tmp" || true
	if [ -s "$tmp" ]; then
		upload cur.txt "$tmp"
	fi
fi

if [ -r /etc/os-release ]; then
	upload os.txt /etc/os-release
else
	uname -a >"$tmp"
	upload os.txt "$tmp"
fi

if [ -r /var/run/dmesg.boot ]; then
	upload cpuinfo.txt /var/run/dmesg.boot
else
	sysctl hw.model hw.ncpu hw.machine >"$tmp" 2>/dev/null || uname -a >"$tmp"
	upload cpuinfo.txt "$tmp"
fi
