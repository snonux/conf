$ORIGIN foo.zone.
$TTL 4h
@        IN  SOA  fishfinger.buetow.org. hostmaster.buetow.org. (
                  @SERIAL@   ; serial
                  1h              ; refresh
                  30m             ; retry
                  7d              ; expire
                  1h )            ; negative
         IN NS   fishfinger.buetow.org.
         IN NS   blowfish.buetow.org.

         IN MX 10 fishfinger.buetow.org.
         IN MX 20 blowfish.buetow.org.

        300 IN A @MASTER_IPV4@ ; Enable failover
        300 IN AAAA @MASTER_IPV6@ ; Enable failover
www     300 IN A @MASTER_IPV4@ ; Enable failover
www     300 IN AAAA @MASTER_IPV6@ ; Enable failover
standby  300 IN A @STANDBY_IPV4@ ; Enable failover
standby  300 IN AAAA @STANDBY_IPV6@ ; Enable failover

f3s           300 IN A @MASTER_IPV4@ ; Enable failover
f3s           300 IN AAAA @MASTER_IPV6@ ; Enable failover
www.f3s       300 IN A @MASTER_IPV4@ ; Enable failover
www.f3s       300 IN AAAA @MASTER_IPV6@ ; Enable failover
standby.f3s   300 IN A @STANDBY_IPV4@ ; Enable failover
standby.f3s   300 IN AAAA @STANDBY_IPV6@ ; Enable failover

stats         300 IN A @MASTER_IPV4@ ; Enable failover
stats         300 IN AAAA @MASTER_IPV6@ ; Enable failover
www.stats     300 IN A @MASTER_IPV4@ ; Enable failover
www.stats     300 IN AAAA @MASTER_IPV6@ ; Enable failover
standby.stats 300 IN A @STANDBY_IPV4@ ; Enable failover
standby.stats 300 IN AAAA @STANDBY_IPV6@ ; Enable failover
