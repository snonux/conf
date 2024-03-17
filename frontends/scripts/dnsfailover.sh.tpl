#!/bin/sh

ZONES_DIR=/var/nsd/zones/master/
MASTER_A=master_a
MASTER_AAAA=master_aaaa
STANDBY_A=standby_a
STANDBY_AAAA=standby_aaaa

process_zone () {
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
            s/^( +) ([0-9]+) .*; (.*)/\1 '`date +%s`' ; \3/;
        }
    '
}

failover_zone () {
    zone=$1
    cat $zone | process_zone
}

for zone in $ZONES_DIR/snonux.foo.zone; do
    failover_zone $zone
done
