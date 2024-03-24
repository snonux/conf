$ORIGIN buetow.org.
$TTL 4h
@        IN  SOA  fishfinger.buetow.org. hostmaster.buetow.org. (
                  <%= time() %>   ; serial
                  1h              ; refresh
                  30m             ; retry
                  7d              ; expire
                  1h )            ; negative
         IN NS    fishfinger.buetow.org.
         IN NS    blowfish.buetow.org.

	     IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
         IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
www      IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
www      IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
standby  IN A <%= $ips->{current_standby}{ipv4} %> ; Enable failover
standby  IN AAAA <%= $ips->{current_standby}{ipv6} %> ; Enable failover
master   IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
master   IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover

         IN MX 10 fishfinger.buetow.org.
         IN MX 20 blowfish.buetow.org.

cool     IN NS ns-75.awsdns-09.com.
cool     IN NS ns-707.awsdns-24.net.
cool     IN NS ns-1081.awsdns-07.org.
cool     IN NS ns-1818.awsdns-35.co.uk.

paul         300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
paul         300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
www.paul     300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
www.paul     300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
standby.paul  300 IN A <%= $ips->{current_standby}{ipv4} %> ; Enable failover
standby.paul  300 IN AAAA <%= $ips->{current_standby}{ipv6} %> ; Enable failover

tmp          300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
tmp          300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
www.tmp      300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
www.tmp      300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
standby.tmp   300 IN A <%= $ips->{current_standby}{ipv4} %> ; Enable failover
standby.tmp   300 IN AAAA <%= $ips->{current_standby}{ipv6} %> ; Enable failover

dory         300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
dory         300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
www.dory     300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
www.dory     300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
standby.dory  300 IN A <%= $ips->{current_standby}{ipv4} %> ; Enable failover
standby.dory  300 IN AAAA <%= $ips->{current_standby}{ipv6} %> ; Enable failover

solarcat        300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
solarcat        300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
www.solarcat    300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
www.solarcat    300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
standby.solarcat 300 IN A <%= $ips->{current_standby}{ipv4} %> ; Enable failover
standby.solarcat 300 IN AAAA <%= $ips->{current_standby}{ipv6} %> ; Enable failover

fotos        300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
fotos        300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
www.fotos    300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
www.fotos    300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
standby.fotos 300 IN A <%= $ips->{current_standby}{ipv4} %> ; Enable failover
standby.fotos 300 IN AAAA <%= $ips->{current_standby}{ipv6} %> ; Enable failover

git          300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
git          300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
www.git      300 IN A <%= $ips->{current_master}{ipv4} %> ; Enable failover
www.git      300 IN AAAA <%= $ips->{current_master}{ipv6} %> ; Enable failover
standby.git   300 IN A <%= $ips->{current_standby}{ipv4} %> ; Enable failover
standby.git   300 IN AAAA <%= $ips->{current_standby}{ipv6} %> ; Enable failover

blowfish   14400 IN A 23.88.35.144
blowfish   14400 IN AAAA 2a01:4f8:c17:20f1::42
blowfish         IN MX 10 fishfinger.buetow.org.
blowfish         IN MX 20 blowfish.buetow.org.
fishfinger 14400 IN A 46.23.94.99
fishfinger 14400 IN AAAA 2a03:6000:6f67:624::99
fishfinger       IN MX 10 fishfinger.buetow.org.
fishfinger       IN MX 20 blowfish.buetow.org.

git1        1800 IN CNAME blowfish.buetow.org.
git2        1800 IN CNAME fishfinger.buetow.org.

zapad.sofia    14400 IN CNAME 79-100-3-54.ip.btc-net.bg.
www2           14400 IN CNAME snonux.codeberg.page.
znc            1800 IN CNAME fishfinger.buetow.org.
www.znc        1800 IN CNAME fishfinger.buetow.org.
standby.znc        1800 IN CNAME fishfinger.buetow.org.
bnc            1800 IN CNAME fishfinger.buetow.org.
www.bnc        1800 IN CNAME fishfinger.buetow.org.
	
protonmail._domainkey.paul  IN CNAME protonmail.domainkey.d4xua2siwqfhvecokhuacmyn5fyaxmjk6q3hu2omv2z43zzkl73yq.domains.proton.ch.
protonmail2._domainkey.paul IN CNAME protonmail2.domainkey.d4xua2siwqfhvecokhuacmyn5fyaxmjk6q3hu2omv2z43zzkl73yq.domains.proton.ch.
protonmail3._domainkey.paul IN CNAME protonmail3.domainkey.d4xua2siwqfhvecokhuacmyn5fyaxmjk6q3hu2omv2z43zzkl73yq.domains.proton.ch.
paul IN TXT protonmail-verification=a42447901e320064d13e536db4d73ce600d715b7
paul IN TXT v=spf1 include:_spf.protonmail.ch mx ~all
paul IN TXT v=DMARC1; p=none
paul IN MX 10 mail.protonmail.ch.
paul IN MX 20 mailsec.protonmail.ch.
paul IN MX 42 blowfish.buetow.org.
paul IN MX 42 fishfinger.buetow.org.

*    IN MX 10 fishfinger.buetow.org.
*    IN MX 20 blowfish.buetow.org.
