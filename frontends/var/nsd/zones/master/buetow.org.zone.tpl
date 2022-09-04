$ORIGIN buetow.org.
$TTL 4h
@        IN  SOA  blowfish.buetow.org. hostmaster.buetow.org. (
                  <%= time() %>   ; serial
                  1h              ; refresh
                  30m             ; retry
                  7d              ; expire
                  1h )            ; negative
         IN NS    blowfish.buetow.org.
         IN NS    fishfinger.buetow.org.

         IN MX 10 blowfish.buetow.org.
         IN MX 20 fishfinger.buetow.org.
         86400 IN A 23.88.35.144
         86400 IN AAAA 2a01:4f8:c17:20f1::42

*        IN MX 10 blowfish.buetow.org.
*        IN MX 20 fishfinger.buetow.org.
*        86400 IN A 23.88.35.144
*        86400 IN AAAA 2a01:4f8:c17:20f1::42

blowfish 86400 IN A 23.88.35.144
blowfish 86400 IN AAAA 2a01:4f8:c17:20f1::42
git1     3600 IN CNAME blowfish
paul     3600 IN CNAME blowfish
tmp      3600 IN CNAME blowfish
dory     3600 IN CNAME blowfish

fishfinger  86400 IN A 46.23.94.99
fishfinger  86400 IN AAAA 2a03:6000:6f67:624::99
git2      3600 IN CNAME fishfinger
www      3600 IN CNAME fishfinger
www.paul 3600 IN CNAME fishfinger
www.tmp  3600 IN CNAME fishfinger
www.dory 3600 IN CNAME fishfinger

vulcan   86400 IN A 95.216.174.192
vulcan   86400 IN AAAA 2a01:4f9:c010:250e::1
vu       86400 IN CNAME vulcan
wolke7   3600 IN CNAME vulcan
edge     3600 IN CNAME vulcan

sofia    86400 IN CNAME 79-100-3-54.ip.btc-net.bg.
www2     3600 IN CNAME snonux.codeberg.page.
