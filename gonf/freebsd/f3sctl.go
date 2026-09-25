package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// F3sctl manages the restricted account f3sctl (github.com/snonux/f3sctl) on
// pi0/pi1 logs in as to stop guests, power a host off and export the zusb
// pool (f3s/freebsd-hosts/f3sctl/README.md, blog part 10). Until task jk2 it
// was set up by hand with setup-agent-freebsd.sh (removed with jk2);
// every task here adopts that live state unchanged:
//   - user f3sctl (uid/gid 1003 on all four), home /var/db/f3sctl 0750,
//     login shell /bin/sh: sshd runs the ForceCommand through the shell, so
//     nologin would break the agent instead of hardening it;
//   - the key in the root-owned /etc/ssh/authorized_keys.d/f3sctl, outside
//     the account's home so it cannot re-authorise itself, pinned with
//     from= to pi0/pi1 (LAN and wg0);
//   - the "Match User f3sctl" block in sshd_config (ForceCommand, no TTY,
//     no forwarding).
//
// The doas rules for the four root verbs (poweroff, zusb-unload, probe,
// carp-quiesce) are NOT here: doas.conf has one owner, Base.Doas
// (freebsd_base_doas, f3s/freebsd-hosts/base/doas.conf). A second recipe
// editing the same file would fight it.
//
// Register with OnCluster(cluster.NameFreeBSD); every task applies to all
// four hosts alike.
type F3sctl struct {
	RequiresRoot
}

const (
	f3sctlUser       = "f3sctl"
	f3sctlHome       = "/var/db/f3sctl"
	authorizedKeysD  = "/etc/ssh/authorized_keys.d"
	f3sctlKeyFile    = authorizedKeysD + "/f3sctl"
	sshdConfig       = "/etc/ssh/sshd_config"
	sshdBin          = "/usr/sbin/sshd"
	f3sctlAssetsPath = "f3sctl/"
)

// DescPackage returns the description for the f3sctl package.
func (F3sctl) DescPackage() string {
	return "Install the f3sctl package (the agent binary /usr/local/bin/f3sctl) from the custom pkg repo"
}

// OptsPackage needs the custom repository: f3sctl is only published there.
func (F3sctl) OptsPackage() TaskOptions {
	return TaskOptions{Privileged(), Needs("freebsd_base_pkg_repo")}
}

// Package installs f3sctl; pkg upgrades it with the rest of the system.
func (F3sctl) Package() {
	Package("f3sctl")
}

// DescAccount returns the description for the agent account.
func (F3sctl) DescAccount() string {
	return "Ensure user/group f3sctl (home " + f3sctlHome + " 0750, shell /bin/sh)"
}

// Account ensures the account and its home. gonf's User is additive-only
// and cannot pin a uid, so a fresh host gets the next free uid (the live
// hosts all have 1003); nothing refers to the number. pw leaves a new
// account's password disabled ("*"), as the setup script's "-w no" did.
func (F3sctl) Account() {
	user := User(f3sctlUser, WithPrimaryGroup(f3sctlUser), WithHome(f3sctlHome),
		WithShell("/bin/sh"))
	EnsureDir(f3sctlHome, Perm(0o750, f3sctlUser+":"+f3sctlUser), DependsOn(user))
}

// DescKey returns the description for the agent's authorized key.
func (F3sctl) DescKey() string {
	return "Install " + f3sctlKeyFile + " (pi0/pi1 from=, restrict, forced f3sctl agent; root-owned)"
}

// Key installs the authorized_keys line. It holds the PUBLIC key only (the
// private one stays on pi0/pi1 in /var/db/f3sctl), so it lives in the repo:
// f3s/freebsd-hosts/f3sctl/authorized_keys. Rotating the key is an edit
// there plus a deploy. The file and its directory stay root-owned and
// world-readable, as sshd's StrictModes requires.
func (F3sctl) Key() {
	EnsureDir(authorizedKeysD, Perm(0o755, Root))
	InstallFile(f3sctlKeyFile, paths.FHostAsset(f3sctlAssetsPath+"authorized_keys"),
		Perm(0o644, Root))
}

// DescSSHD returns the description for sshd_config.
func (F3sctl) DescSSHD() string {
	return "Install sshd_config (stock + Match User f3sctl block), validated with sshd -t; reload sshd on change"
}

// OptsSSHD orders the Match block after the key file and account it names.
func (F3sctl) OptsSSHD() TaskOptions {
	return TaskOptions{Privileged(), Needs("account", "key")}
}

// SSHD owns the whole sshd_config: the FreeBSD 15.1 stock file plus the
// Match User f3sctl block, byte for byte as on f1-f3 (f0 had lost one
// comment line in an earlier etc merge). The whole file rather than line
// edits, because only a content-managed file can be validated as a
// candidate with "sshd -t -f" before it goes live: a broken sshd_config
// locks gonf and paul out of a host whose console is only reachable in
// person. sshd is reloaded, never restarted, so open sessions survive, and
// only after the live config passes "sshd -t" again.
//
// After a FreeBSD upgrade merges a new stock sshd_config, refresh the asset
// from a host (keeping the Match block last) or gonf reverts the merge.
func (F3sctl) SSHD() {
	conf := InstallFile(sshdConfig, paths.FHostAsset(f3sctlAssetsPath+"sshd_config"),
		Perm(0o644, Root), WithValidation(sshdBin, List("-t", "-f", CandidatePath)))
	Command("sh", List("-c", sshdBin+" -t && service sshd reload"), OnChange(conf))
}
