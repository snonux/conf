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

	 IN A <%= $ips->{current_master}{ipv4} %>
         IN AAAA <%= $ips->{current_master}{ipv6} %>

         IN MX 10 fishfinger.buetow.org.
         IN MX 20 blowfish.buetow.org.

cool     IN NS ns-75.awsdns-09.com.
cool     IN NS ns-707.awsdns-24.net.
cool     IN NS ns-1081.awsdns-07.org.
cool     IN NS ns-1818.awsdns-35.co.uk.

www.paul     1800 IN CNAME <%= $ips->{current_master}{fqdn} %>.
www.dory     1800 IN CNAME <%= $ips->{current_master}{fqdn} %>.
www.solarcat 1800 IN CNAME <%= $ips->{current_master}{fqdn} %>.

blowfish   14400 IN A 23.88.35.144
blowfish   14400 IN AAAA 2a01:4f8:c17:20f1::42
fishfinger 14400 IN A 46.23.94.99
fishfinger 14400 IN AAAA 2a03:6000:6f67:624::99

git1        1800 IN CNAME blowfish
git2        1800 IN CNAME fishfinger

mirror          1800 IN CNAME <%= $ips->{current_standby}{fqdn} %>.
mirror.paul     1800 IN CNAME <%= $ips->{current_standby}{fqdn} %>.
mirror.dory     1800 IN CNAME <%= $ips->{current_standby}{fqdn} %>.
mirror.solarcat 1800 IN CNAME <%= $ips->{current_standby}{fqdn} %>.
mirror.fotos    1800 IN CNAME <%= $ips->{current_standby}{fqdn} %>.

zapad.sofia    14400 IN CNAME 79-100-3-54.ip.btc-net.bg.
www2           14400 IN CNAME snonux.codeberg.page.
	
protonmail._domainkey.paul  IN CNAME protonmail.domainkey.d4xua2siwqfhvecokhuacmyn5fyaxmjk6q3hu2omv2z43zzkl73yq.domains.proton.ch.
protonmail2._domainkey.paul IN CNAME protonmail2.domainkey.d4xua2siwqfhvecokhuacmyn5fyaxmjk6q3hu2omv2z43zzkl73yq.domains.proton.ch.
protonmail3._domainkey.paul IN CNAME protonmail3.domainkey.d4xua2siwqfhvecokhuacmyn5fyaxmjk6q3hu2omv2z43zzkl73yq.domains.proton.ch.
paul IN TXT protonmail-verification=a42447901e320064d13e536db4d73ce600d715b7
paul IN TXT v=spf1 include:_spf.protonmail.ch mx ~all
paul IN TXT v=DMARC1; p=none
paul IN MX 10 mail.protonmail.ch.
paul IN MX 20 mailsec.protonmail.ch.
paul IN MX 42 blowfish
paul IN MX 42 fishfinger
paul 1800 IN A <%= $ips->{current_master}{ipv4} %>
paul 1800 IN AAAA <%= $ips->{current_master}{ipv6} %>

* 1800 IN CNAME <%= $ips->{current_master}{fqdn} %>.
