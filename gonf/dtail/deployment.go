// Package dtail configures the fleet's existing dserver installations.
package dtail

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

// ServerHost is dserver's per-host inventory. BindAddress is a concrete LAN or
// WireGuard address; Platform selects package paths, never SSH policy.
type ServerHost struct {
	BindAddress string
	Platform    string
}

// keyCacheHelper is the installed key-cache refresh script on every platform.
const keyCacheHelper = "/usr/local/bin/dserver-update-key-cache.sh"

// Deployment hardens existing package-managed dserver installations.
type Deployment struct{ RequiresRoot }

// Config installs the protected key cache and host-specific log access.
func (Deployment) Config() {
	EachHost(func(host ServerHost) { Install(host) })
}

// Install declares the OS-specific dserver files after optional prerequisites.
// Existing daemon-owned runtime trees fail closed: migrate them using the
// approved runbook before applying. No private host key is generated or copied.
//
// The key-cache refresh (and on Linux the host-key check) is a gate in front
// of the dserver restart, not a step of every run: it runs only when the
// helper, the config or the systemd unit / rc.d script changed, after they
// are written and before the restart they trigger. Unchanged hosts preview
// with 0 would-change; the cache stays fresh through the service's own
// refresh at every start (ExecStartPre / start_precmd) plus a periodic one:
// the RPM's dserver-update-keycache.timer on Rocky, daily.local on OpenBSD
// (frontends.Monitoring), and on the NetBSD Pis and FreeBSD f-hosts a
// hand-installed root crontab line and periodic/daily job (live 2026-11-13,
// not managed here). Gonf is not run on a schedule, so the old every-run
// refresh never kept the cache fresh between applies either.
//
// A gate that fails (unsafe runtime tree, missing host key) fails the apply
// before the restart (an apply stops at its first failing op), so the
// running daemon keeps its old configuration. The new files are already in
// place then, so the next apply sees no change and does not retry the
// restart: after fixing the cause, restart dserver by hand (its start runs
// the same checks).
func Install(host ServerHost, dependencies ...Dependency) {
	content, err := render(host)
	if err != nil {
		Refuse("dserver", host.BindAddress, err)
		return
	}
	if host.Platform == "linux" {
		installLinux(content, dependencies)
		return
	}
	installBSD(host.Platform, content, dependencies)
}

func render(host ServerHost) (string, error) {
	ip := net.ParseIP(host.BindAddress)
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() {
		return "", fmt.Errorf("dserver requires a concrete LAN or WireGuard address")
	}
	switch host.Platform {
	case "openbsd", "netbsd", "freebsd", "linux":
	default:
		return "", fmt.Errorf("unsupported dserver platform %q", host.Platform)
	}
	raw, err := os.ReadFile(paths.GonfAsset("frontends", "dtail.json"))
	if err != nil {
		return "", fmt.Errorf("read dserver template: %w", err)
	}
	var config struct {
		Client json.RawMessage
		Server map[string]any
		Common map[string]any
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return "", fmt.Errorf("parse dserver template: %w", err)
	}
	config.Server["SSHBindAddress"] = host.BindAddress
	if host.Platform == "linux" {
		config.Server["HostKeyFile"] = "/var/lib/dserver/ssh_host_key"
	}
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode dserver configuration: %w", err)
	}
	return string(encoded) + "\n", nil
}

// installLinux installs the Rocky files. The helper source is the RPM's copy
// (packages/files/dtail-rocky), whose #!/usr/bin/sh is what rpmbuild's
// brp-mangle-shebangs makes of #!/bin/sh anyway, so the installed RPM file
// and this InstallFile agree and no run rewrites it.
func installLinux(content string, dependencies []Dependency) {
	helper := InstallFile(keyCacheHelper, paths.PackageAsset("dtail-rocky/dserver-update-key-cache.sh"),
		RootExec, DependsOn(dependencies...))
	config := File("/etc/dserver/dtail.json", WithContent(content), RootOwned, DependsOn(helper))
	unit := InstallFile("/etc/systemd/system/dserver.service", paths.PackageAsset("dtail-rocky/dserver.service"),
		RootOwned, DependsOn(helper))
	reload := Sh("systemctl daemon-reload", OnChange(unit))
	cache := refreshKeyCache(helper, config, unit)
	// A missing volatile key needs an explicit recovery/provisioning decision;
	// dserver.service's ExecStartPre would refuse to start without it.
	ready := Command("test", List("-s", "/var/lib/dserver/ssh_host_key"),
		OnChange(helper, config, unit), DependsOn(cache), WithName("require-preserved-dserver-host-key"))
	Service("dserver", WithRestart, DependsOn(ready, reload), OnChange(config, helper, unit))
}

// installBSD installs the OpenBSD, FreeBSD and NetBSD files from the frontend
// templates; their rc.d scripts refresh the cache in start_precmd.
func installBSD(platform, content string, dependencies []Dependency) {
	script := "scripts/dserver-update-key-cache.sh.tpl"
	rcSource := "etc/rc.d/dserver.tpl"
	configPath := "/etc/dserver/dtail.json"
	rcPath := "/etc/rc.d/dserver"
	switch platform {
	case "freebsd":
		script = "scripts/dserver-update-key-cache-freebsd.sh.tpl"
		rcSource = "etc/rc.d/dserver-freebsd.tpl"
		configPath = "/usr/local/etc/dserver/dtail.json"
		rcPath = "/usr/local/etc/rc.d/dserver"
	case "netbsd":
		script = "scripts/dserver-update-key-cache-netbsd.sh.tpl"
		rcSource = "etc/rc.d/dserver-netbsd.tpl"
	}
	helper := InstallFile(keyCacheHelper, paths.FrontendAsset(script), RootExec, DependsOn(dependencies...))
	config := File(configPath, WithContent(content), RootOwned, DependsOn(helper))
	rc := InstallFile(rcPath, paths.FrontendAsset(rcSource), RootExec, DependsOn(helper))
	cache := refreshKeyCache(helper, config, rc)
	Service("dserver", WithRestart, DependsOn(cache), OnChange(config, helper, rc))
}

// refreshKeyCache runs the helper once when any of watched (the files whose
// change restarts dserver) changed in this apply; see Install.
func refreshKeyCache(watched ...Dependency) Resource {
	return Sh(keyCacheHelper, OnChange(watched...), WithName("refresh-protected-dserver-cache"))
}
