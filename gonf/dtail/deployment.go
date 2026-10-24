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

// Deployment hardens existing package-managed dserver installations.
type Deployment struct{ RequiresRoot }

// Config installs the protected key cache and host-specific log access.
func (Deployment) Config() {
	EachHost(func(host ServerHost) { Install(host) })
}

// Install declares the OS-specific dserver files after optional prerequisites.
// Existing daemon-owned runtime trees fail closed: migrate them using the
// approved runbook before applying. No private host key is generated or copied.
func Install(host ServerHost, dependencies ...Dependency) {
	content, err := render(host)
	if err != nil {
		Refuse("dserver", host.BindAddress, err)
		return
	}
	script := "scripts/dserver-update-key-cache.sh.tpl"
	rcSource := "etc/rc.d/dserver.tpl"
	configPath := "/etc/dserver/dtail.json"
	rcPath := "/etc/rc.d/dserver"
	switch host.Platform {
	case "freebsd":
		script = "scripts/dserver-update-key-cache-freebsd.sh.tpl"
		rcSource = "etc/rc.d/dserver-freebsd.tpl"
		configPath = "/usr/local/etc/dserver/dtail.json"
		rcPath = "/usr/local/etc/rc.d/dserver"
	case "netbsd":
		script = "scripts/dserver-update-key-cache-netbsd.sh.tpl"
		rcSource = "etc/rc.d/dserver-netbsd.tpl"
	}
	scriptPath := paths.FrontendAsset(script)
	if host.Platform == "linux" {
		scriptPath = paths.PackageAsset("dtail-rocky/dserver-update-key-cache.sh")
	}
	helper := InstallFile("/usr/local/bin/dserver-update-key-cache.sh", scriptPath, RootExec, DependsOn(dependencies...))
	cache := Sh("/usr/local/bin/dserver-update-key-cache.sh", DependsOn(helper), WithName("refresh-protected-dserver-cache"))
	ready := cache
	if host.Platform == "linux" {
		// A missing volatile key needs an explicit recovery/provisioning decision.
		ready = Command("test", List("-s", "/var/lib/dserver/ssh_host_key"), DependsOn(cache), WithName("require-preserved-dserver-host-key"))
	}
	config := File(configPath, WithContent(content), RootOwned, DependsOn(ready))
	if host.Platform == "linux" {
		unit := InstallFile("/etc/systemd/system/dserver.service", paths.PackageAsset("dtail-rocky/dserver.service"), RootOwned, DependsOn(ready))
		reload := Sh("systemctl daemon-reload", OnChange(unit))
		Service("dserver", WithRestart, DependsOn(ready, reload), OnChange(config, helper, unit))
		return
	}
	rc := InstallFile(rcPath, paths.FrontendAsset(rcSource), RootExec, DependsOn(cache))
	Service("dserver", WithRestart, DependsOn(cache), OnChange(config, helper, rc))
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
