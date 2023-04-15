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
tmp      3600 IN CNAME blowfish
dory     3600 IN CNAME blowfish
footos   3600 IN CNAME blowfish
paul 86400 IN A 23.88.35.144
paul 86400 IN AAAA 2a01:4f8:c17:20f1::42
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
git2       3600 IN CNAME fishfinger
www        3600 IN CNAME fishfinger
www.tmp    3600 IN CNAME fishfinger
www.znc    3600 IN CNAME fishfinger
bnc        3600 IN CNAME www.znc
www.dory   3600 IN CNAME fishfinger
www.footos 3600 IN CNAME fishfinger
www.paul   3600 IN CNAME fishfinger

vulcan   86400 IN A 95.216.174.192
vulcan   86400 IN AAAA 2a01:4f9:c010:250e::1
vu       86400 IN CNAME vulcan
edge     3600 IN CNAME vulcan

babylon5   86400 IN A 5.75.172.148
babylon5   86400 IN AAAA 2a01:4f8:1c1c:4be9::1
babylon5-2 86400 IN A 5.75.172.148
babylon5-2 86400 IN AAAA 2a01:4f8:1c1c:4be9::2
babylon5-3 86400 IN A 5.75.172.148
babylon5-3 86400 IN AAAA 2a01:4f8:1c1c:4be9::3
cloud      3600 IN CNAME babylon5
bag        3600 IN CNAME babylon5-2
anki       3600 IN CNAME babylon5-3
wolke7     3600 IN CNAME babylon5

zapad.sofia 86400 IN CNAME 79-100-3-54.ip.btc-net.bg.
www2         3600 IN CNAME snonux.codeberg.page.
