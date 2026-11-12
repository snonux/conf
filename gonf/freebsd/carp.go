package freebsd

import (
	"fmt"
	"strconv"
	"strings"

	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Carp manages the CARP layer of the storage pair f0/f1 (the f3s-storage-ha
// VIP 10.20.0.138, see the f3s-storage skill, references/carp.md):
//   - /usr/local/bin/carp, the state/management CLI, on both;
//   - on f0 only, the minutely carp-auto-failback.sh that reclaims MASTER
//     after f0 comes back, with its cron job and log rotation;
//   - carpcontrol.sh, which starts/stops the NFS server and stunnel on a
//     state change, and the devd rule that calls it (task gk2);
//   - carp_load in /boot/loader.conf and the rc.conf CARP alias line
//     ifconfig_<if>_alias0, whose password comes from the vault (task gk2).
//     <if> is the interface carrying the host's lab addresses: re0, or
//     vm_public (the bridge vm-public) after the task 764 move;
//     CarpNode.Interface;
//   - the boot gate (task 764): loader.conf net.inet.carp.allow="0" and the
//     rc.d hook that allows CARP once re0 carries the peer's advertisements
//     (BootGate).
//
// All of it used to be hand-installed; the sources now live in
// f3s/freebsd-hosts/carp. The live files matched them byte for byte when
// gonf took over (2026-09-25), so the takeover changed nothing on the hosts
// except moving the devd rule from /etc/devd.conf into its own drop-in.
//
// Nothing here touches the running CARP state: no ifconfig, no netif or
// carpcontrol.sh run. rc.conf and loader.conf are read at boot, so a changed
// alias line or carp_load takes effect on the next reboot only (changing the
// password on a live pair needs both hosts at once, else each side drops
// the other's advertisements and both become MASTER; see the rotation
// procedure in the f3s-storage skill, references/carp.md). The only daemon
// touched is devd, restarted when its rule changes: devd re-reads its rules
// on start, and a restart neither changes the CARP state nor replays a
// transition.
//
// Register with OnCluster(cluster.NameFreeBSD); the bodies narrow further to
// the CARP members with WhenHostname, since f2/f3 carry no CARP interface.
// Each member carries a CarpNode (gonf/cluster).
type Carp struct {
	RequiresRoot
}

// CarpNode is a CARP member's per-host setting (host data, see gonf/cluster).
// AdvSkew is the advskew of the vhid: 0 (omitted from the line, the kernel
// default) wins the election, so f0 has 0 and f1, the standby, 100.
// Interface carries the VIP: LabBridge ("vm-public") once the host's
// addresses moved onto the bridge (task 764, BridgeMigration), "re0"
// before. It must match the host's rc.conf: the move rewrites the line
// itself, and this field is switched after the move is confirmed.
type CarpNode struct {
	AdvSkew   int
	Interface string
}

const (
	carpScript         = "/usr/local/bin/carp"
	carpFailbackScript = "/usr/local/bin/carp-auto-failback.sh"
	carpControlScript  = "/usr/local/bin/carpcontrol.sh"
	carpDevdDir        = "/usr/local/etc/devd"
	carpDevdHook       = carpDevdDir + "/carp.conf"
	devdConf           = "/etc/devd.conf"
	carpGateHelper     = "/usr/local/libexec/gonf-carp-gate"
	carpGateRC         = "/etc/rc.d/gonf_carp_gate"
	carpGateConf       = "/usr/local/etc/gonf-carp-gate.conf"
)

// carpGateLoaderLine holds every vhid in INIT from module load until the
// gate's waiter sets net.inet.carp.allow=1. ip_carp.c (FreeBSD 15.0/15.1)
// fetches net.inet.carp.allow as a tunable in its VNET sysinit, "to postpone
// carp start after module load"; carp_load in loader.conf loads the module
// before the kernel starts, so the hold is in place before netif.
const carpGateLoaderLine = `net.inet.carp.allow="0"` + "\t" +
	`# CARP held until gonf_carp_gate hears the peer on re0 (task 764); remove only together with the gate`

// carpGateSettle and carpGateDeadline are the gate's timings in seconds: the
// uplink must stay active this long before an advertisement counts (a re(4)
// link reset lasts ~4 s), and after the deadline CARP is allowed anyway so
// a sole survivor still takes the VIP.
const (
	carpGateSettle   = 10
	carpGateDeadline = 180
)

// carpMembers are the hosts that share the storage VIP; carpFailbackHost is
// the preferred MASTER that the failback cron job promotes.
var carpMembers = List("f0", "f1")

const carpFailbackHost = "f0"

// carpFailbackNewsyslogLine keeps five compressed 1 MB generations.
const carpFailbackNewsyslogLine = "/var/log/carp-auto-failback.log\t\troot:wheel\t644  5     1024  *     Z"

// carpControlOldCopies are hand-made backups of earlier carpcontrol.sh
// versions next to the live one on f0 and f1 (July 2025 and before the
// f3s-mount-keys change of 2026-05-30). The repo history keeps both.
var carpControlOldCopies = List(
	carpControlScript+".bak",
	carpControlScript+".pre-f3skeys-20260530192117",
)

// devdStripCarpBlock removes the hand-appended CARP rule from /etc/devd.conf
// once the drop-in carp.conf carries it. The awk program drops each
// "notify 0 { ... };" block that mentions carpcontrol.sh, together with the
// one blank line in front of it, and passes every other line through; on
// f0/f1 the result is the stock 162-line file. The edit goes through a temp
// file in /etc and is only moved into place if the rule is really gone, so
// a failed awk leaves devd.conf as it was.
const devdStripCarpBlock = `f=` + devdConf + `
t=$(mktemp "$f.gonf.XXXXXX") || exit 1
if awk '
function flushblank() { if (blank) { print ""; blank = 0 } }
inblock {
	buf = buf "\n" $0
	if ($0 ~ /carpcontrol\.sh/) hit = 1
	if ($0 ~ /^};[ \t]*$/) {
		if (hit) blank = 0; else { flushblank(); print buf }
		inblock = 0; hit = 0; buf = ""
	}
	next
}
/^notify 0 \{[ \t]*$/ { inblock = 1; buf = $0; next }
{ flushblank() }
/^$/ { blank = 1; next }
{ print }
END { if (inblock) { flushblank(); print buf }; flushblank() }
' "$f" >"$t" && [ -s "$t" ] && ! grep -qF carpcontrol.sh "$t" &&
	chown root:wheel "$t" && chmod 0644 "$t" && mv "$t" "$f"; then
	exit 0
fi
rm -f "$t"
exit 1`

// carpVIP and carpVhid identify the storage VIP in the rc.conf alias line.
// carpCompatVIP is the old 192.168.1.138, kept under the same vhid as
// ifconfig_re0_alias1 after the 824 storage migration and retired in task
// i24; RcConf now only removes that line.
const (
	carpVIP       = "10.20.0.138/32"
	carpCompatVIP = "192.168.1.138/32"
	carpVhid      = "1"
)

// CarpPassSecret is the logical secret reference of the vhid 1 password.
// cmd/gonf maps it to the vault entry Infra/carp-vhid1-pass. It was first
// imported from the live rc.conf (task gk2), then rotated on 2026-09-25
// (task 1l2) because the old value was published in the blog: the new key
// was set live on both hosts in the same second with ifconfig; RcConf
// only brought rc.conf in line for the next boot.
func CarpPassSecret() string {
	return paths.FHostSecret("carp/vhid1.pass")
}

// DescScript returns the description for the carp CLI.
func (Carp) DescScript() string {
	return "Install " + carpScript + " on the CARP members f0/f1 (0755 root:wheel)"
}

// WhenScript narrows the task to the CARP members.
func (Carp) WhenScript() TaskOption { return WhenHostnameIn(carpMembers...) }

// Script installs the carp state/management CLI on f0 and f1.
func (Carp) Script() {
	InstallFile(carpScript, paths.FHostAsset("carp/carp"), RootExec)
}

// DescFailbackScript returns the description for the failback script.
func (Carp) DescFailbackScript() string {
	return "Install " + carpFailbackScript + " on f0 (0755 root:wheel)"
}

// OptsFailbackScript records the carp CLI first: the failback script calls
// "carp state" and "carp master".
func (Carp) OptsFailbackScript() TaskOptions {
	return TaskOptions{Needs(Carp.Script)}
}

// WhenFailbackScript narrows the task to the failback host.
func (Carp) WhenFailbackScript() TaskOption { return WhenHostnameIn(carpFailbackHost) }

// FailbackScript installs the failback script on f0.
func (Carp) FailbackScript() {
	InstallFile(carpFailbackScript,
		paths.FHostAsset("carp/carp-auto-failback.sh"), RootExec)
}

// DescFailbackCron returns the description for the failback cron job.
func (Carp) DescFailbackCron() string {
	return "Root cron on f0: carp-auto-failback.sh every minute"
}

// OptsFailbackCron records the script before the job.
func (Carp) OptsFailbackCron() TaskOptions {
	return TaskOptions{Needs(Carp.FailbackScript)}
}

// WhenFailbackNewsyslog narrows the task to the failback host.
func (Carp) WhenFailbackNewsyslog() TaskOption { return WhenHostnameIn(carpFailbackHost) }

// FailbackNewsyslog rotates /var/log/carp-auto-failback.log on f0 via
// /etc/newsyslog.conf.
func (Carp) FailbackNewsyslog() {
	File("/etc/newsyslog.conf", WithLine(carpFailbackNewsyslogLine), WithMode(0o644))
}

// WhenFailbackCron narrows the task to the failback host.
func (Carp) WhenFailbackCron() TaskOption { return WhenHostnameIn(carpFailbackHost) }

// FailbackCron runs the failback check every minute on f0. The first deploy
// on 2026-09-25 did NOT adopt the identical hand-added line (gonf bug, tracked
// as its own task) and the job ran twice until the unmanaged line was removed
// by hand; on a freshly built f0 there is no such line to collide with.
func (Carp) FailbackCron() {
	CronAt("carp-auto-failback", "* * * * *", carpFailbackScript)
}

// DescControl returns the description for carpcontrol.sh.
func (Carp) DescControl() string {
	return "Install " + carpControlScript + " (the devd CARP hook) on f0/f1 (0555 root:wheel); remove the old hand-made copies"
}

// WhenControl narrows the task to the CARP members.
func (Carp) WhenControl() TaskOption { return WhenHostnameIn(carpMembers...) }

// Control installs carpcontrol.sh with the live mode 0555 and removes the
// two stale copies beside it. The script only runs on a CARP transition
// (devd), so replacing it never runs it.
func (Carp) Control() {
	InstallFile(carpControlScript, paths.FHostAsset("carp/carpcontrol.sh"), Perm(0o555, Root))
	for _, old := range carpControlOldCopies {
		NoFile(old)
	}
}

// DescDevdHook returns the description for the devd CARP rule.
func (Carp) DescDevdHook() string {
	return "devd rule " + carpDevdHook + " calling carpcontrol.sh on CARP MASTER/BACKUP on f0/f1; drop the old copy from /etc/devd.conf; restart devd on change"
}

// OptsDevdHook records carpcontrol.sh first: the rule runs it.
func (Carp) OptsDevdHook() TaskOptions {
	return TaskOptions{Privileged(), Needs(Carp.Control)}
}

// WhenDevdHook narrows the task to the CARP members.
func (Carp) WhenDevdHook() TaskOption { return WhenHostnameIn(carpMembers...) }

// DevdHook moves the CARP rule from the end of /etc/devd.conf (appended by
// hand, blog part 6) into the drop-in carp.conf in /usr/local/etc/devd, a
// directory the stock devd.conf already includes. The drop-in is installed
// first, then the old block is removed (only while it is there), and devd
// restarts once if either changed, so the rule is never loaded twice nor
// missing from a running devd. Keeping /etc/devd.conf stock also keeps
// etcupdate merges clean.
func (Carp) DevdHook() {
	EnsureDir(carpDevdDir, RootOwned)
	hook := InstallFile(carpDevdHook, paths.FHostAsset("carp/devd-carp.conf"), RootOwned)
	strip := Command("sh", List("-c", devdStripCarpBlock),
		OnlyIf("grep", List("-qF", carpControlScript, devdConf)),
		DependsOn(hook), WithName("devd-conf-strip-carp"))
	Sh("service devd restart", OnChange(hook, strip))
}

// DescLoaderConf returns the description for carp_load.
func (Carp) DescLoaderConf() string {
	return `loader.conf carp_load="YES" on f0/f1 (next boot)`
}

// WhenLoaderConf narrows the task to the CARP members.
func (Carp) WhenLoaderConf() TaskOption { return WhenHostnameIn(carpMembers...) }

// LoaderConf owns the carp_load line of /boot/loader.conf on the CARP
// members; Loader owns the other lines of the file. The module is already
// loaded on the running hosts, so nothing is kldloaded here.
func (Carp) LoaderConf() {
	File(loaderConf, WithShellVar("carp_load", "YES"),
		RootOwned, WithName("loader-conf-carp"))
}

// DescRcConf returns the description for the CARP alias line.
func (Carp) DescRcConf() string {
	return "rc.conf ifconfig_<if>_alias0 (CARP vhid 1, VIP 10.20.0.138, per-host interface and advskew, password from the vault) on f0/f1, without the other interface's line or the retired 192.168.1.138 alias1; next boot, no netif restart"
}

// RcConf owns the rc.conf line that creates the CARP VIP at boot. The
// password is the same on both members and must be a plain word of at most
// 19 bytes: the kernel key is CARP_KEY_LEN (20) bytes, but ifconfig copies
// it with strlcpy(..., CARP_KEY_LEN), silently cutting longer input to 19;
// and it sits unquoted inside the rc.conf value.
// gonf keeps the op carrying it off stdout. The line is never applied live:
// see the Carp comment.
//
// The retired compatibility line (old VIP 192.168.1.138 under the same vhid,
// task i24) is removed by its exact rendered content: both members must drop
// it together with the runtime address, because CARP authenticates the
// vhid's whole address set and members with different sets both go MASTER.
// The runtime removal is an operator procedure with the BACKUP fenced
// first (IQ topics/infra/openbsdrouter-plan/RETIREMENT-I24.md), not this task.
//
// The same line on the other interface (re0 or vm-public) is removed by its
// exact content too: two alias0 lines would create the vhid twice at boot.
// Moving the line between interfaces is BridgeMigration's job; this task
// only follows CarpNode.Interface once the move is confirmed. A guard
// refuses the write while a bridge move is pending or while rc.conf still
// carries the other interface's alias0 key, i.e. while the inventory and
// the host disagree: rewriting the live line back would put the VIP on an
// interface without the lab address at the next boot (split brain).
func (Carp) RcConf() {
	pass := MustSecret(CarpPassSecret())
	EachHostWith(func(n CarpNode) {
		other, ok := carpOtherInterface(n.Interface)
		if !ok {
			Refuse("Carp", rcConf, fmt.Errorf("CarpNode.Interface %q is neither %s nor %s", n.Interface, labMember, LabBridge))
			return
		}
		line := carpAliasLine(n.Interface, n, pass)
		compat := strings.Replace(carpAliasLine(labMember, n, pass), "alias0=", "alias1=", 1)
		compat = strings.ReplaceAll(compat, carpVIP, carpCompatVIP)
		guard := Command("sh", List("-c", `test ! -d `+bridgeState+`/pending && ! grep -q '^`+carpAliasKey(other)+`=' `+rcConf+
			` || { echo 'bridge move pending, or CarpNode.Interface not switched to match rc.conf' >&2; exit 1; }`),
			WithName("rc-conf-carp-interface-guard"))
		File(rcConf, WithKeyedLine(carpAliasKey(n.Interface)+"=", line),
			WithoutLine(carpAliasLine(other, n, pass)),
			WithoutLine(compat),
			RootOwned, WithName("rc-conf-carp"), DependsOn(guard))
	})
}

// WhenBootGate narrows the task to the CARP members.
func (Carp) WhenBootGate() TaskOption { return WhenHostnameIn(carpMembers...) }

// BootGate holds CARP at boot until re0 carries the peer's advertisements, so
// a booting member never promotes itself on a dead wire (task 764).
//
// On 2026-11-12 f1 booted with the VIP on the bridge vm-public. The bridge
// stays up while any member is, and when vm-bhyve added the guest's tap0,
// if_bridge muted TXCSUM/TSO on re0, whose link then reset for ~4 s. f1
// heard nothing for 3 x advbase, went MASTER for a second and its
// announcement taught the switch the CARP MAC on f1's port (f0's adverts
// carried re0's own MAC: ether_output hands a bridge member's frames to
// if_bridge before carp_output can set the virtual source MAC).
//
// The gate (f3s/freebsd-hosts/carp/carp-gate): loader.conf keeps CARP off
// from module load; the rc.d hook, before netif, pre-mutes TXCSUM/TSO on re0
// so no tap join or leave resets it, and detaches a waiter that sets
// net.inet.carp.allow=1 after the guest tap joined, re0 stayed active for
// carpGateSettle seconds and one vhid advertisement arrived on re0, or at
// carpGateDeadline. re0 is watched in both layouts: it is the physical
// uplink whether the VIP sits on it or on the bridge (CarpNode.Interface).
// The loader line is written last, so the hold never exists without its
// waiter; to remove the gate, drop that line first. Next boot only.
func (Carp) BootGate() {
	EnsureDir("/usr/local/libexec", RootOwned)
	EnsureDir("/usr/local/etc", RootOwned)
	conf := File(carpGateConf, WithContent(fmt.Sprintf("uplink='%s'\nvhid='%s'\nsettle='%d'\ndeadline='%d'\n",
		labMember, carpVhid, carpGateSettle, carpGateDeadline)), RootOwned)
	helper := InstallFile(carpGateHelper, paths.FHostAsset("carp/carp-gate"), RootExec, DependsOn(conf))
	hook := InstallFile(carpGateRC, paths.FHostAsset("carp/rc-carp-gate"), RootExec, DependsOn(helper))
	File(loaderConf, WithKeyedLine("net.inet.carp.allow=", carpGateLoaderLine),
		RootOwned, WithName("loader-conf-carp-gate"), DependsOn(hook))
}

// carpOtherInterface returns the interface that must not carry the line.
func carpOtherInterface(iface string) (string, bool) {
	switch iface {
	case labMember:
		return LabBridge, true
	case LabBridge:
		return labMember, true
	}
	return "", false
}

// carpAliasKey returns the rc.conf key of iface's alias0. network.subr
// reads ifconfig_vm_public_* for vm-public: it maps ".-/+" to "_".
func carpAliasKey(iface string) string {
	return "ifconfig_" + strings.NewReplacer(".", "_", "-", "_", "/", "_", "+", "_").Replace(iface) + "_alias0"
}

// carpAliasLine renders iface's alias0 line in the live spelling:
// "inet vhid 1 [advskew N ]pass P alias 10.20.0.138/32".
func carpAliasLine(iface string, n CarpNode, pass string) string {
	skew := ""
	if n.AdvSkew != 0 {
		skew = " advskew " + strconv.Itoa(n.AdvSkew)
	}
	return carpAliasKey(iface) + `="inet vhid ` + carpVhid + skew + ` pass ` + pass + ` alias ` + carpVIP + `"`
}
