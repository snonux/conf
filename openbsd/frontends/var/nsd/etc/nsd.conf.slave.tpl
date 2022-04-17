include: "/var/nsd/etc/key.conf"

server:
	hide-version: yes
	verbosity: 1
	database: "" # disable database

remote-control:
	control-enable: yes
	control-interface: /var/run/nsd.sock

<% for my $zone (@$dns_zones) { %>
zone:
	name: "<%= $zone %>"
	allow-notify: 23.88.35.144 blowfish.buetow.org
	request-xfr: 23.88.35.144 blowfish.buetow.org
<% } %>
