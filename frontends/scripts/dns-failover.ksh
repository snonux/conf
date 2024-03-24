#!/bin/ksh

ZONES_DIR=/var/nsd/zones/master/
DEFAULT_MASTER=fishfinger.buetow.org
DEFAULT_STANDBY=blowfish.buetow.org

determine_master_and_standby () {
    local master=$DEFAULT_MASTER
    local standby=$DEFAULT_STANDBY

    # Weekly auto-failover for Let's Encrypt automation
    local -i -r week_of_the_year=$(date +%U)
    if [ $(( week_of_the_year % 2 )) -ne 0 ]; then
        local tmp=$master
        master=$standby
        standby=$tmp
    fi

    local -i health_ok=1
    if ! ftp -4 -o - https://$master/index.txt | grep -q "Welcome to $master"; then
        echo "https://$master/index.txt IPv4 health check failed"
        health_ok=0
    elif ! ftp -6 -o - https://$master/index.txt | grep -q "Welcome to $master"; then
        echo "https://$master/index.txt IPv6 health check failed"
        health_ok=0
    fi

    if [ $health_ok -eq 0 ]; then
        local tmp=$master
        master=$standby
        standby=$tmp
    fi

    echo "Master is $master, standby is $standby"

    host $master | awk '/has address/ { print $(NF) }' >/var/nsd/run/master_a
    host $master | awk '/has IPv6 address/ { print $(NF) }' >/var/nsd/run/master_aaaa
    host $standby | awk '/has address/ { print $(NF) }' >/var/nsd/run/standby_a
    host $standby | awk '/has IPv6 address/ { print $(NF) }' >/var/nsd/run/standby_aaaa
}

transform () {
    sed -E '
        /IN A .*; Enable failover/ {
            /^standby/! {
                s/^(.*) 300 IN A (.*) ; (.*)/\1 300 IN A '$(cat /var/nsd/run/master_a)' ; \3/;
            }
            /^standby/ {
                s/^(.*) 300 IN A (.*) ; (.*)/\1 300 IN A '$(cat /var/nsd/run/standby_a)' ; \3/;
            }
        }
        /IN AAAA .*; Enable failover/ {
            /^standby/! {
                s/^(.*) 300 IN AAAA (.*) ; (.*)/\1 300 IN AAAA '$(cat /var/nsd/run/master_aaaa)' ; \3/;
            }
            /^standby/ {
                s/^(.*) 300 IN AAAA (.*) ; (.*)/\1 300 IN AAAA '$(cat /var/nsd/run/standby_aaaa)' ; \3/;
            }
        }
        / ; serial/ {
            s/^( +) ([0-9]+) .*; (.*)/\1 '$(date +%s)' ; \3/;
        }
    '
}

zone_is_ok () {
    local -r zone=$1
    local -r domain=${zone%.zone}
    dig $domain @localhost | grep -q "$domain.*IN.*NS"
}

failover_zone () {
    local -r zone_file=$1
    local -r zone=$(basename $zone_file)

    # Race condition (e.g. script execution abored in the middle previous run)
    if [ -f $zone_file.bak ]; then
        mv $zone_file.bak $zone_file
    fi

    cat $zone_file | transform > $zone_file.new.tmp 

    grep -v ' ; serial' $zone_file.new.tmp > $zone_file.new.noserial.tmp
    grep -v ' ; serial' $zone_file > $zone_file.old.noserial.tmp

    echo "Has zone $zone_file changed?"
    if diff -u $zone_file.old.noserial.tmp $zone_file.new.noserial.tmp; then
        echo "The zone $zone_file hasn't changed"
        rm $zone_file.*.tmp
        return 0
    fi

    cp $zone_file $zone_file.bak
    mv $zone_file.new.tmp $zone_file
    rm $zone_file.*.tmp
    echo "Reloading nsd"
    nsd-control reload

    if ! zone_is_ok $zone; then
        echo "Rolling back $zone_file changes"
        cp $zone_file $zone_file.invalid
        mv $zone_file.bak $zone_file
        echo "Reloading nsd"
        nsd-control reload
        zone_is_ok $zone
        return 3
    fi

    for cleanup in invalid bak; do
        if [ -f $zone_file.$cleanup ]; then
            rm $zone_file.$cleanup
        fi
    done

    echo "Failover of zone $zone to $MASTER completed"
    return 1
}

main () {
    determine_master_and_standby

    local -i ec=0
    for zone_file in $ZONES_DIR/*.zone; do
        if ! failover_zone $zone_file; then
            ec=1
        fi
    done

    # ec other than 0: CRON will send out an E-Mail.
    exit $ec
}

main
