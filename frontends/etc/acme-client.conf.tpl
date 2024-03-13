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

<% for my $host (@$acme_hosts) { -%>
<%   for my $prefix ('', 'www.', 'mirror.') { -%>
domain <%= $prefix.$host %> {
	domain key "/etc/ssl/private/<%= $prefix.$host %>.key"
	domain full chain certificate "/etc/ssl/<%= $prefix.$host %>.fullchain.pem"
	sign with letsencrypt
}
<%   } -%>
<% } -%>

# Mail server domains
domain <%= "$hostname.$domain" %> {
	domain key "/etc/ssl/private/<%= "$hostname.$domain" %>.key"
	domain full chain certificate "/etc/ssl/<%= "$hostname.$domain" %>.fullchain.pem"
	sign with letsencrypt
}
