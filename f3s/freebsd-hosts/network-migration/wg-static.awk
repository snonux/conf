# Canonical static records from `wg show wg0 dump`; pipe directly to sort/hash.
# Records contain secrets: never print or persist them as diagnostic output.
NR == 1 {
    if (NF != 4) { bad = 1; exit 1 }
    print "interface:private=" $1
    print "interface:public=" $2
    print "interface:listen=" $3
    print "interface:mark=" $4
    next
}
{
    if (NF != 8 || seen[$1]++) { bad = 1; exit 1 }
    print "peer:" $1 ":psk=" $2
    print "peer:" $1 ":endpoint=" $3
    print "peer:" $1 ":keepalive=" $8
    count = split($4, ips, ",")
    for (i = 1; i <= count; i++) print "peer:" $1 ":allowed=" ips[i]
}
END { if (bad || NR < 1) exit 1 }
