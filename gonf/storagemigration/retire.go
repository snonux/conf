// Package storagemigration retires the storage-migration client helper's host
// artifacts (task 874, item 4 of IQ openbsdrouter-plan/CLEANUP.md). The
// helper moved the Linux NFS-over-TLS clients (r0-r2 and earth) from the old
// storage VIP 192.168.1.138 to 10.20.0.138; every client accepted the move
// on 2026-11-07, and the managed stunnel.conf (rnodes_nfs_client_stunnel,
// system_nfs_client_stunnel) carries the new endpoint.
//
// One-shot: once r0-r2 and earth ran their retire task, a later commit
// deletes this package and its registrations.
package storagemigration

import (
	. "github.com/snonux/gonf/api"
)

// ClientRetirement removes the client helper's watchdog timer and service,
// the stunnel drop-in that ran the helper before and after every stunnel
// start, the helper and its transaction journals. Run by name, one host at
// a time: it is Operational, so no aggregate picks it up.
type ClientRetirement struct{ RequiresRoot }

const (
	clientHelper     = "/usr/local/sbin/gonf-storage-client"
	clientState      = "/var/lib/gonf-storage-client"
	clientTimer      = "gonf-storage-client-rollback.timer"
	stunnelDropInDir = "/etc/systemd/system/stunnel.service.d"
	stunnelDropIn    = stunnelDropInDir + "/50-gonf-storage-rollback.conf"
)

func (ClientRetirement) OptsRetire() TaskOptions { return TaskOptions{Operational()} }

// Retire removes the storage-client watchdog, stunnel drop-in and helper.
func (ClientRetirement) Retire() {
	// Refuse while a transaction is pending: its rollback still needs the
	// helper. Skipped (no change) when nothing is pending.
	pending := Command("sh", List("-c", "echo 'pending storage-client transaction' >&2; exit 1"),
		OnlyIf("test", List("-e", clientState+"/pending")), WithName("storage-client-idle"))
	reloaded := retireClientUnits(pending)
	// The running stunnel keeps its tunnel: dropping ExecStartPre/Post only
	// changes what its next start runs. Belt and braces before the helper
	// goes: a stale ExecStartPre on a missing helper would fail that start.
	// The guard makes it a refusal that is skipped (no change) once clean.
	stale := `systemctl show -p ExecStartPre -p ExecStartPost --value stunnel.service | grep -q gonf-storage-client`
	check := Command("sh", List("-c", "echo 'stunnel.service still runs gonf-storage-client' >&2; exit 1"),
		OnlyIf("sh", List("-c", stale)), DependsOn(reloaded), WithName("stunnel-without-storage-client"))
	NoFile(clientHelper, DependsOn(check))
	NoDir(clientState, WithPrune, DependsOn(check))
}

// retireClientUnits stops and disables the timer (which drops its
// timers.target.wants link), deletes the unit files and the stunnel drop-in,
// removes the drop-in directory when it is left empty, and reloads systemd.
// The returned resource is the reload, so callers order after all of it.
func retireClientUnits(pending Resource) Resource {
	disable := Command("systemctl", List("disable", "--now", "--quiet", clientTimer),
		OnlyIf("test", List("-e", "/etc/systemd/system/"+clientTimer)), DependsOn(pending))
	// The service is static (only the timer starts it); nothing else
	// Requires or Wants either unit.
	removed := NoFile(List("/etc/systemd/system/"+clientTimer,
		"/etc/systemd/system/gonf-storage-client-rollback.service", stunnelDropIn), DependsOn(disable))
	emptyDir := `[ -d "$1" ] && [ -z "$(ls -A "$1")" ]`
	rmdir := Command("rmdir", List(stunnelDropInDir),
		OnlyIf("sh", List("-c", emptyDir, "empty", stunnelDropInDir)), DependsOn(removed))
	return Sh("systemctl daemon-reload", OnChange(disable, removed, rmdir))
}
