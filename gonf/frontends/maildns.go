package frontends

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"
	"github.com/snonux/gonf/resource"

	"codeberg.org/snonux/conf/gonf/paths"
)

const (
	smtpdValidationDir = "/var/tmp/gonf-smtpd"
	dnsFailoverCommand = "/usr/local/bin/dns-failover.ksh"
	dnsPublishCommand  = "/usr/local/bin/dns-publish.ksh"
	dnsPublisherDir    = "/var/nsd/etc/gonf-publisher"
	dnsPublisherZones  = dnsPublisherDir + "/zones"
)

const legacyDNSFailoverCronPresent = `crontab -l -u root 2>/dev/null | awk '
  /^# BEGIN GONF Cron\[root\/frontend-nsd-failover\]$/ { managed = 1; next }
  /^# END GONF Cron\[root\/frontend-nsd-failover\]$/ { managed = 0; next }
  !managed && $0 !~ /^[[:space:]]*#/ && index($0, "/usr/local/bin/dns-failover.ksh") { found = 1 }
  END { exit !found }
'`

const removeLegacyDNSFailoverCron = `tmp=$(mktemp /tmp/gonf-nsd-failover.XXXXXXXX)
trap 'rm -f "$tmp"' EXIT HUP INT TERM
{ crontab -l -u root 2>/dev/null || true; } | awk '
  /^# BEGIN GONF Cron\[root\/frontend-nsd-failover\]$/ { managed = 1; print; next }
  /^# END GONF Cron\[root\/frontend-nsd-failover\]$/ { managed = 0; print; next }
  !managed && $0 !~ /^[[:space:]]*#/ && index($0, "/usr/local/bin/dns-failover.ksh") { next }
  { print }
' >"$tmp"
crontab -u root "$tmp"`

// MailDNS contains the frontend SMTP and authoritative-DNS recipes. The
// services share no mutable configuration state, but sit together because they
// are the mail/DNS ownership boundary formerly represented by the Rex tasks.
type MailDNS struct {
	RequiresRoot
}

// DescSMTPD returns the description for the OpenSMTPD recipe.
func (MailDNS) DescSMTPD() string {
	return "Render, validate, and converge frontend OpenSMTPD"
}

// SMTPD stages every table plus a host-specific configuration, validates the
// staged configuration, and only then permits its live inputs to change.
// newaliases is intentionally limited to aliases changes; smtpd restarts for
// its own configuration and lookup-table changes, matching the Rex behavior.
func (MailDNS) SMTPD() {
	for _, host := range ClusterHosts() {
		server := MustHostValue[Server](host, ValueServer)
		WhenHostname(host, func() {
			validationDir := Dir(smtpdValidationDir, WithMode(0o700), WithOwner("root"), WithGroup("wheel"))
			candidates := smtpdCandidates(server, validationDir)
			check := Command("smtpd", List("-n", "-f", filepath.Join(smtpdValidationDir, "smtpd.conf")),
				DependsOn(candidates...), WithName("validate-smtpd-config"))
			live := smtpdLiveFiles(server, check)
			aliases := live[0]
			newAliases := Command("newaliases", List(), OnChange(aliases), WithName("rebuild-mail-aliases"))
			Service("smtpd", WithRestart, DependsOn(newAliases), OnChange(live[1:]...))
		})
	}
}

// DescNSD returns the description for the authoritative DNS recipe.
func (MailDNS) DescNSD() string {
	return "Render, validate, and converge frontend authoritative NSD zones"
}

// NSD installs immutable publisher inputs and invokes the sole publisher on
// blowfish. The publisher, not this task, owns effective zones, SOA serials,
// failover state, validation, locking, and reload/rollback. Fishfinger only
// receives those effective zones through NSD zone transfer.
func (MailDNS) NSD() {
	onFrontends(func() {
		flags := File("/etc/rc.conf.local", WithLine("nsd_flags="), WithName("rc-conf-nsd-flags"))
		key := strings.TrimSpace(MustSecret(paths.FrontendSecret("var/nsd/etc/nsd_key.txt")))
		data := TemplateData()

		WhenHostname(DNSPublisher, func() {
			publisherScript := InstallFile(dnsPublishCommand, legacyFrontendAsset("scripts/dns-publish.ksh"),
				WithMode(0o500), WithOwner("root"), WithGroup("wheel"))
			inputs := dnsPublisherInputs(data, key)
			deps := append([]resource.Dependency{flags, publisherScript}, inputs...)
			publisher := Command(dnsPublishCommand, List(), DependsOn(deps...), WithName("publish-nsd-zones"))
			Service("nsd", WithRestart, DependsOn(publisher), OnChange(flags))
		})

		WhenHostname(Master, func() {
			// The standby has no source templates and no zone-writing command.
			// Its master-file path is solely NSD's transfer destination.
			liveKey := File("/var/nsd/etc/key.conf", WithContent(renderNSDKey(key)),
				WithMode(0o640), WithOwner("root"), WithGroup("_nsd"))
			config := File("/var/nsd/etc/nsd.conf", WithContent(renderNSDSlaveConfig("/var/nsd/etc/key.conf", data.DNSZones)),
				WithMode(0o640), WithOwner("root"), WithGroup("_nsd"), DependsOn(liveKey))
			Service("nsd", WithRestart, OnChange(flags, liveKey, config))
		})
	})
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
		cleanup := Command("sh", List("-ceu", removeLegacyDNSFailoverCron),
			OnlyIf("sh", List("-c", legacyDNSFailoverCronPresent)),
			WithName("remove-legacy-dns-failover-cron"))
		Cron("frontend-nsd-failover", WithCommand("-ns "+dnsFailoverCommand), WithMinute("*"), DependsOn(script, publisher, cleanup))
	})
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

func smtpdCandidates(server Server, validationDir Resource) []resource.Dependency {
	candidates := make([]resource.Dependency, 0, len(mailTableNames)+1)
	for _, name := range mailTableNames {
		candidates = append(candidates, InstallFile(filepath.Join(smtpdValidationDir, name), legacyFrontendAsset(filepath.Join("etc/mail", name)),
			WithMode(0o600), WithOwner("root"), WithGroup("wheel"), DependsOn(validationDir)))
	}
	candidates = append(candidates, File(filepath.Join(smtpdValidationDir, "smtpd.conf"),
		WithContent(renderSMTPD(server, smtpdValidationDir)), WithMode(0o600), WithOwner("root"), WithGroup("wheel"), DependsOn(validationDir)))
	return candidates
}

func smtpdLiveFiles(server Server, check Resource) []resource.Dependency {
	live := make([]resource.Dependency, 0, len(mailTableNames)+1)
	for _, name := range mailTableNames {
		live = append(live, InstallFile(filepath.Join("/etc/mail", name), legacyFrontendAsset(filepath.Join("etc/mail", name)),
			WithMode(0o644), WithOwner("root"), WithGroup("wheel"), DependsOn(check)))
	}
	live = append(live, File("/etc/mail/smtpd.conf", WithContent(renderSMTPD(server, "/etc/mail")),
		WithMode(0o644), WithOwner("root"), WithGroup("wheel"), DependsOn(check)))
	return live
}

var mailTableNames = []string{
	"aliases",
	"virtualdomains",
	"virtualusers",
	"reject-senders",
	"reject-domains",
	"reject-recipients",
}

func renderSMTPD(server Server, tableDir string) string {
	return fmt.Sprintf(`# This file is managed by Gonf; validate before applying.
pki "buetow_org_tls" cert "/etc/ssl/%[1]s.fullchain.pem"
pki "buetow_org_tls" key "/etc/ssl/private/%[1]s.key"

table aliases file:%[2]s/aliases
table virtualdomains file:%[2]s/virtualdomains
table virtualusers file:%[2]s/virtualusers
table reject-senders file:%[2]s/reject-senders
table reject-domains file:%[2]s/reject-domains
table reject-recipients file:%[2]s/reject-recipients

listen on socket
listen on all tls pki "buetow_org_tls" hostname "%[1]s"

action localmail mbox alias <aliases>
action receive mbox virtual <virtualusers>
action outbound relay

match from any mail-from <reject-senders> reject
match from any mail-from <reject-domains> reject
match from any for rcpt-to <reject-recipients> reject
match from any for domain <virtualdomains> action receive
match from local for local action localmail
match from local for any action outbound
`, server.FQDN, tableDir)
}

func renderNSDKey(key string) string {
	return fmt.Sprintf("key:\n\tname: blowfish.buetow.org\n\talgorithm: hmac-sha256\n\tsecret: %q\n", key)
}

func renderNSDConfig(keyPath string, zones []string, zoneDir string) string {
	var builder strings.Builder
	appendf(&builder, "include: %q\n\n", keyPath)
	appendString(&builder, `server:
	hide-version: yes
	verbosity: 1
	database: "" # disable database
	debug-mode: no

remote-control:
	control-enable: yes
	control-interface: /var/run/nsd.sock
`)
	for _, zone := range zones {
		appendf(&builder, "\nzone:\n\tname: %q\n\tzonefile: %q\n", zone, filepath.Join(zoneDir, zone+".zone"))
	}
	return builder.String()
}

func renderNSDSlaveConfig(keyPath string, zones []string) string {
	var builder strings.Builder
	appendf(&builder, "include: %q\n\n", keyPath)
	appendString(&builder, `server:
	hide-version: yes
	verbosity: 1
	database: "" # disable database
	debug-mode: no

remote-control:
	control-enable: yes
	control-interface: /var/run/nsd.sock
`)
	for _, zone := range zones {
		appendf(&builder, "\nzone:\n\tname: %q\n\tzonefile: %q\n\tallow-notify: %q\n\trequest-xfr: AXFR %q\n",
			zone, filepath.Join("slave", zone+".zone"), DNSPublisher+"."+Domain, DNSPublisher+"."+Domain)
	}
	return builder.String()
}

func renderDNSPublisherConfig(zones, removedZones []string) string {
	master := MustServer(Master)
	standby := MustServer(Standby)
	var builder strings.Builder
	appendf(&builder, "DEFAULT_ROLE=%q\n", Master)
	appendf(&builder, "MASTER_NAME=%q\nMASTER_IPV4=%q\nMASTER_IPV6=%q\n", master.Name, master.IPv4, master.IPv6)
	appendf(&builder, "STANDBY_NAME=%q\nSTANDBY_IPV4=%q\nSTANDBY_IPV6=%q\n", standby.Name, standby.IPv4, standby.IPv6)
	appendf(&builder, "PUBLISHER_FQDN=%q\n", DNSPublisher+"."+Domain)
	appendf(&builder, "ZONES=%q\n", strings.Join(zones, " "))
	appendf(&builder, "REMOVED_ZONES=%q\n", strings.Join(removedZones, " "))
	return builder.String()
}

func renderZoneTemplate(zone string, f3sHosts []string) string {
	content := mustReadFrontendAsset(filepath.Join("var/nsd/zones/master", zone+".zone.tpl"))
	if zone == "buetow.org" {
		content = replaceLegacyF3SZoneLoop(content, renderF3SZoneTemplateRecords(f3sHosts))
	}
	replacer := strings.NewReplacer(
		"<%= time() %>", "@SERIAL@",
		"<%= $ips->{current_master}{ipv4} %>", "@MASTER_IPV4@",
		"<%= $ips->{current_master}{ipv6} %>", "@MASTER_IPV6@",
		"<%= $ips->{current_standby}{ipv4} %>", "@STANDBY_IPV4@",
		"<%= $ips->{current_standby}{ipv6} %>", "@STANDBY_IPV6@",
		"fishfinger.buetow.org. hostmaster.buetow.org.", DNSPublisher+"."+Domain+". hostmaster."+Domain+".",
	)
	content = replacer.Replace(content)
	if strings.Contains(content, "<%") {
		panic(fmt.Sprintf("unrendered legacy NSD template directive in %q", zone))
	}
	return content
}

func replaceLegacyF3SZoneLoop(content, records string) string {
	start := strings.Index(content, "<% for my $host")
	end := strings.Index(content, "\n\n; So joern")
	if start == -1 || end == -1 || end <= start {
		panic("buetow.org zone no longer contains the expected f3s host loop")
	}
	return content[:start] + records + content[end+2:]
}

func renderF3SZoneTemplateRecords(hosts []string) string {
	var builder strings.Builder
	for _, host := range hosts {
		if !strings.HasPrefix(host, "ipv6.") {
			appendf(&builder, "%s.         300 IN A @MASTER_IPV4@ ; Enable failover\nwww.%s.     300 IN A @MASTER_IPV4@ ; Enable failover\nstandby.%s. 300 IN A @STANDBY_IPV4@ ; Enable failover\n", host, host, host)
		}
		if !strings.HasPrefix(host, "ipv4.") {
			appendf(&builder, "%s.         300 IN AAAA @MASTER_IPV6@ ; Enable failover\nwww.%s.     300 IN AAAA @MASTER_IPV6@ ; Enable failover\nstandby.%s. 300 IN AAAA @STANDBY_IPV6@ ; Enable failover\n", host, host, host)
		}
	}
	return builder.String()
}

func mustReadFrontendAsset(name string) string {
	content, err := os.ReadFile(legacyFrontendAsset(name))
	if err != nil {
		panic(fmt.Errorf("read required frontend asset %q: %w", name, err))
	}
	return string(content)
}
