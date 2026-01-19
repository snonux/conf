#
# $OpenBSD: acme-client.conf,v 1.4 2020/09/17 09:13:06 florian Exp $
#
authority letsencrypt {
	api url "https://acme-v02.api.letsencrypt.org/directory"
	account key "/etc/acme/letsencrypt-privkey.pem"
}

authority letsencrypt-staging {
	api url "https://acme-staging-v02.api.letsencrypt.org/directory"
	account key "/etc/acme/letsencrypt-staging-privkey.pem"
}

authority buypass {
	api url "https://api.buypass.com/acme/directory"
	account key "/etc/acme/buypass-privkey.pem"
	contact "mailto:me@example.com"
}

authority buypass-test {
	api url "https://api.test4.buypass.no/acme/directory"
	account key "/etc/acme/buypass-test-privkey.pem"
	contact "mailto:me@example.com"
}

<<<<<<< HEAD
<% for my $host (@$acme_hosts) { -%>
||||||| 7b4d629
<% for my $host (@$acme_hosts) { -%>
<%   for my $prefix ('', 'www.', 'standby.') { -%>
domain <%= $prefix.$host %> {
	domain key "/etc/ssl/private/<%= $prefix.$host %>.key"
	domain full chain certificate "/etc/ssl/<%= $prefix.$host %>.fullchain.pem"
=======
<% for my $host (@$acme_hosts) {
     next if $host eq 'blowfish.buetow.org' or $host eq 'fishfinger.buetow.org'; -%>
>>>>>>> 529caf525d3c8594bcf0208697629827113dc1fc
domain <%= $host %> {
	alternative names { www.<%= $host %> }
	domain key "/etc/ssl/private/<%= $host %>.key"
	domain full chain certificate "/etc/ssl/<%= $host %>.fullchain.pem"
	sign with letsencrypt
}
domain standby.<%= $host %> {
	domain key "/etc/ssl/private/standby.<%= $host %>.key"
	domain full chain certificate "/etc/ssl/standby.<%= $host %>.fullchain.pem"
	sign with letsencrypt
}
<% } -%>
