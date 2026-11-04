// Package workstation manages privileged Fedora workstation configuration.
package workstation

import . "github.com/snonux/gonf/api"

// Register keeps workstation tasks separate from every server aggregate.
// Existing task names remain available from conf; package tasks retain their
// Fedora profile guard and system tasks retain their per-host guards.
func Register() {
	RegisterMethods(Pkg{}, WithPrefix("pkg_"), WhenProfile("fedora"))
	RegisterMethods(System{}, WithPrefix("system_"))
	AggregateTasks("workstation", "Configure Fedora workstation packages and system settings",
		"pkg_opendoas", "pkg_fish_tools", "pkg_taskwarrior", "pkg_helix", "pkg_fedora",
		"system_hosts", "system_wireguard", "system_uptimed", "system_fish_shell", "system_timezone",
	)
}
