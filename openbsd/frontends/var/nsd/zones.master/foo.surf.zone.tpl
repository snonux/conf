$ORIGIN foo.surf.
$TTL 4h
@        IN  SOA  blowfish.buetow.org. hostmaster.buetow.org. (
                  <%= time() %>   ; serial
                  1h              ; refresh
                  30m             ; retry
                  7d              ; expire
                  1h )            ; negative
         IN NS   blowfish.buetow.org.
         IN NS   twofish.buetow.org.

         IN MX 20 buetow.org.
         IN MX 10 www.buetow.org.

	 86400 IN A 108.160.134.135
         86400 IN AAAA 2401:c080:1000:45af:5400:3ff:fec6:ca1d
www      86400 IN CNAME blowfish.buetow.org.
