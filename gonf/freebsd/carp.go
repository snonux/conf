package freebsd

import (
	"strconv"

	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// Carp manages the CARP layer of the storage pair f0/f1 (the f3s-storage-ha
// VIP 192.168.1.138, see the f3s-storage skill, references/carp.md):
//   - /usr/local/bin/carp, the state/management CLI, on both;
//   - on f0 only, the minutely carp-auto-failback.sh that reclaims MASTER
//     after f0 comes back, with its cron job and log rotation;
//   - carpcontrol.sh, which starts/stops the NFS server and stunnel on a
//     state change, and the devd rule that calls it (task gk2);
//   - carp_load in /boot/loader.conf and the rc.conf CARP alias line
//     ifconfig_re0_alias0, whose password comes from the vault (task gk2).
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
type CarpNode struct {
	AdvSkew int
}

const (
	carpScript         = "/usr/local/bin/carp"
	carpFailbackScript = "/usr/local/bin/carp-auto-failback.sh"
	carpControlScript  = "/usr/local/bin/carpcontrol.sh"
	carpDevdDir        = "/usr/local/etc/devd"
	carpDevdHook       = carpDevdDir + "/carp.conf"
	devdConf           = "/etc/devd.conf"
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
const (
	carpVIP  = "192.168.1.138/32"
	carpVhid = "1"
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

// Script installs the carp state/management CLI on f0 and f1.
func (Carp) Script() {
	WhenHostname(carpMembers, func() {
		InstallFile(carpScript, paths.FHostAsset("carp/carp"), Perm(0o755, Root))
	})
}

// DescFailbackScript returns the description for the failback script.
func (Carp) DescFailbackScript() string {
	return "Install " + carpFailbackScript + " on f0 (0755 root:wheel)"
}

// OptsFailbackScript records the carp CLI first: the failback script calls
// "carp state" and "carp master".
func (Carp) OptsFailbackScript() TaskOptions {
	return TaskOptions{Needs("script")}
}

// FailbackScript installs the failback script on f0.
func (Carp) FailbackScript() {
	WhenHostname(carpFailbackHost, func() {
		InstallFile(carpFailbackScript,
			paths.FHostAsset("carp/carp-auto-failback.sh"), Perm(0o755, Root))
	})
}

// DescFailbackCron returns the description for the failback cron job.
func (Carp) DescFailbackCron() string {
	return "Root cron on f0: carp-auto-failback.sh every minute"
}

// OptsFailbackCron records the script before the job.
func (Carp) OptsFailbackCron() TaskOptions {
	return TaskOptions{Needs("failback_script")}
}

// DescFailbackNewsyslog returns the description for the log rotation line.
func (Carp) DescFailbackNewsyslog() string {
	return "Rotate /var/log/carp-auto-failback.log on f0 via /etc/newsyslog.conf"
}

// FailbackNewsyslog rotates the failback log: while f0 sits in INIT or is
// blocked from failing back, the script appends a SKIP line every minute.
func (Carp) FailbackNewsyslog() {
	WhenHostname(carpFailbackHost, func() {
		File("/etc/newsyslog.conf", WithLine(carpFailbackNewsyslogLine), WithMode(0o644))
	})
}

// FailbackCron runs the failback check every minute on f0. The first deploy
// on 2026-09-25 did NOT adopt the identical hand-added line (gonf bug, tracked
// as its own task) and the job ran twice until the unmanaged line was removed
// by hand; on a freshly built f0 there is no such line to collide with.
func (Carp) FailbackCron() {
	WhenHostname(carpFailbackHost, func() {
		CronAt("carp-auto-failback", "* * * * *", carpFailbackScript)
	})
}

// DescControl returns the description for carpcontrol.sh.
func (Carp) DescControl() string {
	return "Install " + carpControlScript + " (the devd CARP hook) on f0/f1 (0555 root:wheel); remove the old hand-made copies"
}

// Control installs carpcontrol.sh with the live mode 0555 and removes the
// two stale copies beside it. The script only runs on a CARP transition
// (devd), so replacing it never runs it.
func (Carp) Control() {
	WhenHostname(carpMembers, func() {
		InstallFile(carpControlScript, paths.FHostAsset("carp/carpcontrol.sh"), Perm(0o555, Root))
		for _, old := range carpControlOldCopies {
			NoFile(old)
		}
	})
}

// DescDevdHook returns the description for the devd CARP rule.
func (Carp) DescDevdHook() string {
	return "devd rule " + carpDevdHook + " calling carpcontrol.sh on CARP MASTER/BACKUP on f0/f1; drop the old copy from /etc/devd.conf; restart devd on change"
}

// OptsDevdHook records carpcontrol.sh first: the rule runs it.
func (Carp) OptsDevdHook() TaskOptions {
	return TaskOptions{Privileged(), Needs("control")}
}

// DevdHook moves the CARP rule from the end of /etc/devd.conf (appended by
// hand, blog part 6) into the drop-in carp.conf in /usr/local/etc/devd, a
// directory the stock devd.conf already includes. The drop-in is installed
// first, then the old block is removed (only while it is there), and devd
// restarts once if either changed, so the rule is never loaded twice nor
// missing from a running devd. Keeping /etc/devd.conf stock also keeps
// etcupdate merges clean.
func (Carp) DevdHook() {
	WhenHostname(carpMembers, func() {
		EnsureDir(carpDevdDir, Perm(0o755, Root))
		hook := InstallFile(carpDevdHook, paths.FHostAsset("carp/devd-carp.conf"), Perm(0o644, Root))
		strip := Command("sh", List("-c", devdStripCarpBlock),
			OnlyIf("grep", List("-qF", carpControlScript, devdConf)),
			DependsOn(hook), WithName("devd-conf-strip-carp"))
		Sh("service devd restart", OnChange(hook, strip))
	})
}

// DescLoaderConf returns the description for carp_load.
func (Carp) DescLoaderConf() string {
	return `loader.conf carp_load="YES" on f0/f1 (next boot)`
}

// LoaderConf owns the carp_load line of /boot/loader.conf on the CARP
// members; Loader owns the other lines of the file. The module is already
// loaded on the running hosts, so nothing is kldloaded here.
func (Carp) LoaderConf() {
	WhenHostname(carpMembers, func() {
		File(loaderConf, WithKeyedLine(`carp_load=`, `carp_load="YES"`),
			Perm(0o644, Root), WithName("loader-conf-carp"))
	})
}

// DescRcConf returns the description for the CARP alias line.
func (Carp) DescRcConf() string {
	return "rc.conf ifconfig_re0_alias0 (CARP vhid 1, VIP 192.168.1.138, per-host advskew, password from the vault) on f0/f1; next boot, no netif restart"
}

// RcConf owns the rc.conf line that creates the CARP VIP at boot. The
// password is the same on both members and must be a plain word of at most
// 19 bytes: the kernel key is CARP_KEY_LEN (20) bytes, but ifconfig copies
// it with strlcpy(..., CARP_KEY_LEN), silently cutting longer input to 19;
// and it sits unquoted inside the rc.conf value.
// gonf keeps the op carrying it off stdout. The line is never applied live:
// see the Carp comment.
func (Carp) RcConf() {
	pass := MustSecret(CarpPassSecret())
	for _, host := range carpMembers {
		WhenHostname(host, func() {
			File(rcConf, WithKeyedLine(`ifconfig_re0_alias0=`, carpAliasLine(HostData[CarpNode](host), pass)),
				Perm(0o644, Root), WithName("rc-conf-carp"))
		})
	}
}

// carpAliasLine renders ifconfig_re0_alias0 in the live spelling:
// "inet vhid 1 [advskew N ]pass P alias 192.168.1.138/32".
func carpAliasLine(n CarpNode, pass string) string {
	skew := ""
	if n.AdvSkew != 0 {
		skew = " advskew " + strconv.Itoa(n.AdvSkew)
	}
	return `ifconfig_re0_alias0="inet vhid ` + carpVhid + skew + ` pass ` + pass + ` alias ` + carpVIP + `"`
}
