$ORIGIN foo.zone.
$TTL 4h
@        IN  SOA  blowfish.buetow.org. hostmaster.buetow.org. (
                  <%= time() %>   ; serial
                  1h              ; refresh
                  30m             ; retry
                  7d              ; expire
                  1h )            ; negative
         IN NS   blowfish.buetow.org.
         IN NS   fishfinger.buetow.org.

         IN MX 10 blowfish.buetow.org.
         IN MX 20 fishfinger.buetow.org.

         1800 IN A 23.88.35.144
         1800 IN AAAA 2a01:4f8:c17:20f1::42
*        1800 IN CNAME blowfish.buetow.org.
www      1800 IN CNAME fishfinger.buetow.org.
www.tmp  1800 IN CNAME fishfinger.buetow.org.
codeberg 1800 IN CNAME snonux.codeberg.page.
