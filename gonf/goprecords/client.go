// Package goprecords declares the uptimed upload client every gonf-managed
// goprecords client shares: the OpenBSD frontends (frontends.Maintenance)
// and the FreeBSD f-hosts (freebsd.Goprecords), both hourly from root's
// crontab with the output in syslog. Only the cron minute differs, so the
// token, the script, the cron command and the optional-token policy live
// here once.
package goprecords

import (
	"strings"

	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

const (
	// TokenFile is where the upload client reads the host's token as root.
	TokenFile = "/etc/goprecords-upload.token"
	// ClientScript is the installed upload client; schedules run it with
	// GOPRECORDS_HOST=<host> in the environment.
	ClientScript = "/usr/local/bin/goprecords-upload-client.sh"
	// LogTag is the syslog tag of the client's output (see CronCommand).
	LogTag = "goprecords-upload"
)

// clientAsset is the repo copy of the POSIX upload client (also mirrored in
// the goprecords repo's contrib/ and scripts/). Its PATH covers /usr/local
// (FreeBSD, OpenBSD) and /usr/pkg (NetBSD), so one copy serves every OS.
func clientAsset() string {
	return paths.FrontendAsset("scripts/goprecords-upload-client.sh")
}

// Client declares curl, the host's token and the upload script, and returns
// the script resource for the caller's schedule to depend on. Call it inside
// the EachHost body so a single-host push resolves only that host's token.
//
// tokenRef is the logical secret path (paths.FrontendSecret /
// paths.FHostSecret). ok is false when the host has no token; the caller
// then declares no schedule either.
//
// Optional-token policy (task 262). OptionalSecret distinguishes "no such
// secret" (secret.ErrNotFound) from every other failure: a locked,
// unreachable or misconfigured secret store still fails the whole plan
// record loudly (see gonf's docs/design/secrets.md), it is never read as
// "this host has no token". Given a genuine not-found, three policies were
// considered for a host's already-deployed token file and schedule:
//   - keep (chosen): declare nothing further for this host and leave
//     whatever is already on the destination exactly as it is. Matches P9's
//     "no automatic deletion of legacy copies": actively disabling or
//     removing a live token is a destination-state change of its own, which
//     needs its own controlled rotation/recovery exercise (see gonf's
//     docs/design/consumer-dsl-simplification-plan.md, P9) and explicit
//     authorization, not a side effect of a secret-provider change.
//   - disable: also strip the schedule so a stale token stops being
//     submitted while the token file stays. Rejected: it leaves secret
//     material on disk while silently changing the host's schedule.
//   - remove: also delete the token file (NoFile) and the schedule.
//     Rejected: deleting what may be the operator's only remaining copy of
//     a secret needs the controlled recovery exercise P9 calls for.
//
// curl is declared either way: the client needs it, and installing it
// touches no secret.
func Client(tokenRef string) (script Resource, ok bool) {
	Package("curl")
	token, ok := OptionalSecret(tokenRef)
	if !ok {
		// keep: see the optional-token policy above.
		return nil, false
	}
	token = strings.TrimRight(token, "\r\n")
	if token == "" {
		return nil, false
	}
	File(TokenFile, WithContent(token+"\n"), RootPrivate)
	return InstallFile(ClientScript, clientAsset(), RootExec), true
}

// CommandLine returns the bare client invocation for host: the client with
// GOPRECORDS_HOST set, which names the host's stats on the server and must
// match the name its token was issued for. CronCommand wraps it; the
// frontends also use it verbatim to remove their former /etc/daily.local
// line.
func CommandLine(host string) string {
	return "GOPRECORDS_HOST=" + host + " " + ClientScript
}

// CronCommand returns the root cron command that runs the client for host
// with its output piped into logger(1) under LogTag, so an upload failure
// (curl's error while goprecords is unreachable, e.g. during the nightly
// f3s power-off, when the frontends' relayd falls back to httpd and a PUT
// gets 405) lands in /var/log/messages instead of root's mailbox. The same
// string on FreeBSD and OpenBSD: both have /usr/bin/env and logger.
func CronCommand(host string) string {
	return "/usr/bin/env " + CommandLine(host) + " 2>&1 | logger -t " + LogTag
}
