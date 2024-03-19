#!/bin/sh

ZONES_DIR=/var/nsd/zones/master/
MASTER_A=master_a
MASTER_AAAA=master_aaaa
STANDBY_A=standby_a
STANDBY_AAAA=standby_aaaa

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
            s/^( +) ([0-9]+) .*; (.*)/\1 '"`date +%s`"' ; \3/;
        }
    '
}

failover_zone () {
    zone=$1
    cat $zone | transform > $zone.new.tmp 

    grep -v ' ; serial' $zone.new > $zone.new.noserial.tmp
    grep -v ' ; serial' $zone > $zone.old.noserial.tmp

    diff $zone.new.noserial.tmp $zone.old.noserial.tmp
    if [ $? -eq 0 ]; then
        echo "zone $zone hasn't changed"
        rm $zone.*.tmp
        return
    fi

    cp $zone $zone.bak
    mv $zone.new.tmp $zone
    rm $zone.*.tmp
    nsd-control reload $zone

    dig $zone @localhost
    # Todo: Use different return check, als ec may be 0 anyway
    if [ $? -eq 0 ]; then
        return
    fi

    echo "Rolling back $zone changes"
    cp $zone $zone.invalid
    mv $zone.bak $zone
    nsd-control reload $zone
}

for zone in $ZONES_DIR/snonux.foo.zone; do
    failover_zone $zone
done
