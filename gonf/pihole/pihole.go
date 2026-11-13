// Package pihole declares the Pi-hole Docker deployment on pi2/pi3.
package pihole

import (
	"strings"

	. "github.com/snonux/gonf/api"

	"github.com/snonux/conf/gonf/etchosts"
	"github.com/snonux/conf/gonf/paths"
)

// Deployment owns the compose file and the LAN wildcard of the Pi-hole
// containers on pi2/pi3 (~paul/pihole). Both used to be hand-edited on each
// Pi; the image pin of 2026-09-25 is what brought them under gonf, so a
// rebuilt Pi gets the pinned version instead of :latest. The host-local .env
// (WEBPASSWORD) and Pi-hole's own state under etc-pihole stay unmanaged.
//
// Register with OnCluster(cluster.NameRockyPis).
type Deployment struct {
	RequiresRoot
}

const piholeDir = "/home/paul/pihole"

// hostDNSRows is the content of 99-f3s-lan-hosts.conf: the fleet's host
// records in the lab view, from the shared etchosts inventory.
func hostDNSRows() string {
	return strings.Join(etchosts.PiHoleHostRecords(), "\n") + "\n"
}

// Compose installs ~paul/pihole/docker-compose.yml and the LAN wildcard,
// recreates on change.
//
// Compose installs the compose file, the dnsmasq wildcard
// (*.f3s.lan.buetow.org -> the storage VIP) and the fleet host records
// (99-f3s-lan-hosts.conf, validated first), and runs `docker compose up -d`
// when any of them changes. DNS on the LAN depends on these two Pis, so deploy
// them one at a time when the image tag changes (the compose file pins it).
func (Deployment) Compose() {
	rows := hostDNSRows()
	compose := InstallFile(piholeDir+"/docker-compose.yml",
		paths.PiholeAsset("docker-compose.yml"), Perm(0o644, "paul:paul"))
	wildcard := InstallFile(piholeDir+"/etc-dnsmasq.d/99-f3s-lan-wildcard.conf",
		paths.PiholeAsset("dnsmasq.d/99-f3s-lan-wildcard.conf"), RootOwned)
	// Privileged validators require a root-owned parent chain; the live mount
	// lives below paul's home, so validate the same bytes outside that tree.
	const validatedDir = "/var/lib/gonf-pihole"
	EnsureDir(validatedDir, RootPrivate)
	validated := File(validatedDir+"/99-f3s-lan-hosts.conf", WithContent(rows), RootPrivate,
		WithValidation("sh", List("-ec", `docker exec -i pihole pihole-FTL dnsmasq-test --conf-file=- < "$1"`, "dnsmasq-check", CandidatePath)))
	hosts := File(piholeDir+"/etc-dnsmasq.d/99-f3s-lan-hosts.conf", WithContent(rows), RootOwned, DependsOn(validated))
	Command("docker", List("compose", "up", "-d", "--force-recreate", "--wait", "--wait-timeout", "90"), WithDir(piholeDir),
		WithName("sh -c cd "+piholeDir+" && docker compose up -d"),
		OnChange(compose, wildcard, hosts))
}
