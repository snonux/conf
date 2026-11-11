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

// stagingDir held the resolver-migration candidates (compose, wildcard and
// host records) that the retired network/endpoint migration helpers applied.
const stagingDir = "/var/lib/f3s-pihole-migration"

// OptsMigrationRetire keeps the one-shot cleanup out of routine deployments.
func (Deployment) OptsMigrationRetire() TaskOptions { return TaskOptions{Operational()} }

// MigrationRetire removes the resolver-migration staging directory and the dormant firewall migration policy (task 774).
//
// Neither is read by the live Pi-hole or the f3s-firewall loader (which uses
// candidate.nft and confirmed.nft). Retire on one Pi at a time and check DNS
// on both before the next. One-shot: delete this task once both Pis are done.
func (Deployment) MigrationRetire() {
	files := NoFile(List(stagingDir+"/docker-compose.yml", stagingDir+"/99-f3s-lan-wildcard.conf",
		stagingDir+"/99-f3s-lan-hosts.conf"))
	// No prune: unexpected files refuse removal rather than being discarded.
	NoDir(stagingDir, DependsOn(files))
	NoFile("/etc/f3s-firewall/migration.nft")
}

func hostDNSRows() (string, error) {
	candidate, err := etchosts.RenderMigration("accepted", "lab")
	if err != nil {
		return "", err
	}
	var rows []string
	for _, row := range candidate.PiHoleDNSMasq {
		if !strings.HasPrefix(row, "#") && !strings.HasPrefix(row, "address=") {
			rows = append(rows, row)
		}
	}
	return strings.Join(rows, "\n") + "\n", nil
}

// Compose installs ~paul/pihole/docker-compose.yml and the LAN wildcard,
// recreates on change.
//
// Compose installs the compose file and the dnsmasq wildcard
// (*.f3s.lan.buetow.org -> the storage VIP), and runs `docker compose up -d`
// when either changes. DNS on the LAN depends on these two Pis, so deploy
// them one at a time when the image tag changes (the compose file pins it).
func (Deployment) Compose() {
	rows, err := hostDNSRows()
	if err != nil {
		Refuse("Pi-hole hosts", piholeDir, err)
		return
	}
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
