#!/bin/ksh

# Every outbound lookup here is wrapped in timeout(1). Without it a single
# hung DNS query wedges the whole script, and because the cron entry uses -s
# (single instance) no later run can start either -- which is exactly what
# happened: a 'host' call blocked in kqread on 2026-08-25 and froze this
# host's zone for a week, leaving the two nameservers advertising different
# masters and breaking Let's Encrypt validation fleet-wide.
readonly LOOKUP_TIMEOUT=10
# Brief HTTPS blips (especially IPv6 to fishfinger) used to flip DNS on a
# single failed ftp(1). Require this many consecutive failures before the
# master is considered down.
readonly HEALTH_TRIES=3

ZONES_DIR=/var/nsd/zones/master/
DEFAULT_MASTER=fishfinger.buetow.org
DEFAULT_STANDBY=blowfish.buetow.org

# Resolve one address and store it, but only if the lookup actually returned
# something. Writing the file unconditionally is what produced the 0-byte
# /var/nsd/run/standby_a seen on blowfish: a failed lookup emptied the file,
# and the sed below would then substitute an empty address into the zone.
# Keeping the previous value is always safer than publishing a broken record.
lookup_into () {
    local -r target_file=$1
    local -r hostname=$2
    local -r pattern=$3

    local -r result=$(timeout $LOOKUP_TIMEOUT host "$hostname" | awk "$pattern { print \$(NF) }")

    if [ -z "$result" ]; then
        echo "Lookup of $hostname ($pattern) failed or timed out, keeping $target_file"
        return 1
    fi

    echo "$result" >"$target_file"
}

# One IP-family HTTPS probe. proto is 4 or 6 (passed to ftp -4/-6).
health_check_once () {
    local -r proto=$1
    local -r master=$2

    timeout $LOOKUP_TIMEOUT ftp -$proto -o - "https://$master/index.txt" \
        | grep -q "Welcome to $master"
}

# Retry a single family so a one-shot TLS/EOF blip does not trigger failover.
health_check_family () {
    local -r proto=$1
    local -r master=$2
    local -i attempt=1

    while [ $attempt -le $HEALTH_TRIES ]; do
        if health_check_once "$proto" "$master"; then
            return 0
        fi
        echo "https://$master/index.txt IPv$proto health check failed (try $attempt/$HEALTH_TRIES)"
        attempt=$((attempt + 1))
    done

    return 1
}

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
    if ! health_check_family 4 "$master"; then
        health_ok=0
    elif ! health_check_family 6 "$master"; then
        health_ok=0
    fi

    if [ $health_ok -eq 0 ]; then
        local tmp=$master
        master=$standby
        standby=$tmp
    fi

    echo "Master is $master, standby is $standby"

    lookup_into /var/nsd/run/master_a       "$master"  '/has address/'
    lookup_into /var/nsd/run/master_aaaa    "$master"  '/has IPv6 address/'
    lookup_into /var/nsd/run/standby_a      "$standby" '/has address/'
    lookup_into /var/nsd/run/standby_aaaa   "$standby" '/has IPv6 address/'
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
