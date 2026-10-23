package frontends

import . "github.com/snonux/gonf/api"

const accessLogRotation = `/var/www/logs/access.log	root:_dserver	640  7    102400    $W0   Z "pkill -USR1 -u root -U root -x httpd"`

// LowRisk restricts monitoring and access-log files.
// Serialize the mute-path cutover with f3sctl power operations. This task does
// not alter SSH policy or restart daemons; retire NRPE separately after checks.
func (Monitoring) LowRisk() {
	EachHost(func(server Server) {
		mute := installMuteControl()
		File("/etc/gogios.json", WithContent(renderGogios(server)),
			Perm(0o640, "root:_gogios"), DependsOn(mute))
	})
	File("/etc/newsyslog.conf", WithKeyedLine("/var/www/logs/access.log", accessLogRotation), RootOwned)
	EnsureFile("/var/www/logs/access.log", Perm(0o640, "root:_dserver"))
}

// OptsRetireNRPE keeps the one-time package retirement out of aggregates.
func (Monitoring) OptsRetireNRPE() TaskOptions { return TaskOptions{Operational()} }

// RetireNRPE removes NRPE after confirming it is unused and saving its package.
func (Monitoring) RetireNRPE() { NoPackage("nrpe") }

// installMuteControl keeps the only writable marker directory away from /tmp.
// f3sctl can write; Gogios can traverse and inspect, but cannot create a mute.
// Existing f3sctl SSH access is a prerequisite, never configured by this task.
func installMuteControl(dependencies ...Dependency) Resource {
	EnsureDir("/var/db/f3s", RootOwned)
	dir := Dir("/var/db/f3s/monitoring", Perm(0o750, "f3sctl:_gogios"), DependsOn(dependencies...))
	helper := InstallFile("/usr/local/bin/f3s-gogios-mute", legacyFrontendAsset("scripts/f3s-gogios-mute"),
		Perm(0o555, Root), DependsOn(dir))
	rc := ensureRCLocal()
	File("/etc/rc.local", WithLine("rm -f "+f3sTakenDown), RootOwned, DependsOn(rc, dir))
	return helper
}
