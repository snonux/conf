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
	<% for my $slave_ip (qw/108.160.134.135 46.23.94.99/) { %>
	notify: <%= $slave_ip %> blowfish.buetow.org
	provide-xfr: <%= $slave_ip %> blowfish.buetow.org
	<% } -%>
<% } %>
