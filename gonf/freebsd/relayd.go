package freebsd

import (
	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Relayd manages the LAN ingress of the k3s cluster on the CARP pair f0/f1
// (f3s-k3s skill, references/ingress.md; blog part 7): relayd listens on the
// CARP VIP 192.168.1.138 ports 80 and 443 and forwards plain TCP to Traefik
// on r0-r2. It is pure TCP passthrough, Traefik terminates TLS for
// *.f3s.lan.buetow.org, so relayd holds no certificate. PF has to be enabled
// for relayd; its ruleset is only "skip lo0, pass everything".
//
// Both files were hand-installed (owner paul:wheel, 0644) and byte-identical
// on f0 and f1 when gonf took over on 2026-09-25 (task mk2); the sources now
// live in f3s/freebsd-hosts/relayd. The only difference the takeover makes is
// the owner, root:wheel: a metadata-only repair, which is not a change, so it
// neither reloads PF nor restarts relayd.
//
// Both hosts carry the same ingress config on purpose: after a CARP failover
// f1 must serve the VIP exactly like f0. On the BACKUP relayd runs too (the
// VIP address stays configured on re0, CARP only stops answering for it), so
// a failover needs no service start.
//
// Safety rules, because a broken pf.conf or relayd.conf locks the LAN out:
//   - each file is validated as a candidate (pfctl -nf, relayd -n -f) and
//     only published when the check passes; a failed check changes nothing
//     and fires no reload;
//   - PF is reloaded with "pfctl -f" only when pf.conf changed; gonf never
//     disables or stops PF;
//   - relayd is RESTARTED, not reloaded, only when relayd.conf changed: a
//     reload (SIGHUP) keeps state such as TLS keypairs, and on 2026-06-30 a
//     never-restarted relayd kept serving an expired certificate after the
//     switch to passthrough.
//
// Roll out one host at a time, BACKUP first (push to f1, check, then f0),
// and after each PF reload open a NEW ssh session (LAN and WireGuard) while
// the old one stays up.
//
// The backend table lists r0-r2's LAN addresses literally. The r-node
// inventory (rnodes.K3sNode) only carries their wg0 addresses, and relayd
// must reach the nodes on the LAN, so deriving the table would mean a second
// per-host field used only here; the table changes only with the k3s nodes.
//
// Register with OnCluster(cluster.NameFreeBSD); the bodies narrow to the CARP
// members with WhenHostname, since f2/f3 have no VIP and run no relayd.
type Relayd struct {
	RequiresRoot
}

const (
	pfConf     = "/etc/pf.conf"
	relaydConf = "/usr/local/etc/relayd.conf"
)

// relaydRcConfKeys are the rc.conf keys for PF and its log device, in the
// live spelling. relayd_enable is not here: Service("relayd") owns it (the
// FreeBSD service backend enables through rc.conf), so there is one owner
// per key.
var relaydRcConfKeys = []struct{ key, value string }{
	{"pf_enable", "YES"},
	{"pflog_enable", "YES"},
}

// DescRcConf returns the description for the PF rc.conf keys.
func (Relayd) DescRcConf() string {
	return "rc.conf pf_enable/pflog_enable=YES on f0/f1 (PF for the LAN relayd; read at boot, nothing restarted)"
}

// WhenRcConf narrows the task to the CARP members.
func (Relayd) WhenRcConf() TaskOption { return WhenHostnameIn(carpMembers...) }

// RcConf owns pf_enable and pflog_enable. rc.conf is read at boot; PF and
// pflog already run on both hosts, and nothing here starts or stops them.
func (Relayd) RcConf() {
	opts := []FileOption{RootOwned, WithName("rc-conf-relayd")}
	for _, kv := range relaydRcConfKeys {
		opts = append(opts, WithShellVar(kv.key, kv.value))
	}
	File(rcConf, opts...)
}

// DescPf returns the description for the PF ruleset.
func (Relayd) DescPf() string {
	return "/etc/pf.conf on f0/f1 (pass-all for relayd), validated with pfctl -nf; pfctl -f only when it changed"
}

// WhenPf narrows the task to the CARP members.
func (Relayd) WhenPf() TaskOption { return WhenHostnameIn(carpMembers...) }

// Pf installs the ruleset after a pfctl -nf check of the candidate and
// loads it with pfctl -f only when the content changed. pfctl -f replaces
// the rules atomically and keeps PF enabled and its states; a parse error
// would leave the old rules in place, and the candidate check keeps it from
// getting that far.
func (Relayd) Pf() {
	conf := InstallFile(pfConf, paths.FHostAsset("relayd/pf.conf"), RootOwned,
		WithValidation("pfctl", List("-nf", CandidatePath)))
	Sh("pfctl -f "+pfConf, OnChange(conf))
}

// DescDaemon returns the description for relayd.
func (Relayd) DescDaemon() string {
	return "relayd on f0/f1: package, " + relaydConf + " (VIP :80/:443 -> r0-r2, TCP passthrough), validated with relayd -n; enabled, running, restarted only when the config changed"
}

// OptsDaemon records PF first: relayd depends on PF being enabled.
func (Relayd) OptsDaemon() TaskOptions {
	return TaskOptions{Privileged(), Needs(Relayd.Pf)}
}

// WhenDaemon narrows the task to the CARP members.
func (Relayd) WhenDaemon() TaskOption { return WhenHostnameIn(carpMembers...) }

// Daemon installs relayd and its config after a relayd -n check of the
// candidate, keeps the service enabled (relayd_enable="YES") and running,
// and restarts it only when relayd.conf changed (see the Relayd comment for
// why restart and not reload).
func (Relayd) Daemon() {
	pkg := Package("relayd")
	conf := InstallFile(relaydConf, paths.FHostAsset("relayd/relayd.conf"), RootOwned,
		WithValidation("relayd", List("-n", "-f", CandidatePath)), DependsOn(pkg))
	Service("relayd", WithRestart, OnChange(conf))
}
