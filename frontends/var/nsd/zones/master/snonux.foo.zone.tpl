$ORIGIN snonux.foo.
$TTL 4h
@        IN  SOA  blowfish.buetow.org. hostmaster.buetow.org. (
                  <%= time() %>   ; serial
                  1h              ; refresh
                  30m             ; retry
                  7d              ; expire
                  1h )            ; negative
         IN NS   fishfinger.buetow.org.
         IN NS   blowfish.buetow.org.

         IN MX 10 fishfinger.buetow.org.
         IN MX 20 blowfish.buetow.org.

        300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
        300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
www     300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
www     300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
mirror  300 IN A <%= $ips->{current_standby}{ipv4} %> ; Enable failover
mirror  300 IN AAAA <%= $ips->{current_standby}{ipv6} %> ; Enable failover
