package rnodes

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/etchosts"
	"github.com/snonux/conf/gonf/paths"
)

// Base manages the host baseline r0-r2 were installed with by hand (f3s blog
// part 4, task ok2). The values are the live ones of 2026-09-25, Rocky 9.8;
// only the file layout is cleaned up.
//
// Deliberately NOT managed here:
//   - static IP (nmcli, enp0s5 manual, gw/dns 192.168.1.1) and hostnamectl:
//     bootstrap, a wrong value cuts the node off.
//   - the timezone: rocky_timezone.
//   - wg0 configs, keys, peers: ~/git/wireguardmeshgenerator; perms,
//     wireguard-tools and wg-quick@wg0 enablement: WireGuard (wireguard.go).
//
// SELinux is enforcing on r0-r2 and gonf keeps no file labels, so every task
// that writes into /etc relabels what it changed (restorecon).
//
// Register with OnCluster(cluster.NameRockyK3s).
type Base struct {
	RequiresRoot
}

const (
	etcHosts        = "/etc/hosts"
	sshdDropIn      = "/etc/ssh/sshd_config.d/00-f3s-auth.conf"
	sshdAnacondaIn  = "/etc/ssh/sshd_config.d/01-permitrootlogin.conf"
	dracutNVMe      = "/etc/dracut.conf.d/nvme.conf"
	dracutNVMeDup   = "/etc/dracut.conf.d/lvm-nvme.conf"
	lvmConf         = "/etc/lvm/lvm.conf"
	baseAssetsPath  = "base/"
	wireGuardDomain = "wireguard_t"
)

// Hosts renders /etc/hosts (stock header, registry.lan -> 127.0.0.1, LAN and
// wg0 rows from the shared etchosts inventory).
//
// Hosts renders the whole file from the rows the f-hosts' /etc/hosts
// (freebsd.Base.Hosts) shares. The hand-made file listed the wg0 names twice,
// in "IP short fqdn" and in "IP fqdn short" form, and lacked f3, rocky, the
// Pis; registry.lan.buetow.org -> 127.0.0.1 (the in-cluster
// registry's NodePort on the node itself, f3s blog part 7) stays.
func (Base) Hosts() {
	hosts := InstallFile(etcHosts, paths.RNodeAsset(baseAssetsPath+"hosts.tmpl"),
		RootOwned, WithTemplateData(etchosts.TemplateData()))
	relabel(etcHosts, hosts)
}

// SSHD installs the sshd auth drop-in (root by key, no passwords), validated
// with sshd -t; reloads sshd on change.
//
// SSHD owns the login policy in a drop-in that sorts before every other one
// (sshd keeps the first value per keyword). It keeps the live policy:
// PermitRootLogin yes (gonf and ops log in as root with a key),
// PasswordAuthentication no, KbdInteractiveAuthentication no.
//
// The candidate must pass "sshd -t" on its own, and the full config again
// before sshd is reloaded: a broken config would lock gonf out. A reload keeps
// open sessions. The Anaconda 01-permitrootlogin.conf (r0 only, PermitRootLogin
// yes) is shadowed by the drop-in and removed.
func (Base) SSHD() {
	dropIn := InstallFile(sshdDropIn, paths.RNodeAsset(baseAssetsPath+"sshd-00-auth.conf"),
		RootPrivate, WithValidation("/usr/sbin/sshd", List("-t", "-f", CandidatePath)))
	anaconda := NoFile(sshdAnacondaIn, DependsOn(dropIn))
	relabeled := relabel(sshdDropIn, dropIn)
	Command("sh", List("-c", "/usr/sbin/sshd -t && systemctl reload sshd"),
		OnChange(dropIn, anaconda), DependsOn(relabeled))
}

// Nvme keeps the NVMe initramfs drivers (dracut, rebuilt only on change, no
// reboot) and lvm.conf use_devicesfile = 0.
//
// Nvme keeps the two prerequisites of the NVMe-emulated bhyve disk (f3s
// blog part 4): the nvme drivers in every initramfs, and LVM scanning all
// devices, since the PV moved from /dev/vda to /dev/nvme0n1.
//
// r1 had the dracut lines in two files (nvme.conf without hostonly=no plus
// lvm-nvme.conf with both lines); nvme.conf now holds both and lvm-nvme.conf
// goes. Only a changed dracut file rebuilds the initramfs, for every
// installed kernel (--regenerate-all, a plain -f would only rebuild the
// running one). Nothing reboots: the nightly power cycle boots the new
// image. With hostonly=no dracut does not copy lvm.conf into the initramfs,
// so an lvm.conf change needs no rebuild.
//
// lvm.conf is shared with the distro: gonf owns only the use_devicesfile
// line. The keyed line is written unindented; lvm.conf does not care.
func (Base) Nvme() {
	conf := InstallFile(dracutNVMe, paths.RNodeAsset(baseAssetsPath+"dracut-nvme.conf"),
		RootOwned)
	dup := NoFile(dracutNVMeDup)
	relabel(dracutNVMe, conf)
	Sh("dracut -f --regenerate-all", OnChange(conf, dup))

	lvm := File(lvmConf,
		WithKeyedLine("use_devicesfile =", "use_devicesfile = 0"),
		RootOwned)
	relabel(lvmConf, lvm)
}

// SelinuxWireguard keeps SELinux enforcing with only wireguard_t permissive
// (semanage permissive -a).
//
// SelinuxWireguard marks the wireguard_t domain permissive, the one local
// SELinux customization found on r0-r2 (set by hand with the WireGuard setup).
// semanage comes with policycoreutils-python-utils. The command only runs
// when wireguard_t is not listed yet; the rest of the policy stays enforcing.
func (Base) SelinuxWireguard() {
	pkg := Package("policycoreutils-python-utils")
	Command("semanage", List("permissive", "-a", wireGuardDomain),
		Unless("sh", List("-c", "semanage permissive -l | grep -Eq '^"+wireGuardDomain+"[[:space:]]*$'")),
		DependsOn(pkg))
}

// FirewalldOff keeps firewalld stopped and disabled (decided in task tj2,
// f3s-k3s install.md).
//
// FirewalldOff keeps firewalld stopped and disabled, the decision of task
// tj2 (f3s-k3s skill, install.md "Host firewall on r0/r1/r2"): its nftables
// rules fight kube-proxy and flannel, and the nodes sit on the LAN and the
// wg0 mesh only. The package stays installed.
func (Base) FirewalldOff() {
	NoService("firewalld")
}

// relabel restores the SELinux label of path after changed rewrote it. gonf
// writes a new file and renames it into place, so it would otherwise carry
// the directory's default type (etc_t for /etc/hosts instead of
// net_conf_t).
func relabel(path string, changed Resource) Resource {
	return Sh("restorecon "+path, OnChange(changed))
}
