package frontends

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/snonux/gonf/api"

	"codeberg.org/snonux/conf/gonf/paths"
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
	// nsdKeyName is the TSIG key both NSD configurations share: the
	// publisher names it next to the secondary's address in notify and
	// provide-xfr, the secondary next to the publisher's address in
	// allow-notify and request-xfr.
	nsdKeyName = DNSPublisher + "." + Domain
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
func (MailDNS) OptsSMTPD() TaskOptions { return TaskOptions{Needs("acme")} }

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

// NSD installs immutable publisher inputs and invokes the sole publisher on
// blowfish (DNSPublisher), and configures fishfinger (the Master service host)
// as the NSD secondary that receives the effective zones by zone transfer.
// Each host block declares its own nsd_flags line, so the task records one
// scope per host inside the task's frontends guard.
//
// The logical reference paths.FrontendSecret("var/nsd/etc/nsd_key.txt")
// resolves through the provider cmd/gonf/main.go configures
// (api.SetSecretProvider): since task ze2 the foostore/KeePass entry
// Infra/nsd-tsig-key (Password field), through secret.NewFallback. A locked
// or unavailable vault fails the plan loudly rather than falling back to
// gonf/secrets/frontends/var/nsd/etc/nsd_key.txt, which is kept only as a
// legacy copy for an unmapped reference (see gonf/secrets/README.md, "Typed
// provider references and cutover policy"). This call site and the
// byte-for-byte key.conf output below it are unchanged by the cutover.
func (MailDNS) NSD() {
	key := strings.TrimSpace(MustSecret(paths.FrontendSecret("var/nsd/etc/nsd_key.txt")))
	data := TemplateData()
	WhenHostname(DNSPublisher, func() { nsdPublisher(data, key) })
	WhenHostname(Master, func() { nsdSecondary(data, key) })
}

// DescDNSFailover returns the description for the DNS high-availability job.
func (MailDNS) DescDNSFailover() string {
	return "Install the frontend DNS failover script and root cron entry"
}

// DNSFailover installs the health decision client. It never writes an effective
// zone: successful role changes are handed to dns-publish.ksh, which shares the
// same lock and transaction as ordinary Gonf publication.
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

// nsdPublisher declares blowfish's publisher. The publisher, not this task,
// owns effective zones, SOA serials, failover state, validation, locking, and
// rollback, and it makes the running NSD pick up each commit: a zone reload
// for zone-only changes, a restart when its nsd.conf or TSIG key changed (the
// Service below only watches the flags). Every publisher input is rendered
// first (renderPublisherFiles): a render failure refuses the host's publisher
// declarations instead of declaring an input with empty content.
func nsdPublisher(data Data, key string) {
	files, ok := renderPublisherFiles(data, key)
	if !ok {
		return
	}
	flags := nsdFlags()
	publisherScript := InstallFile(dnsPublishCommand, legacyFrontendAsset("scripts/dns-publish.ksh"),
		Perm(0o500, Root))
	inputs := dnsPublisherInputs(data.DNSZones, files)
	deps := append([]Dependency{flags, publisherScript}, inputs...)
	publisher := Sh(dnsPublishCommand, DependsOn(deps...), WithName("publish-nsd-zones"))
	Service("nsd", WithRestart, DependsOn(publisher), OnChange(flags))
}

// nsdSecondary declares fishfinger's NSD secondary. It has no source
// templates and no zone-writing command: its slave zone files are solely NSD's
// transfer destination. The key include and the configuration are validated
// together with nsd-checkconf, staged inside NSD's chroot, before either is
// live, and a published change restarts NSD. Both
// members are rendered before anything is declared, so a render failure
// refuses the host's secondary declarations (renderBatch).
func nsdSecondary(data Data, key string) {
	var batch renderBatch
	keyConf := batch.render("/var/nsd/etc/key.conf", func() (string, error) { return renderNSDKey(key) })
	nsdConf := batch.render("/var/nsd/etc/nsd.conf", func() (string, error) {
		return renderNSDSlaveConfig(MemberPath("key.conf"), data.DNSZones)
	})
	if batch.refused() {
		return
	}
	flags := nsdFlags()
	config := ConfigSet("nsd",
		ConfigFile("key.conf", "/var/nsd/etc/key.conf", WithContent(keyConf),
			Perm(0o640, "root:_nsd")),
		ConfigFile("nsd.conf", "/var/nsd/etc/nsd.conf", WithContent(nsdConf),
			Perm(0o640, "root:_nsd")),
		WithChroot("/var/nsd"),
		WithSetValidation("nsd-checkconf", List(MemberPath("nsd.conf"))))
	Service("nsd", WithRestart, OnChange(flags, config))
}

// publisherFiles is the rendered content of the publisher's immutable
// inputs: one zone template per DNS zone (zones, in data.DNSZones order),
// plus its shell-sourced publisher.conf, TSIG key.conf and master nsd.conf.
type publisherFiles struct {
	zones         []string
	publisherConf string
	keyConf       string
	nsdConf       string
}

// publisherZonePath is the live path of zone's immutable publisher input.
func publisherZonePath(zone string) string {
	return filepath.Join(dnsPublisherZones, zone+".zone.tpl")
}

// renderPublisherFiles renders every publisher input on the controller. On
// a render failure it reports the declaration error (renderBatch.refused)
// and returns false, so nsdPublisher declares none of the host's resources.
func renderPublisherFiles(data Data, key string) (publisherFiles, bool) {
	var batch renderBatch
	files := publisherFiles{zones: make([]string, 0, len(data.DNSZones))}
	for _, zone := range data.DNSZones {
		files.zones = append(files.zones, batch.render(publisherZonePath(zone), func() (string, error) {
			return renderZoneTemplate(zone, data.F3SHosts)
		}))
	}
	files.publisherConf = batch.render(filepath.Join(dnsPublisherDir, "publisher.conf"), func() (string, error) {
		return renderDNSPublisherConfig(data.DNSZones, data.DNSZonesRemove)
	})
	files.keyConf = batch.render(filepath.Join(dnsPublisherDir, "key.conf"), func() (string, error) {
		return renderNSDKey(key)
	})
	files.nsdConf = batch.render(filepath.Join(dnsPublisherDir, "nsd.conf"), func() (string, error) {
		return renderNSDConfig("/var/nsd/etc/key.conf", data.DNSZones, "master")
	})
	return files, !batch.refused()
}

// dnsPublisherInputs declares the publisher's input directories and its
// already-rendered input files (see renderPublisherFiles); zones and
// files.zones share one order. Each file applies after its directory
// without DependsOn: gonf orders a path after the Dir that contains it.
func dnsPublisherInputs(zones []string, files publisherFiles) []Dependency {
	inputs := []Dependency{
		Dir(dnsPublisherDir, Perm(0o700, Root)),
		Dir(dnsPublisherZones, Perm(0o700, Root)),
	}
	input := func(path, content string) {
		inputs = append(inputs, File(path, WithContent(content), Perm(0o600, Root)))
	}
	for i, zone := range zones {
		input(publisherZonePath(zone), files.zones[i])
	}
	input(filepath.Join(dnsPublisherDir, "publisher.conf"), files.publisherConf)
	input(filepath.Join(dnsPublisherDir, "key.conf"), files.keyConf)
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
			Perm(0o644, Root)))
	}
	return append(opts,
		ConfigFile("smtpd.conf", "/etc/mail/smtpd.conf", WithContent(smtpdConf),
			Perm(0o644, Root)),
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

// nsdKeyTemplateData is the explicit, typed data for assets/key.conf.tmpl.
// Secret already carries Go's %q quoting (strconv.Quote): the template only
// interpolates it, so the literal resolved secret value stays byte-identical
// to what the automatic plan secret scanner (api.RecordPlanTo) matches on,
// exactly as the previous fmt.Sprintf("%q", key) rendering did.
type nsdKeyTemplateData struct {
	Name   string
	Secret string
}

// renderNSDKey renders the TSIG key stanza both NSD roles include.
func renderNSDKey(key string) (string, error) {
	data := nsdKeyTemplateData{Name: nsdKeyName, Secret: strconv.Quote(key)}
	return renderControllerTemplate(frontendAsset("key.conf.tmpl"), data)
}

// nsdZoneRecord is one zone's row in a rendered nsd.conf: master and
// secondary configurations both key each zone by name, its zonefile path,
// and the one peer address+TSIG-key pair NSD needs for notify/provide-xfr
// (master) or allow-notify/request-xfr (secondary). Name and ZoneFile already
// carry Go's %q quoting (strconv.Quote, see nsdZoneRecords) because the
// templates interpolate them verbatim; Peer is emitted unquoted.
type nsdZoneRecord struct {
	Name     string
	ZoneFile string
	Peer     string
}

// nsdConfigData is the explicit, typed data shared by assets/nsd-master.conf.tmpl
// and assets/nsd-slave.conf.tmpl. Include is the complete, already-quoted
// operand of the "include:" line: each role's renderer decides how its key
// path is quoted (see renderNSDConfig and renderNSDSlaveConfig), so both
// templates just interpolate it.
type nsdConfigData struct {
	Include string
	Zones   []nsdZoneRecord
}

// nsdZoneRecords builds one pre-quoted nsdZoneRecord per zone, with its
// zonefile below zoneDir and the shared peer ("<IPv4> <TSIG key name>").
func nsdZoneRecords(zones []string, zoneDir, peer string) []nsdZoneRecord {
	records := make([]nsdZoneRecord, 0, len(zones))
	for _, zone := range zones {
		records = append(records, nsdZoneRecord{
			Name:     strconv.Quote(zone),
			ZoneFile: strconv.Quote(filepath.Join(zoneDir, zone+".zone")),
			Peer:     peer,
		})
	}
	return records
}

// renderNSDConfig renders the publisher's (blowfish's) NSD master
// configuration from assets/nsd-master.conf.tmpl. Every zone notifies the NSD
// secondary (the Master service host, fishfinger) and allows it to transfer
// the zone, both authenticated with the shared TSIG key nsdKeyName: without
// provide-xfr NSD refuses the secondary's request-xfr, and without notify the
// secondary only picks up a new serial at the zone's SOA refresh. The
// secondary's IPv4 mirrors its own allow-notify/request-xfr
// (renderNSDSlaveConfig), which name the publisher's IPv4, so notifies and
// transfers both run over IPv4. keyPath here is a fixed live path, not a
// ConfigSet placeholder, so it is quoted with strconv.Quote like the zone
// names and zonefile paths.
func renderNSDConfig(keyPath string, zones []string, zoneDir string) (string, error) {
	secondary := MustServer(Master).IPv4 + " " + nsdKeyName
	data := nsdConfigData{Include: strconv.Quote(keyPath), Zones: nsdZoneRecords(zones, zoneDir, secondary)}
	return renderControllerTemplate(frontendAsset("nsd-master.conf.tmpl"), data)
}

// renderNSDSlaveConfig renders the NSD secondary's (fishfinger's)
// configuration from assets/nsd-slave.conf.tmpl. keyPath is a ConfigSet
// member placeholder, which contains NUL bytes: it is wrapped in literal
// quotes instead of strconv.Quote, because quoting would escape the NUL
// bytes and break the placeholder gonf substitutes.
//
// NSD's allow-notify and request-xfr take an address followed by a TSIG key
// name (or NOKEY); a bare hostname does not parse. The address is the DNS
// publisher's IPv4 from the frontend inventory and the key is nsdKeyName,
// matching the retired Rex nsd.conf.slave.tpl ("23.88.35.144
// blowfish.buetow.org"; removed in conf task v42, see git history).
// The explicit AXFR transfer type is kept from the earlier gonf port.
func renderNSDSlaveConfig(keyPath string, zones []string) (string, error) {
	master := MustServer(DNSPublisher).IPv4 + " " + nsdKeyName
	data := nsdConfigData{Include: `"` + keyPath + `"`, Zones: nsdZoneRecords(zones, "slave", master)}
	return renderControllerTemplate(frontendAsset("nsd-slave.conf.tmpl"), data)
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

// renderDNSPublisherConfig renders the publisher's shell-sourced identity and
// zone-list file from assets/publisher.conf.tmpl.
func renderDNSPublisherConfig(zones, removedZones []string) (string, error) {
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
		PublisherFQDN: q(DNSPublisher + "." + Domain),
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
// render. The SOA MNAME override to DNSPublisher is likewise not a template
// variable: the plan's DNS publication contract fixes the stable publisher
// identity regardless of what a template's own SOA line names, so it stays a
// deliberate post-render replacement here rather than something a template
// edit could accidentally change.
//
// A render failure, or a leftover legacy Rex "<%" directive in the result (a
// template not fully ported to Go's text/template), is returned for the
// caller to report (see renderPublisherFiles).
func renderZoneTemplate(zone string, f3sHosts []string) (string, error) {
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
	content = strings.ReplaceAll(content,
		"fishfinger.buetow.org. hostmaster.buetow.org.", DNSPublisher+"."+Domain+". hostmaster."+Domain+".")
	if strings.Contains(content, "<%") {
		return "", fmt.Errorf("unrendered legacy NSD template directive in %q", zone)
	}
	return content, nil
}
