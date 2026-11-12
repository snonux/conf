package rocky

import (
	. "github.com/snonux/gonf/api"
)

// NetworkRetirement removes the Rocky network-migration helper's host
// artifacts from pi2, pi3, r0-r2 and the standalone guest (task c74, item 1
// of IQ openbsdrouter-plan/CLEANUP.md): the 30-second watchdog timer, the
// boot and runtime restore units, the r-nodes' k3s maintenance drop-in, the
// helper, its config and its transaction journals. The NetworkManager
// profiles already carry the lab IPv4, gateway 10.20.0.1 and ULA (verified
// per host before the run); the restore units only acted on a pending
// transaction.
//
// One-shot: once every host ran rocky_network_migration_retire, a later
// commit deletes this file and its registration. It is Operational, so the
// rocky aggregate never picks it up: run it by name, one host at a time.
//
// Left alone on purpose: /etc/rancher/k3s/config.yaml.d/60-network-migration.yaml
// on r0-r2 (live k3s config, "flannel-iface: enp0s5"), and the orphan
// /usr/local/sbin/gonf-endpoint-rollback with /var/lib/gonf-endpoint, which
// wireguardmeshgenerator's endpoint tooling owns.
type NetworkRetirement struct{ RequiresRoot }

const (
	networkMigrationHelper = "/usr/local/sbin/gonf-network-migration"
	networkState           = "/var/lib/gonf-network-migration"
	networkDropIn          = "/etc/systemd/system/k3s.service.d/90-gonf-network-maintenance.conf"
)

// networkUnits are the helper's systemd units. On the r-nodes
// NetworkManager.service.requires links -boot and k3s.service.requires links
// -runtime; on the Pis docker is a Requires= of the resolver runtime unit.
var networkUnits = List("gonf-network-rollback.timer", "gonf-network-rollback.service",
	"gonf-network-rollback-boot.service", "gonf-network-rollback-runtime.service")

func (NetworkRetirement) OptsRetire() TaskOptions { return TaskOptions{Operational()} }

// Retire removes the network-migration watchdog, restore units, drop-in and helper.
func (NetworkRetirement) Retire() {
	// Refuse while a transaction is pending: its rollback still needs the
	// helper. Skipped (no change) when nothing is pending.
	pending := Command("sh", List("-c", "echo 'pending network-migration transaction' >&2; exit 1"),
		OnlyIf("test", List("-d", networkState+"/pending")))
	disable := retireNetworkUnits(pending)
	NoFile(List(networkMigrationHelper, "/etc/gonf-network-migration.conf"), DependsOn(disable))
	NoDir(networkState, WithPrune, DependsOn(disable))
}

// retireNetworkUnits stops the timer, disables the units, checks that k3s and
// NetworkManager no longer depend on them, then deletes the unit files and
// the drop-in and reloads systemd. Disable drops the enablement links
// (systemctl disable reloads the daemon itself). Only the timer is stopped:
// a stop job for a unit k3s or NetworkManager Requires would stop them too.
// The active oneshot boot/runtime units stay loaded as "not-found" until the
// next boot.
func retireNetworkUnits(pending Resource) Resource {
	present := `for u in "$@"; do [ -e "/etc/systemd/system/$u" ] && exit 0; done; exit 1`
	stopTimer := Command("systemctl", List("stop", "gonf-network-rollback.timer"),
		OnlyIf("systemctl", List("is-active", "--quiet", "gonf-network-rollback.timer")), DependsOn(pending))
	disable := Command("sh", append(List("-c", `for u in "$@"; do [ ! -e "/etc/systemd/system/$u" ] || systemctl disable --quiet "$u"; done`, "disable"), networkUnits...),
		OnlyIf("sh", append(List("-c", present, "present"), networkUnits...)), DependsOn(stopTimer))
	// Belt and braces before any file goes: a remaining Requires/Wants on a
	// deleted unit would fail k3s or NetworkManager at their next start.
	check := Command("sh", List("-c", `for u in k3s.service NetworkManager.service; do
	case "$(systemctl show -p Requires -p Wants --value "$u" | tr '\n' ' ')" in
	*gonf-network-rollback*) echo "$u still depends on gonf-network-rollback" >&2; exit 1 ;;
	esac
done`), OnChange(disable))
	var files []string
	for _, unit := range networkUnits {
		files = append(files, "/etc/systemd/system/"+unit)
	}
	removed := NoFile(append(files, networkDropIn), DependsOn(disable, check))
	Sh("systemctl daemon-reload", OnChange(disable, removed))
	return disable
}
