package workstation

import (
	. "github.com/snonux/gonf/api"
)

// earthStunnelConfig is Earth's NFS-over-TLS client: the loopback listener
// its noauto NFS mounts use, tunnelled with mutual TLS to the CARP storage
// VIP 10.20.0.138 on f0/f1. The certificates stay owned by the host.
const earthStunnelConfig = `cert = /etc/stunnel/earth-stunnel.pem
CAfile = /etc/stunnel/ca-cert.pem
client = yes
verify = 2

[nfs-ha]
accept = 127.0.0.1:2323
connect = 10.20.0.138:2323
`

// OptsNFSClientStunnel limits the canonical transport to Earth.
func (System) OptsNFSClientStunnel() TaskOptions {
	return TaskOptions{WhenLinux(), WhenHostnameIn("earth")}
}

// NFSClientStunnel manages Earth's accepted NFS TLS endpoint without restarting transport.
func (System) NFSClientStunnel() {
	File("/etc/stunnel/stunnel.conf", WithContent(earthStunnelConfig), RootOwned)
}
