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

# Counters must be SUMMED per label, not printed per rule. pf expands an
# address list -- `to { a b c }` -- into one rule per address, all carrying
# the same label, so a rule covering the three r-nodes emits three lines.
# Printing them raw produces duplicate series with identical label sets,
# which the textfile collector rejects. Summing is also the semantically
# right answer: "bytes to Traefik" is the total across all three nodes.
emit() {
    metric="$1"
    col="$2"
    help="$3"
    echo "# HELP $metric $help"
    echo "# TYPE $metric counter"
    pfctl -sl 2>/dev/null | awk -v m="$metric" -v c="$col" '
        NF>=8 { sum[$1] += $c }
        END   { for (l in sum) printf "%s{label=\"%s\"} %d\n", m, l, sum[l] }'
}

# Column numbers are load-bearing and easy to get wrong -- the packet and
# byte pairs interleave, so an off-by-one silently reports packet counts as
# bytes. Verified against live output; $6 + $8 == $4 is the check that the
# in/out byte columns are the right ones.
{
    emit pf_label_bytes_in_total  6 'Bytes received, per pf rule label.'
    emit pf_label_bytes_out_total 8 'Bytes sent, per pf rule label.'
    emit pf_label_packets_total   3 'Packets matched, per pf rule label.'
    emit pf_label_states_total    9 'States created, per pf rule label.'
} > "$TMP" 2>/dev/null

mv "$TMP" "$OUT"
