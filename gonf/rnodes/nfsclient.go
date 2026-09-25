package rnodes

import (
	"fmt"

	. "github.com/snonux/gonf/api"
)

// NFS client side of the r-nodes: the k3s volumes are an NFSv4 mount of
// f3s-storage-ha (the CARP VIP 192.168.1.138 on f0/f1) tunnelled through a
// local stunnel client, 127.0.0.1:2323 -> 192.168.1.138:2323 with mutual TLS.
// These tasks adopt the live r0-r2 state of 2026-09-25 (blog part 6, "Client
// Configuration for NFS via Stunnel", is outdated where it says soft mount).
//
// None of them restarts stunnel or remounts NFS: r0-r2 run live pods on the
// hard mount, and gonf applies to the three nodes in parallel. A changed
// stunnel.conf or fstab line takes effect with the nightly f-host power cycle
// (or a manual restart, one node at a time).

const (
	// nfsFstabKey owns the one /etc/fstab line of the NFS mount.
	nfsFstabKey = "127.0.0.1:/k3svolumes "
	// nfsFstabLine is hard,timeo=600 on purpose: check-nfs-mount.sh repairs
	// with the fstab options (see its header), so this line is also the repair
	// mount's option set, and a soft mount can fail writes with EIO.
	nfsFstabLine = nfsFstabKey +
		"/data/nfs/k3svolumes nfs4 port=2323,_netdev,hard,timeo=600,retrans=3 0 0"

	// stunnelConfFormat is the client config; %s is the inventory host name
	// (r0, r1, r2) naming its client certificate+key PEM.
	stunnelConfFormat = `cert = /etc/stunnel/%s-stunnel.pem
CAfile = /etc/stunnel/ca-cert.pem
client = yes
verify = 2

[nfs-ha]
accept = 127.0.0.1:2323
connect = 192.168.1.138:2323
`
)

// DescNFSClientPackages returns the NFS client packages task description.
func (Maintenance) DescNFSClientPackages() string {
	return "Install the NFS client packages (stunnel, nfs-utils)"
}

// NFSClientPackages installs stunnel (the TLS tunnel to the storage VIP) and
// nfs-utils (mount.nfs4, nfsidmap, rpcbind dependency).
func (Maintenance) NFSClientPackages() {
	Packages("stunnel", "nfs-utils")
}

// DescNFSClientStunnel returns the stunnel client task description.
func (Maintenance) DescNFSClientStunnel() string {
	return "Manage the stunnel NFS client config and key permissions (no restart; next boot)"
}

// OptsNFSClientStunnel orders the config after the stunnel package.
func (Maintenance) OptsNFSClientStunnel() TaskOptions {
	return TaskOptions{Needs("rnodes_nfs_client_packages")}
}

// NFSClientStunnel renders /etc/stunnel/stunnel.conf per host and keeps the
// distro stunnel.service enabled and running.
//
// The client certificate+key (/etc/stunnel/rN-stunnel.pem) and the CA
// certificate stay on the host, created by hand from the f0 stunnel CA; gonf
// only tightens the key file to 0600 (it was world-readable 0644). EnsureFile
// keeps the bytes and never reads them into the plan.
//
// A changed stunnel.conf does not restart stunnel: that would drop the NFS
// transport on all three nodes at once. It is read at the next boot.
//
// /etc/stunnel/stunnel.pem on r0 was an unused self-signed "nfs-stunnel"
// certificate from the first setup (2025-07); no config referenced it.
func (Maintenance) NFSClientStunnel() {
	for _, host := range ClusterHosts() {
		WhenHostname(host, func() {
			File("/etc/stunnel/stunnel.conf",
				WithContent(fmt.Sprintf(stunnelConfFormat, host)),
				Perm(0o644, Root))
			EnsureFile("/etc/stunnel/"+host+"-stunnel.pem", Perm(0o600, Root))
		})
	}
	NoFile("/etc/stunnel/stunnel.pem")
	Service("stunnel")
}

// DescNFSClientIdmapd returns the NFSv4 idmapd domain task description.
func (Maintenance) DescNFSClientIdmapd() string {
	return "Set the NFSv4 id-mapping Domain in /etc/idmapd.conf"
}

// NFSClientIdmapd owns the Domain line of /etc/idmapd.conf; it must match
// the f-hosts' nfsuserd_flags="-domain lan.buetow.org", or file owners map
// to nobody. The keyed line
// also drops the duplicate Domain line r0 had. nfsidmap reads the file per
// upcall, so nothing needs a restart.
func (Maintenance) NFSClientIdmapd() {
	File("/etc/idmapd.conf",
		WithKeyedLine("Domain =", "Domain = lan.buetow.org"),
		Perm(0o644, Root))
}

// DescNFSClientSysctl returns the inotify sysctl task description.
func (Maintenance) DescNFSClientSysctl() string {
	return "Raise fs.inotify.max_user_instances to 512 on r-nodes"
}

// NFSClientSysctl raises the inotify instance limit (default 128): without
// it nfs-idmapd could fail to start with "Too many open files" (blog part 6),
// the k3s pods' file watchers taking the default budget. A changed file is
// loaded at once; loading it does not touch the mount.
func (Maintenance) NFSClientSysctl() {
	const path = "/etc/sysctl.d/99-inotify.conf"
	conf := File(path,
		WithContent("fs.inotify.max_user_instances = 512\n"),
		Perm(0o644, Root))
	Sh("sysctl --load "+path, OnChange(conf))
}

// DescNFSClientUnits returns the NFS client units task description.
func (Maintenance) DescNFSClientUnits() string {
	return "Enable and start rpcbind and nfs-client.target"
}

// OptsNFSClientUnits orders the units after nfs-utils.
func (Maintenance) OptsNFSClientUnits() TaskOptions {
	return TaskOptions{Needs("rnodes_nfs_client_packages")}
}

// NFSClientUnits keeps rpcbind and nfs-client.target enabled. nfs-idmapd is
// static and inactive on a client (NFSv4 client id mapping uses nfsidmap
// upcalls), so it is not managed here.
func (Maintenance) NFSClientUnits() {
	Service("rpcbind")
	Service("nfs-client.target")
}

// DescNFSClientFstab returns the NFS fstab line task description.
func (Maintenance) DescNFSClientFstab() string {
	return "Manage the /etc/fstab line of the k3s NFS volume mount (no remount)"
}

// NFSClientFstab owns the one fstab line of /data/nfs/k3svolumes; the rest
// of fstab stays hand-managed. A changed line only reloads systemd so the
// generated data-nfs-k3svolumes.mount matches; the live mount is left alone
// and picks up new options at the next boot (never remount from gonf).
func (Maintenance) NFSClientFstab() {
	fstab := File("/etc/fstab",
		WithKeyedLine(nfsFstabKey, nfsFstabLine),
		Perm(0o644, Root))
	DaemonReload(OnChange(fstab))
}
