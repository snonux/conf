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
         1800 IN A 23.88.35.144
         1800 IN AAAA 2a01:4f8:c17:20f1::42

*        IN MX 10 blowfish.buetow.org.
*        IN MX 20 fishfinger.buetow.org.
*        1800 IN A 23.88.35.144
*        1800 IN AAAA 2a01:4f8:c17:20f1::42

blowfish 86400 IN A 23.88.35.144
blowfish 86400 IN AAAA 2a01:4f8:c17:20f1::42
git1     1800 IN CNAME blowfish
tmp      1800 IN CNAME blowfish
dory     1800 IN CNAME blowfish
footos   1800 IN CNAME blowfish
fotos    1800 IN CNAME blowfish
paul 1800 IN A 23.88.35.144
paul 1800 IN AAAA 2a01:4f8:c17:20f1::42
paul IN TXT protonmail-verification=a42447901e320064d13e536db4d73ce600d715b7
paul IN TXT v=spf1 include:_spf.protonmail.ch mx ~all
paul IN TXT v=DMARC1; p=none
paul IN MX 10 mail.protonmail.ch.
paul IN MX 20 mailsec.protonmail.ch.
paul IN MX 42 blowfish
paul IN MX 42 fishfinger
protonmail._domainkey.paul IN CNAME protonmail.domainkey.d4xua2siwqfhvecokhuacmyn5fyaxmjk6q3hu2omv2z43zzkl73yq.domains.proton.ch.
protonmail2._domainkey.paul IN CNAME protonmail2.domainkey.d4xua2siwqfhvecokhuacmyn5fyaxmjk6q3hu2omv2z43zzkl73yq.domains.proton.ch.
protonmail3._domainkey.paul IN CNAME protonmail3.domainkey.d4xua2siwqfhvecokhuacmyn5fyaxmjk6q3hu2omv2z43zzkl73yq.domains.proton.ch.

fishfinger 86400 IN A 46.23.94.99
fishfinger 86400 IN AAAA 2a03:6000:6f67:624::99
git2       1800 IN CNAME fishfinger
www        1800 IN CNAME fishfinger
www.tmp    1800 IN CNAME fishfinger
www.znc    1800 IN CNAME fishfinger
bnc        1800 IN CNAME www.znc
www.dory   1800 IN CNAME fishfinger
www.footos 1800 IN CNAME fishfinger
www.fotos 1800 IN CNAME fishfinger
www.paul   1800 IN CNAME fishfinger

vulcan   86400 IN A 95.216.174.192
vulcan   86400 IN AAAA 2a01:4f9:c010:250e::1
vu       86400 IN CNAME vulcan
edge     1800 IN CNAME vulcan

zapad.sofia 86400 IN CNAME 79-100-3-54.ip.btc-net.bg.
www2         1800 IN CNAME snonux.codeberg.page.
