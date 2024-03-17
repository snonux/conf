include: "/var/nsd/etc/key.conf"

server:
	hide-version: yes
	verbosity: 1
	database: "" # disable database
	debug-mode: no

remote-control:
	control-enable: yes
	control-interface: /var/run/nsd.sock

<% for my $zone (@$dns_zones) { %>
zone:
	name: "<%= $zone %>"
	zonefile: "master/<%= $zone %>.zone"
<% } %>
