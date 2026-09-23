package frontends

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"
	"github.com/snonux/gonf/resource"

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
	return "Render, validate, and converge frontend OpenSMTPD (needs /etc/ssl certificates from frontends_acme + a frontends_acme_invoke run)"
}

// SMTPD publishes every lookup table plus the host-specific configuration as
// one Gonf ConfigSet: the complete set is staged privately under /etc/mail and
// validated with `smtpd -n` before any live table or configuration changes.
// newaliases is intentionally limited to the aliases member; smtpd restarts for
// its own configuration and the other lookup tables, matching the Rex behavior.
// The obsolete fixed-path candidate directory of the previous recipe is
// removed.
func (MailDNS) SMTPD() {
	ForHosts(ValueServer, func(_ string, server Server) {
		NoDir(legacySMTPDValidationDir, WithPrune)
		mail := ConfigSet("smtpd", smtpdConfigSet(server)...)
		newAliases := Command("newaliases", List(), OnChange(mail.Member("aliases")), WithName("rebuild-mail-aliases"))
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
// scope per host and no enclosing all-frontends scope.
//
// The logical reference paths.FrontendSecret("var/nsd/etc/nsd_key.txt")
// resolves through whatever provider is configured (api.SetSecretProvider);
// today that is unchanged from before task 262 — gonf's default
// secret.FileProvider, reading gonf/secrets/frontends/var/nsd/etc/nsd_key.txt
// below this checkout. A later, separately authorized cutover to a real
// external store (see secrets/README.md, "Typed provider references and
// cutover policy") can move this one reference without touching this call
// site or the byte-for-byte key.conf output below it: gonf's
// secret.NewFallback composes the new store with secret.FileProvider so an
// unmigrated reference keeps reading this checkout exactly as it does now.
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
	onFrontends(func() {
		script := InstallFile(dnsFailoverCommand, legacyFrontendAsset("scripts/dns-failover.ksh"),
			WithMode(0o500), WithOwner("root"), WithGroup("wheel"))
		publisher := InstallFile(dnsPublishCommand, legacyFrontendAsset("scripts/dns-publish.ksh"),
			WithMode(0o500), WithOwner("root"), WithGroup("wheel"))
		Cron("frontend-nsd-failover", WithCommand("-ns "+dnsFailoverCommand),
			WithLegacyCommand("-ns "+dnsFailoverCommand), WithMinute("*"), DependsOn(script, publisher))
	})
}

// nsdFlags declares the empty nsd_flags line that enables NSD on the host.
func nsdFlags() Resource {
	return rcConfLocalLine("nsd_flags=", "rc-conf-nsd-flags")
}

// nsdPublisher declares blowfish's publisher. The publisher, not this task,
// owns effective zones, SOA serials, failover state, validation, locking, and
// rollback, and it makes the running NSD pick up each commit: a zone reload
// for zone-only changes, a restart when its nsd.conf or TSIG key changed (the
// Service below only watches the flags).
func nsdPublisher(data Data, key string) {
	flags := nsdFlags()
	publisherScript := InstallFile(dnsPublishCommand, legacyFrontendAsset("scripts/dns-publish.ksh"),
		WithMode(0o500), WithOwner("root"), WithGroup("wheel"))
	inputs := dnsPublisherInputs(data, key)
	deps := append([]resource.Dependency{flags, publisherScript}, inputs...)
	publisher := Command(dnsPublishCommand, List(), DependsOn(deps...), WithName("publish-nsd-zones"))
	Service("nsd", WithRestart, DependsOn(publisher), OnChange(flags))
}

// nsdSecondary declares fishfinger's NSD secondary. It has no source
// templates and no zone-writing command: its slave zone files are solely NSD's
// transfer destination. The key include and the configuration are validated
// together with nsd-checkconf, staged inside NSD's chroot, before either is
// live, and a published change restarts NSD.
func nsdSecondary(data Data, key string) {
	flags := nsdFlags()
	config := ConfigSet("nsd",
		ConfigFile("key.conf", "/var/nsd/etc/key.conf", WithContent(renderNSDKey(key)),
			WithMode(0o640), WithOwner("root"), WithGroup("_nsd")),
		ConfigFile("nsd.conf", "/var/nsd/etc/nsd.conf", WithContent(renderNSDSlaveConfig(MemberPath("key.conf"), data.DNSZones)),
			WithMode(0o640), WithOwner("root"), WithGroup("_nsd")),
		WithChroot("/var/nsd"),
		WithSetValidation("nsd-checkconf", List(MemberPath("nsd.conf"))))
	Service("nsd", WithRestart, OnChange(flags, config))
}

func dnsPublisherInputs(data Data, key string) []resource.Dependency {
	inputDir := Dir(dnsPublisherDir, WithMode(0o700), WithOwner("root"), WithGroup("wheel"))
	zoneDir := Dir(dnsPublisherZones, WithMode(0o700), WithOwner("root"), WithGroup("wheel"), DependsOn(inputDir))
	inputs := []resource.Dependency{inputDir, zoneDir}
	for _, zone := range data.DNSZones {
		inputs = append(inputs, File(filepath.Join(dnsPublisherZones, zone+".zone.tpl"),
			WithContent(renderZoneTemplate(zone, data.F3SHosts)), WithMode(0o600), WithOwner("root"), WithGroup("wheel"), DependsOn(zoneDir)))
	}
	inputs = append(inputs, File(filepath.Join(dnsPublisherDir, "publisher.conf"),
		WithContent(renderDNSPublisherConfig(data.DNSZones, data.DNSZonesRemove)), WithMode(0o600), WithOwner("root"), WithGroup("wheel"), DependsOn(inputDir)))
	inputs = append(inputs, File(filepath.Join(dnsPublisherDir, "key.conf"), WithContent(renderNSDKey(key)),
		WithMode(0o600), WithOwner("root"), WithGroup("wheel"), DependsOn(inputDir)))
	inputs = append(inputs, File(filepath.Join(dnsPublisherDir, "nsd.conf"),
		WithContent(renderNSDConfig("/var/nsd/etc/key.conf", data.DNSZones, "master")), WithMode(0o600), WithOwner("root"), WithGroup("wheel"), DependsOn(inputDir)))
	return inputs
}

// smtpdConfigSet returns the SMTPD set: every lookup table (keyed by its file
// name), smtpd.conf referencing them through member placeholders, and the
// `smtpd -n` validator run against the staged configuration.
func smtpdConfigSet(server Server) []ConfigSetOption {
	opts := make([]ConfigSetOption, 0, len(mailTableNames)+2)
	for _, name := range mailTableNames {
		opts = append(opts, ConfigFile(name, filepath.Join("/etc/mail", name), WithSource(legacyFrontendAsset(filepath.Join("etc/mail", name))),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel")))
	}
	return append(opts,
		ConfigFile("smtpd.conf", "/etc/mail/smtpd.conf", WithContent(renderSMTPD(server)),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel")),
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
// template, run on the controller (see renderControllerTemplate).
func renderSMTPD(server Server) string {
	data := smtpdTemplateData{
		FQDN:             server.FQDN,
		Aliases:          MemberPath("aliases"),
		VirtualDomains:   MemberPath("virtualdomains"),
		VirtualUsers:     MemberPath("virtualusers"),
		RejectSenders:    MemberPath("reject-senders"),
		RejectDomains:    MemberPath("reject-domains"),
		RejectRecipients: MemberPath("reject-recipients"),
	}
	return renderControllerTemplate("smtpd.conf", frontendAsset("smtpd.conf.tmpl"), data)
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
func renderNSDKey(key string) string {
	data := nsdKeyTemplateData{Name: nsdKeyName, Secret: strconv.Quote(key)}
	return renderControllerTemplate("key.conf", frontendAsset("key.conf.tmpl"), data)
}

// nsdZoneRecord is one zone's row in a rendered nsd.conf: master and
// secondary configurations both key each zone by name, its zonefile path,
// and the one peer address+TSIG-key pair NSD needs for notify/provide-xfr
// (master) or allow-notify/request-xfr (secondary).
type nsdZoneRecord struct {
	Name     string
	ZoneFile string
	Peer     string
}

// nsdConfigData is the explicit, typed data shared by assets/nsd-master.conf.tmpl
// and assets/nsd-slave.conf.tmpl.
type nsdConfigData struct {
	KeyPath string
	Zones   []nsdZoneRecord
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
// ConfigSet placeholder, so the template quotes it with the "quote" func.
func renderNSDConfig(keyPath string, zones []string, zoneDir string) string {
	secondary := MustServer(Master).IPv4 + " " + nsdKeyName
	records := make([]nsdZoneRecord, 0, len(zones))
	for _, zone := range zones {
		records = append(records, nsdZoneRecord{Name: zone, ZoneFile: filepath.Join(zoneDir, zone+".zone"), Peer: secondary})
	}
	data := nsdConfigData{KeyPath: keyPath, Zones: records}
	return renderControllerTemplate("nsd-master.conf", frontendAsset("nsd-master.conf.tmpl"), data)
}

// renderNSDSlaveConfig renders the NSD secondary's (fishfinger's)
// configuration from assets/nsd-slave.conf.tmpl. keyPath is a ConfigSet
// member placeholder, which contains NUL bytes: the template wraps it in
// literal quotes instead of passing it through the "quote" func, because
// quoting would escape the placeholder.
//
// NSD's allow-notify and request-xfr take an address followed by a TSIG key
// name (or NOKEY); a bare hostname does not parse. The address is the DNS
// publisher's IPv4 from the frontend inventory and the key is nsdKeyName,
// matching the Rex nsd.conf.slave.tpl ("23.88.35.144 blowfish.buetow.org").
// The explicit AXFR transfer type is kept from the earlier gonf port.
func renderNSDSlaveConfig(keyPath string, zones []string) string {
	master := MustServer(DNSPublisher).IPv4 + " " + nsdKeyName
	records := make([]nsdZoneRecord, 0, len(zones))
	for _, zone := range zones {
		records = append(records, nsdZoneRecord{Name: zone, ZoneFile: filepath.Join("slave", zone+".zone"), Peer: master})
	}
	data := nsdConfigData{KeyPath: keyPath, Zones: records}
	return renderControllerTemplate("nsd-slave.conf", frontendAsset("nsd-slave.conf.tmpl"), data)
}

// publisherConfigData is the explicit, typed data for assets/publisher.conf.tmpl.
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
func renderDNSPublisherConfig(zones, removedZones []string) string {
	master := MustServer(Master)
	standby := MustServer(Standby)
	data := publisherConfigData{
		DefaultRole:   Master,
		MasterName:    master.Name,
		MasterIPv4:    master.IPv4,
		MasterIPv6:    master.IPv6,
		StandbyName:   standby.Name,
		StandbyIPv4:   standby.IPv4,
		StandbyIPv6:   standby.IPv6,
		PublisherFQDN: DNSPublisher + "." + Domain,
		Zones:         strings.Join(zones, " "),
		RemovedZones:  strings.Join(removedZones, " "),
	}
	return renderControllerTemplate("publisher.conf", frontendAsset("publisher.conf.tmpl"), data)
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
func renderZoneTemplate(zone string, f3sHosts []string) string {
	hosts := make([]f3sZoneHost, 0, len(f3sHosts))
	for _, host := range f3sHosts {
		family := SiteFor(host).Family
		hosts = append(hosts, f3sZoneHost{Name: host, HasA: family != 6, HasAAAA: family != 4})
	}
	assetPath := legacyFrontendAsset(filepath.Join("var/nsd/zones/master", zone+".zone.tpl"))
	content := renderControllerTemplate(zone+".zone.tpl", assetPath, zoneTemplateData{F3SHosts: hosts})
	content = strings.ReplaceAll(content,
		"fishfinger.buetow.org. hostmaster.buetow.org.", DNSPublisher+"."+Domain+". hostmaster."+Domain+".")
	if strings.Contains(content, "<%") {
		panic(fmt.Sprintf("unrendered legacy NSD template directive in %q", zone))
	}
	return content
}

// controllerTemplateFuncs are the functions available to every controller-
// side native template rendered through renderControllerTemplate. quote
// applies Go's %q string quoting (strconv.Quote), matching the previous
// fmt.Sprintf %q verb used throughout the raw-string renderers these
// templates replace.
var controllerTemplateFuncs = template.FuncMap{"quote": strconv.Quote}

// renderControllerTemplate reads, parses, and executes a native
// text/template asset on the controller, returning the rendered content.
// Unlike WithTemplateData (resource/file/template.go), which renders on the
// destination host from facts detected there at apply time, this always runs
// here while the recipe records: SMTPD/NSD candidate content must be
// identical regardless of which frontend later applies it, so the frontend
// Go renderers deliberately keep the controller-render distinction the
// consumer DSL review asked to preserve (see docs/consumer-dsl-simplification-plan.md,
// "External templates and consumer layout"). A missing or malformed asset is
// a broken source-controlled template, not a recipe or input error, so this
// panics like the rest of this package's asset loading (e.g. MustServer).
func renderControllerTemplate(name, assetPath string, data any) string {
	raw, err := os.ReadFile(assetPath)
	if err != nil {
		panic(fmt.Errorf("read required frontend template %q: %w", assetPath, err))
	}
	tmpl, err := template.New(name).Funcs(controllerTemplateFuncs).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		panic(fmt.Errorf("parse frontend template %q: %w", assetPath, err))
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Errorf("render frontend template %q: %w", assetPath, err))
	}
	return buf.String()
}
