// Package pihole declares the Pi-hole Docker deployment on pi2/pi3.
package pihole

import (
	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"

	"codeberg.org/snonux/conf/gonf/paths"
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

// DescCompose returns the description for the compose deployment.
func (Deployment) DescCompose() string {
	return "Install ~paul/pihole/docker-compose.yml and the LAN wildcard, recreate on change"
}

// Compose installs the compose file and the dnsmasq wildcard
// (*.f3s.lan.buetow.org -> the storage VIP), and runs `docker compose up -d`
// when either changes. DNS on the LAN depends on these two Pis, so deploy
// them one at a time when the image tag changes (the compose file pins it).
func (Deployment) Compose() {
	compose := InstallFile(piholeDir+"/docker-compose.yml",
		paths.PiholeAsset("docker-compose.yml"), Perm(0o644, "paul:paul"))
	wildcard := InstallFile(piholeDir+"/etc-dnsmasq.d/99-f3s-lan-wildcard.conf",
		paths.PiholeAsset("dnsmasq.d/99-f3s-lan-wildcard.conf"), Perm(0o644, Root))
	Command("sh", List("-c", "cd "+piholeDir+" && docker compose up -d"),
		OnChange(compose, wildcard))
}
