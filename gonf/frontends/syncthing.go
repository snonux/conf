package frontends

import (
	"fmt"
	"strings"

	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/paths"
)

// check_syncthing (tools/check_syncthing) reports out-of-sync folders of the
// f3s Syncthing instance through its REST API; see its README for the exit
// codes and the ignore-file format.
const (
	syncthingPluginPath = "/usr/local/bin/check_syncthing"
	syncthingConfDir    = "/etc/check_syncthing"
	syncthingKeyFile    = syncthingConfDir + "/api.key"
	syncthingIgnoreFile = syncthingConfDir + "/ignore"
	// syncthingStateDir keeps when each folder first went out of sync, for
	// the WARNING-to-CRITICAL escalation. It is under /var/db, not
	// /var/run, so the durations survive a frontend reboot.
	syncthingStateDir = "/var/db/check_syncthing"
	syncthingURL      = "https://syncthing.f3s.buetow.org"
)

// syncthingIgnore lists known out-of-sync folders that must not alert.
// Galaxy Tab S10 FE DCIM has 4 pending files whose source device is
// disconnected (f3s/syncthing/README.md); the limits make it alert again if
// that backlog grows.
const syncthingIgnore = `# check_syncthing ignore list, managed by gonf (frontends_gogios).
# <folder ID or label>[; max-items=N][; max-bytes=N]
Galaxy Tab S10 FE DCIM; max-items=4; max-bytes=20226448
`

// syncthingPlugin is the controller-built OpenBSD binary of check_syncthing.
// It is not committed: build it with `mage buildOpenBSD` in
// tools/check_syncthing before applying frontends_gogios. A missing binary
// refuses the recording with that hint, like check_shuriken_age's source.
func syncthingPlugin() string {
	return RequireControllerFile(paths.ToolAsset("check_syncthing/check_syncthing.openbsd-amd64"),
		"frontends_gogios (check_syncthing plugin)",
		"run `mage buildOpenBSD` in tools/check_syncthing")
}

// installSyncthingPlugin installs check_syncthing with its API key, ignore
// list and state directory, all readable by _gogios only. It returns the
// resources the Gogios cron jobs wait for, or nil after refusing a
// malformed key.
func installSyncthingPlugin(source string, account Resource) []Dependency {
	key := strings.TrimSpace(MustSecret(paths.FrontendSecret("etc/check_syncthing/api.key")))
	if key == "" || strings.ContainsAny(key, " \t\r\n\x00") {
		Refuse("File", syncthingKeyFile, fmt.Errorf("Syncthing API key must be a nonempty single token"))
		return nil
	}
	plugin := InstallFile(syncthingPluginPath, source, RootExec)
	Dir(syncthingConfDir, Perm(0o750, "root:_gogios"), DependsOn(account))
	keyFile := File(syncthingKeyFile, WithContent(key+"\n"), Perm(0o640, "root:_gogios"), WithSensitive)
	ignore := File(syncthingIgnoreFile, WithContent(syncthingIgnore), Perm(0o640, "root:_gogios"))
	state := Dir(syncthingStateDir, Perm(0o750, "_gogios:_gogios"), DependsOn(account))
	return []Dependency{plugin, keyFile, ignore, state}
}

// addSyncthingChecks adds "Check Syncthing", which pages on stuck folders
// and on transfers that stay out of sync for over an hour. It runs every
// 30 minutes, off the */5 grid, because /rest/db/status is expensive; the
// 8s timeout stays under CheckTimeoutS (10s). It pauses while f3s is taken
// down and depends on the Syncthing HTTPS checks, so an unreachable cluster
// reports there instead of here.
func addSyncthingChecks(checks map[string]gogiosCheck) {
	host := strings.TrimPrefix(syncthingURL, "https://")
	checks["Check Syncthing"] = gogiosCheck{
		Plugin: syncthingPluginPath,
		Args: List("-url", syncthingURL, "-api-key-file", syncthingKeyFile,
			"-ignore-file", syncthingIgnoreFile, "-state-dir", syncthingStateDir,
			"-escalate-after", "1h", "-timeout", "8s"),
		OnlyIfNotExists: f3sTakenDown,
		RandomSpread:    10,
		RunInterval:     1800,
		DependsOn:       List("Check HTTPS IPv4 "+host, "Check HTTPS IPv6 "+host),
	}
}
