package frontends

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/snonux/gonf/api"
)

const (
	// legacySMTPDValidationDir held the fixed-path SMTPD candidates before
	// the ConfigSet migration; SMTPD removes it (a no-op once gone). Drop the
	// cleanup when no host has the directory any more.
	legacySMTPDValidationDir = "/var/tmp/gonf-smtpd"
	dnsFailoverCommand       = "/usr/local/bin/dns-failover.ksh"
	dnsPublishCommand        = "/usr/local/bin/dns-publish.ksh"
	dnsPublisherDir          = "/var/nsd/etc/gonf-publisher"
	dnsPublisherZones        = dnsPublisherDir + "/zones"
	// nsdSlaveZoneDir is where the retired NSD secondary (fishfinger until
	// the KISS HA restore) received its transferred zones. The directory
	// itself is part of OpenBSD's base layout and stays; only the stale
	// transferred zone files are pruned (nsdPublisher).
	nsdSlaveZoneDir = "/var/nsd/zones/slave"
	// retiredNSDKeyFile (live NSD include) and retiredNSDKeyInputFile
	// (publisher input) held the TSIG key of the retired AXFR secondary
	// setup. Both frontends are independent masters without zone transfer,
	// so nsdPublisher removes them (see there for the ordering).
	retiredNSDKeyFile      = "/var/nsd/etc/key.conf"
	retiredNSDKeyInputFile = dnsPublisherDir + "/key.conf"
)

// MailDNS contains the frontend SMTP and authoritative-DNS recipes. The
// services share no mutable configuration state, but sit together because they
// are the mail/DNS ownership boundary formerly represented by the Rex tasks.
type MailDNS struct {
	RequiresRoot
}

// DescSMTPD returns the description for the OpenSMTPD recipe.
func (MailDNS) DescSMTPD() string {
	return "Render, validate, and converge frontend OpenSMTPD (needs /etc/ssl certificates from a frontends_acme_invoke run)"
}

// OptsSMTPD records the ACME setup (frontends_acme) before SMTPD; the
// certificates themselves come from the explicit, Operational
// frontends_acme_invoke, which Needs must not name (a task needing
// Operational work would drop out of the frontends aggregate).
func (MailDNS) OptsSMTPD() TaskOptions { return TaskOptions{Needs(Maintenance.ACME)} }

// SMTPD publishes every lookup table plus the host-specific configuration as
// one Gonf ConfigSet: the complete set is staged privately under /etc/mail and
// validated with `smtpd -n` before any live table or configuration changes.
// newaliases is intentionally limited to the aliases member; smtpd restarts for
// its own configuration and the other lookup tables, matching the Rex behavior.
// The obsolete fixed-path candidate directory of the previous recipe is
// removed. A smtpd.conf render failure refuses the host's SMTPD declarations
// (see refuseRender) instead of declaring the set with an empty smtpd.conf.
func (MailDNS) SMTPD() {
	EachHost(func(server Server) {
		smtpdConf, err := renderSMTPD(server)
		if err != nil {
			refuseRender("/etc/mail/smtpd.conf", err)
			return
		}
		NoDir(legacySMTPDValidationDir, WithPrune)
		mail := ConfigSet("smtpd", smtpdConfigSet(smtpdConf)...)
		newAliases := Sh("newaliases", OnChange(mail.Member("aliases")), WithName("rebuild-mail-aliases"))
		Service("smtpd", WithRestart, DependsOn(newAliases), OnChange(mail.Members(smtpdRestartMembers()...)...))
	})
}

// DescNSD returns the description for the authoritative DNS recipe.
func (MailDNS) DescNSD() string {
	return "Render, validate, and converge frontend authoritative NSD zones"
}

// NSD installs immutable publisher inputs and invokes the publisher on every
// frontend: each one is an independent NSD master for all zones (KISS HA, no
// zone transfer), and dns-failover.ksh picks the role both publish. Each
// host block declares its own nsd_flags line, so the task records one scope
// per host inside the task's frontends guard. No secret is involved: without
// zone transfer NSD needs no TSIG key (removed 2026-11-13).
func (MailDNS) NSD() {
	data := TemplateData()
	EachHost(func(server Server) { nsdPublisher(data, server) })
}

// DNSFailover installs the frontend DNS failover script and root cron entry.
//
// DNSFailover installs the health decision client on both frontends. It never
// writes an effective zone: role changes are handed to the local
// dns-publish.ksh, which shares the same lock and transaction as ordinary
// Gonf publication.
func (MailDNS) DNSFailover() {
	script := InstallFile(dnsFailoverCommand, legacyFrontendAsset("scripts/dns-failover.ksh"),
		Perm(0o500, Root))
	publisher := InstallFile(dnsPublishCommand, legacyFrontendAsset("scripts/dns-publish.ksh"),
		Perm(0o500, Root))
	CronAt("frontend-nsd-failover", "* * * * *", "-ns "+dnsFailoverCommand, DependsOn(script, publisher))
}

// nsdFlags declares the empty nsd_flags line that enables NSD on the host.
// It stays a line of /etc/rc.conf.local instead of Service WithFlags(""):
// /etc/rc.d/nsd sets default flags (-c /var/nsd/etc/nsd.conf), which
// `rcctl get nsd flags` reports for the empty line, so empty flags would
// never converge.
func nsdFlags() Resource {
	return rcConfLocalLine("nsd_flags=", "rc-conf-nsd-flags")
}

// nsdPublisher declares server's publisher. The publisher, not this task,
// owns effective zones, SOA serials, failover state, validation, locking, and
// rollback, and it makes the running NSD pick up each commit: a zone reload
// for zone-only changes, a restart when its nsd.conf changed (the
// Service below only watches the flags). Every publisher input is rendered
// first (renderPublisherFiles): a render failure refuses the host's publisher
// declarations instead of declaring an input with empty content.
//
// The retired secondary's transferred zone files are pruned only after the
// publisher ran: by then the live nsd.conf names the master zones, so NSD
// never loses a zone file it is configured to serve. The files are absent on
// a host that was never a secondary, which makes this a no-op there. The
// publisher reads them once before (serial_for) to keep serials monotonic.
// The retired TSIG key files go the same way: the live key.conf only after
// the publisher installed an nsd.conf without its include, the input at once
// (the publisher no longer reads it).
func nsdPublisher(data Data, server Server) {
	files, ok := renderPublisherFiles(data, server)
	if !ok {
		return
	}
	flags := nsdFlags()
	publisherScript := InstallFile(dnsPublishCommand, legacyFrontendAsset("scripts/dns-publish.ksh"),
		Perm(0o500, Root))
	inputs := dnsPublisherInputs(data.DNSZones, files)
	deps := append([]Dependency{flags, publisherScript}, inputs...)
	publisher := Sh(dnsPublishCommand, DependsOn(deps...), WithName("publish-nsd-zones"))
	service := Service("nsd", WithRestart, DependsOn(publisher), OnChange(flags))
	for _, zone := range data.DNSZones {
		NoFile(filepath.Join(nsdSlaveZoneDir, zone+".zone"), DependsOn(publisher, service))
	}
	NoFile(retiredNSDKeyFile, DependsOn(publisher, service))
	NoFile(retiredNSDKeyInputFile)
}

// publisherFiles is the rendered content of the publisher's immutable
// inputs: one zone template per DNS zone (zones, in data.DNSZones order),
// plus its shell-sourced publisher.conf and master nsd.conf.
type publisherFiles struct {
	zones         []string
	publisherConf string
	nsdConf       string
}

// publisherZonePath is the live path of zone's immutable publisher input.
func publisherZonePath(zone string) string {
	return filepath.Join(dnsPublisherZones, zone+".zone.tpl")
}

// renderPublisherFiles renders every publisher input of server on the
// controller. On a render failure it reports the declaration error
// (renderBatch.refused) and returns false, so nsdPublisher declares none of
// the host's resources.
func renderPublisherFiles(data Data, server Server) (publisherFiles, bool) {
	var batch renderBatch
	files := publisherFiles{zones: make([]string, 0, len(data.DNSZones))}
	for _, zone := range data.DNSZones {
		files.zones = append(files.zones, batch.render(publisherZonePath(zone), func() (string, error) {
			return renderZoneTemplate(zone, data.F3SHosts, server)
		}))
	}
	files.publisherConf = batch.render(filepath.Join(dnsPublisherDir, "publisher.conf"), func() (string, error) {
		return renderDNSPublisherConfig(data.DNSZones, data.DNSZonesRemove, server)
	})
	files.nsdConf = batch.render(filepath.Join(dnsPublisherDir, "nsd.conf"), func() (string, error) {
		return renderNSDConfig(data.DNSZones, "master")
	})
	return files, !batch.refused()
}

// dnsPublisherInputs declares the publisher's input directories and its
// already-rendered input files (see renderPublisherFiles); zones and
// files.zones share one order. Each file applies after its directory
// without DependsOn: gonf orders a path after the Dir that contains it.
func dnsPublisherInputs(zones []string, files publisherFiles) []Dependency {
	inputs := []Dependency{
		Dir(dnsPublisherDir, RootPrivate),
		Dir(dnsPublisherZones, RootPrivate),
	}
	input := func(path, content string) {
		inputs = append(inputs, File(path, WithContent(content), RootPrivate))
	}
	for i, zone := range zones {
		input(publisherZonePath(zone), files.zones[i])
	}
	input(filepath.Join(dnsPublisherDir, "publisher.conf"), files.publisherConf)
	input(filepath.Join(dnsPublisherDir, "nsd.conf"), files.nsdConf)
	return inputs
}

// smtpdConfigSet returns the SMTPD set: every lookup table (keyed by its file
// name), smtpd.conf (smtpdConf, already rendered by renderSMTPD) referencing
// them through member placeholders, and the `smtpd -n` validator run against
// the staged configuration.
func smtpdConfigSet(smtpdConf string) []ConfigSetOption {
	opts := make([]ConfigSetOption, 0, len(mailTableNames)+2)
	for _, name := range mailTableNames {
		opts = append(opts, ConfigFile(name, filepath.Join("/etc/mail", name), WithSource(legacyFrontendAsset(filepath.Join("etc/mail", name))),
			RootOwned))
	}
	return append(opts,
		ConfigFile("smtpd.conf", "/etc/mail/smtpd.conf", WithContent(smtpdConf),
			RootOwned),
		WithSetValidation("smtpd", List("-n", "-f", MemberPath("smtpd.conf"))))
}

// smtpdRestartMembers are the members whose publication restarts smtpd:
// everything except aliases, which only rebuilds the alias database.
func smtpdRestartMembers() []string {
	members := make([]string, 0, len(mailTableNames))
	for _, name := range mailTableNames {
		if name != "aliases" {
			members = append(members, name)
		}
	}
	return append(members, "smtpd.conf")
}

var mailTableNames = []string{
	"aliases",
	"virtualdomains",
	"virtualusers",
	"reject-senders",
	"reject-domains",
	"reject-recipients",
}

// smtpdTemplateData is the explicit, typed data for assets/smtpd.conf.tmpl.
// The table fields hold ConfigSet member placeholders (see MemberPath), not
// paths: Gonf substitutes each one for the staged candidate path while
// validating and for the live /etc/mail/<table> path when publishing.
type smtpdTemplateData struct {
	FQDN             string
	Aliases          string
	VirtualDomains   string
	VirtualUsers     string
	RejectSenders    string
	RejectDomains    string
	RejectRecipients string
}

// renderSMTPD renders smtpd.conf from the native assets/smtpd.conf.tmpl
// template, run on the controller (see renderControllerTemplate, whose
// error it returns).
func renderSMTPD(server Server) (string, error) {
	data := smtpdTemplateData{
		FQDN:             server.FQDN,
		Aliases:          MemberPath("aliases"),
		VirtualDomains:   MemberPath("virtualdomains"),
		VirtualUsers:     MemberPath("virtualusers"),
		RejectSenders:    MemberPath("reject-senders"),
		RejectDomains:    MemberPath("reject-domains"),
		RejectRecipients: MemberPath("reject-recipients"),
	}
	return renderControllerTemplate(frontendAsset("smtpd.conf.tmpl"), data)
}

// nsdZoneRecord is one zone's row in the rendered nsd.conf: its name and
// zonefile path. Both already carry Go's %q quoting (strconv.Quote, see
// nsdZoneRecords) because the template interpolates them verbatim.
type nsdZoneRecord struct {
	Name     string
	ZoneFile string
}

// nsdConfigData is the explicit, typed data for assets/nsd-master.conf.tmpl.
type nsdConfigData struct {
	Zones []nsdZoneRecord
}

// nsdZoneRecords builds one pre-quoted nsdZoneRecord per zone, with its
// zonefile below zoneDir.
func nsdZoneRecords(zones []string, zoneDir string) []nsdZoneRecord {
	records := make([]nsdZoneRecord, 0, len(zones))
	for _, zone := range zones {
		records = append(records, nsdZoneRecord{
			Name:     strconv.Quote(zone),
			ZoneFile: strconv.Quote(filepath.Join(zoneDir, zone+".zone")),
		})
	}
	return records
}

// renderNSDConfig renders a frontend's NSD master configuration from
// assets/nsd-master.conf.tmpl. Both frontends are independent masters (KISS
// HA): no zone notifies or transfers to the peer, because each frontend
// renders its own zones and dns-failover.ksh keeps their roles in step, so
// NSD needs no TSIG key and the configuration includes none.
func renderNSDConfig(zones []string, zoneDir string) (string, error) {
	data := nsdConfigData{Zones: nsdZoneRecords(zones, zoneDir)}
	return renderControllerTemplate(frontendAsset("nsd-master.conf.tmpl"), data)
}

// publisherConfigData is the explicit, typed data for assets/publisher.conf.tmpl.
// Every field already carries Go's %q quoting (strconv.Quote, applied by
// renderDNSPublisherConfig), which also makes each value a valid double-quoted
// shell word for the ksh scripts that source the file; the template only
// interpolates them.
type publisherConfigData struct {
	DefaultRole   string
	MasterName    string
	MasterIPv4    string
	MasterIPv6    string
	StandbyName   string
	StandbyIPv4   string
	StandbyIPv6   string
	PublisherFQDN string
	Zones         string
	RemovedZones  string
}

// renderDNSPublisherConfig renders server's shell-sourced publisher identity
// and zone-list file from assets/publisher.conf.tmpl. dns-failover.ksh
// sources the same file for the two role names.
func renderDNSPublisherConfig(zones, removedZones []string, server Server) (string, error) {
	master := MustServer(Master)
	standby := MustServer(Standby)
	q := strconv.Quote
	data := publisherConfigData{
		DefaultRole:   q(Master),
		MasterName:    q(master.Name),
		MasterIPv4:    q(master.IPv4),
		MasterIPv6:    q(master.IPv6),
		StandbyName:   q(standby.Name),
		StandbyIPv4:   q(standby.IPv4),
		StandbyIPv6:   q(standby.IPv6),
		PublisherFQDN: q(server.FQDN),
		Zones:         q(strings.Join(zones, " ")),
		RemovedZones:  q(strings.Join(removedZones, " ")),
	}
	return renderControllerTemplate(frontendAsset("publisher.conf.tmpl"), data)
}

// f3sZoneHost is one f3s host's row in the buetow.org zone template's
// {{range .F3SHosts}} block. A single-family name (an "ipv4."/"ipv6."
// alternative name, see SiteFor) gets only its own record type.
type f3sZoneHost struct {
	Name    string
	HasA    bool
	HasAAAA bool
}

// zoneTemplateData is the explicit, typed data for the native zone templates
// under frontends/var/nsd/zones/master/*.zone.tpl. Only buetow.org.zone.tpl
// references F3SHosts; the other zones ignore it.
type zoneTemplateData struct {
	F3SHosts []f3sZoneHost
}

// renderZoneTemplate renders one zone candidate from its native
// frontends/var/nsd/zones/master/<zone>.zone.tpl template, run on the
// controller. The template itself carries the @SERIAL@/@MASTER_IPV4@/
// @MASTER_IPV6@/@STANDBY_IPV4@/@STANDBY_IPV6@ tokens as plain literal text
// (not Go template actions): they are the target-local publisher's contract
// (dns-publish.ksh's `sed -e "s|@SERIAL@|$serial|g" ...`), resolved only
// there from failover state gonf does not have, never by this controller
// render. The SOA MNAME override to server (the frontend that publishes this
// copy, since both are independent masters) is likewise not a template
// variable: it is a deliberate post-render replacement here rather than
// something a template edit could accidentally change.
//
// A render failure, or a leftover legacy Rex "<%" directive in the result (a
// template not fully ported to Go's text/template), is returned for the
// caller to report (see renderPublisherFiles).
func renderZoneTemplate(zone string, f3sHosts []string, server Server) (string, error) {
	hosts := make([]f3sZoneHost, 0, len(f3sHosts))
	for _, host := range f3sHosts {
		family := SiteFor(host).Family
		hosts = append(hosts, f3sZoneHost{Name: host, HasA: family != 6, HasAAAA: family != 4})
	}
	assetPath := legacyFrontendAsset(filepath.Join("var/nsd/zones/master", zone+".zone.tpl"))
	content, err := renderControllerTemplate(assetPath, zoneTemplateData{F3SHosts: hosts})
	if err != nil {
		return "", err
	}
	if zone == Domain {
		// Generated from the shared inventory by go generate ./etchosts.
		// Keeping the immutable fragment avoids a frontends↔etchosts import
		// cycle and leaves the separately cache-gated wildcard in its owner.
		// The "Accepted LAN" heading predates the fragment's rename (f74);
		// it stays so the published zone does not change.
		lan, err := os.ReadFile(legacyFrontendAsset("var/nsd/zones/master/fleet-lan.inc"))
		if err != nil {
			return "", fmt.Errorf("read fleet LAN records: %w", err)
		}
		content = strings.TrimRight(content, "\n") + "\n\n; Accepted LAN records (generated)\n" + string(lan)
	}
	content = strings.ReplaceAll(content,
		"fishfinger.buetow.org. hostmaster.buetow.org.", server.FQDN+". hostmaster."+Domain+".")
	if strings.Contains(content, "<%") {
		return "", fmt.Errorf("unrendered legacy NSD template directive in %q", zone)
	}
	return content, nil
}
