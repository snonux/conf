#!/bin/sh
#
# Export pf per-label traffic counters as Prometheus metrics.
#
# Bridges the gap left by Traefik's metrics, which only cover HTTP that
# relayd forwards into the k3s ingress. Gemini, git+ssh, the pi0/pi1 static
# sites and the NodePort relays all bypass Traefik, so it reports nothing for
# them however busy they are. pf sits under all of it and sees everything, and
# the labelled rules at the end of pf.conf attribute bytes per service.
#
# Written for node_exporter's textfile collector: the file must land
# atomically, or node_exporter can scrape a half-written file and emit a
# parse error, so build in a temp file and rename.
#
# `pfctl -sl` columns are:
#   label evaluations total-packets total-bytes packets-in bytes-in \
#         packets-out bytes-out state-creations
set -u

OUTDIR="${OUTDIR:-/var/node_exporter}"
OUT="$OUTDIR/pf_labels.prom"
TMP="$OUT.$$"

[ -d "$OUTDIR" ] || exit 0

{
    echo '# HELP pf_label_bytes_in_total Bytes received, per pf rule label.'
    echo '# TYPE pf_label_bytes_in_total counter'
    pfctl -sl 2>/dev/null | awk 'NF>=8 {printf "pf_label_bytes_in_total{label=\"%s\"} %s\n", $1, $5}'

    echo '# HELP pf_label_bytes_out_total Bytes sent, per pf rule label.'
    echo '# TYPE pf_label_bytes_out_total counter'
    pfctl -sl 2>/dev/null | awk 'NF>=8 {printf "pf_label_bytes_out_total{label=\"%s\"} %s\n", $1, $7}'

    echo '# HELP pf_label_packets_total Packets matched, per pf rule label.'
    echo '# TYPE pf_label_packets_total counter'
    pfctl -sl 2>/dev/null | awk 'NF>=8 {printf "pf_label_packets_total{label=\"%s\"} %s\n", $1, $3}'

    echo '# HELP pf_label_states_total States created, per pf rule label.'
    echo '# TYPE pf_label_states_total counter'
    pfctl -sl 2>/dev/null | awk 'NF>=8 {printf "pf_label_states_total{label=\"%s\"} %s\n", $1, $8}'
} > "$TMP" 2>/dev/null

mv "$TMP" "$OUT"
