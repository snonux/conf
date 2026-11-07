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

// OptsMigrationDNS keeps future ingress addresses out of ordinary deployments.
func (Deployment) OptsMigrationDNS() TaskOptions { return TaskOptions{Operational()} }

// OptsMigrationService keeps guarded resolver service staging out of ordinary deployments.
func (Deployment) OptsMigrationService() TaskOptions { return TaskOptions{Operational()} }

// MigrationService stages the resolver service for the existing recovery transaction.
func (Deployment) MigrationService() {
	const staging = "/var/lib/f3s-pihole-migration"
	EnsureDir(staging, RootPrivate)
	InstallFile(staging+"/docker-compose.yml", paths.PiholeAsset("docker-compose.yml"), RootPrivate)
}

// MigrationDNS stages a future wildcard outside every active container mount.
func (Deployment) MigrationDNS() {
	const staging = "/var/lib/f3s-pihole-migration"
	EnsureDir(staging, RootPrivate)
	InstallFile(staging+"/99-f3s-lan-wildcard.conf",
		paths.GonfAsset("pihole", "network-migration/99-f3s-lan-wildcard-new-vip.conf"), RootPrivate)
}

func (Deployment) OptsMigrationCurrentDNS() TaskOptions { return TaskOptions{Operational()} }
func (Deployment) OptsMigrationDNSCanary() TaskOptions  { return TaskOptions{Operational()} }

// MigrationDNSCanary stages a conf-only disposable record to prove directory loading.
func (Deployment) MigrationDNSCanary() { stageCurrentDNS(true) }

// MigrationCurrentDNS stages accepted host records and the current wildcard.
func (Deployment) MigrationCurrentDNS() {
	stageCurrentDNS(false)
}

func stageCurrentDNS(canary bool) {
	const staging = "/var/lib/f3s-pihole-migration"
	rows, err := hostDNSRows()
	if err != nil {
		Refuse("Pi-hole hosts", staging, err)
		return
	}
	if canary {
		rows += "host-record=gonf-924-dns-canary.lan.buetow.org,192.0.2.24,60\n"
	}
	EnsureDir(staging, RootPrivate)
	InstallFile(staging+"/docker-compose.yml", paths.PiholeAsset("docker-compose.yml"), RootPrivate,
		WithValidation("docker", List("compose", "--env-file", piholeDir+"/.env", "-f", CandidatePath, "config", "--quiet")))
	File(staging+"/99-f3s-lan-hosts.conf", WithContent(rows), RootPrivate,
		WithValidation("sh", List("-ec", `docker exec -i pihole pihole-FTL dnsmasq-test --conf-file=- < "$1"`, "dnsmasq-check", CandidatePath)))
	InstallFile(staging+"/99-f3s-lan-wildcard.conf", paths.PiholeAsset("dnsmasq.d/99-f3s-lan-wildcard.conf"), RootPrivate,
		WithValidation("sh", List("-ec", `docker exec -i pihole pihole-FTL dnsmasq-test --conf-file=- < "$1"`, "dnsmasq-check", CandidatePath)))
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
