$ORIGIN irregular.ninja.
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

         1800 IN A <%= $ips->{current_master}{ipv4} %>
         1800 IN AAAA <%= $ips->{current_master}{ipv6} %>
*        1800 IN CNAME <%= $ips->{current_master}{fqdn} %>.
mirror   1800 IN CNAME <%= $ips->{current_standby}{fqdn} %>.
