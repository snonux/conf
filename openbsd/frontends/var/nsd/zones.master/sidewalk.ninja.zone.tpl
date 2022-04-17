$ORIGIN sidewalk.ninja.
$TTL 4h
@        IN  SOA  blowfish.buetow.org. hostmaster.buetow.org. (
                  <%= time() %>   ; serial
                  1h              ; refresh
                  30m             ; retry
                  7d              ; expire
                  1h )            ; negative
         IN NS   blowfish.buetow.org.
         IN NS   twofish.buetow.org.

         IN MX 10 buetow.org.
         IN MX 20 www.buetow.org.

         86400 IN A 23.88.35.144
         86400 IN AAAA 2a01:4f8:c17:20f1::42
*        86400 IN CNAME blowfish.buetow.org.
www      86400 IN CNAME twofish.buetow.org.
