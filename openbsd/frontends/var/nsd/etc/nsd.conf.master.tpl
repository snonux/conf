include: "/var/nsd/etc/key.conf"

server:
	hide-version: yes
	verbosity: 1
	database: "" # disable database
	debug-mode: no

remote-control:
	control-enable: yes
	control-interface: /var/run/nsd.sock

zone:
	name: "buetow.org"
	zonefile: "master/buetow.org.zone"
	notify: 108.160.134.135 blowfish.buetow.org
	provide-xfr: 108.160.134.135 blowfish.buetow.org

zone:
	name: "dtail.dev"
	zonefile: "master/dtail.dev.zone"
	notify: 108.160.134.135 blowfish.buetow.org
	provide-xfr: 108.160.134.135 blowfish.buetow.org

zone:
	name: "foo.zone"
	zonefile: "master/foo.zone.zone"
	notify: 108.160.134.135 blowfish.buetow.org
	provide-xfr: 108.160.134.135 blowfish.buetow.org

zone:
	name: "foo.surf"
	zonefile: "master/foo.surf.zone"
	notify: 108.160.134.135 blowfish.buetow.org
	provide-xfr: 108.160.134.135 blowfish.buetow.org

zone:
	name: "snonux.de"
	zonefile: "master/snonux.de.zone"
	notify: 108.160.134.135 blowfish.buetow.org
	provide-xfr: 108.160.134.135 blowfish.buetow.org

zone:
	name: "irregular.ninja"
	zonefile: "master/irregular.ninja.zone"
	notify: 108.160.134.135 blowfish.buetow.org
	provide-xfr: 108.160.134.135 blowfish.buetow.org

zone:
	name: "sidewalk.ninja"
	zonefile: "master/sidewalk.ninja.zone"
	notify: 108.160.134.135 blowfish.buetow.org
	provide-xfr: 108.160.134.135 blowfish.buetow.org

zone:
	name: "snonux.me"
	zonefile: "master/snonux.me.zone"
	notify: 108.160.134.135 blowfish.buetow.org
	provide-xfr: 108.160.134.135 blowfish.buetow.org
