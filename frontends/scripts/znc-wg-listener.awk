# Preserve every byte except the single listener's Host directive. Never log
# input: the surrounding ZNC config includes credentials and IRC networks.
/^[[:space:]]*<Listener / { inside = 1; listeners++ }
inside && /^[[:space:]]*Host[[:space:]]*=/ {
    hosts++
    if ($3 != address) different = 1
    print "\tHost = " address
    next
}
inside && /^[[:space:]]*Port[[:space:]]*=/ { ports++; if ($3 != "4242") bad = 1 }
inside && /^[[:space:]]*IPv4[[:space:]]*=/ { v4++; if ($3 != "true") bad = 1 }
inside && /^[[:space:]]*IPv6[[:space:]]*=/ { v6++; if ($3 != "false") bad = 1 }
inside && /^[[:space:]]*SSL[[:space:]]*=/ { tls++; if ($3 != "true") bad = 1 }
/^[[:space:]]*<\/Listener>/ {
    if (!inside) bad = 1
    if (!hosts) print "\tHost = " address
    inside = 0
    ends++
}
{ print }
END {
    if (bad || inside || listeners != 1 || ends != 1 || hosts > 1 ||
        ports != 1 || v4 != 1 || v6 != 1 || tls != 1) exit 2
    if (check && (hosts != 1 || different)) exit 1
}
