#!/bin/ksh

ZONES_DIR=/var/nsd/zones/master/
DEFAULT_MASTER=fishfinger.buetow.org
DEFAULT_STANDBY=blowfish.buetow.org

determine_master_and_standby () {
    local master=$DEFAULT_MASTER
    local standby=$DEFAULT_STANDBY

    # Based on the week of the year, we swap the master/standby roles.
    # This is so that we always have up-to-date Let's Encrypt TLS certificates
    # renewed on either server.
    local -i week_of_the_year=$(date +%U)
    if [ $(( week_of_the_year % 2 )) -ne 0 ]; then
        local tmp=$master
        master=$standby
        standby=$tmp
    fi

    echo "Master is $master, standby is $standby"

    MASTER_A=$(host $master | awk '/has address/ { print $(NF) }')
    MASTER_AAAA=$(host $master | awk '/has IPv6 address/ { print $(NF) }')
    STANDBY_A=$(host $standby | awk '/has address/ { print $(NF) }')
    STANDBY_AAAA=$(host $standby | awk '/has IPv6 address/ { print $(NF) }')
}

transform () {
    sed -E '
        /IN A .*; Enable failover/ {
            /^mirror/! {
                s/^(.*) 300 IN A (.*) ; (.*)/\1 300 IN A '$MASTER_A' ; \3/;
            }
            /^mirror/ {
                s/^(.*) 300 IN A (.*) ; (.*)/\1 300 IN A '$STANDBY_A' ; \3/;
            }
        }
        /IN AAAA .*; Enable failover/ {
            /^mirror/! {
                s/^(.*) 300 IN AAAA (.*) ; (.*)/\1 300 IN AAAA '$MASTER_AAAA' ; \3/;
            }
            /^mirror/ {
                s/^(.*) 300 IN AAAA (.*) ; (.*)/\1 300 IN AAAA '$STANDBY_AAAA' ; \3/;
            }
        }
        / ; serial/ {
            s/^( +) ([0-9]+) .*; (.*)/\1 '"$(date +%s)"' ; \3/;
        }
    '
}

zone_is_ok () {
    local zone=$1
    local domain=${zone%.zone}

    echo "Testing zone $zone (if no NS output, then doesn't work)"
    dig $domain @localhost | grep "$domain.*IN.*NS"
}

failover_zone () {
    local zone_file=$1
    local zone=$(basename $zone_file)

    cat $zone_file | transform > $zone_file.new.tmp 

    grep -v ' ; serial' $zone_file.new.tmp > $zone_file.new.noserial.tmp
    grep -v ' ; serial' $zone_file > $zone_file.old.noserial.tmp

    if diff $zone_file.new.noserial.tmp $zone_file.old.noserial.tmp; then
        echo "zone $zone_file hasn't changed"
        rm $zone_file.*.tmp
        return
    fi

    cp $zone_file $zone_file.bak
    mv $zone_file.new.tmp $zone_file
    rm $zone_file.*.tmp
    nsd-control reload

    if zone_is_ok $zone; then
        if [ -f $zone_file.invalid ]; then
            rm $zone_file.invalid
        fi
        echo "Failover of zone $zone completed"
        return
    fi

    echo "Rolling back $zone_file changes"
    cp $zone_file $zone_file.invalid
    mv $zone_file.bak $zone_file
    nsd-control reload
    zone_is_ok $zone
}

main () {
    determine_master_and_standby
    for zone_file in $ZONES_DIR/*.zone; do
        failover_zone $zone_file
    done
}

main
