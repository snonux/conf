package frontends

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	. "github.com/snonux/gonf/api"
	. "github.com/snonux/gonf/api/options"
	"github.com/snonux/gonf/resource"

	"codeberg.org/snonux/conf/gonf/paths"
)

const (
	smtpdValidationDir = "/var/tmp/gonf-smtpd"
	nsdValidationDir   = "/var/nsd/etc/gonf-validate"
	nsdValidationZones = "/var/nsd/zones/gonf-validate"
	dnsFailoverCommand = "/usr/local/bin/dns-failover.ksh"
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

// NSD renders the legacy zone data controller-side. Candidate keys, config,
// and zones are kept below NSD's chroot so nsd-checkconf validates the same
// path interpretation as the live daemon. The TSIG value never appears in a
// command argument, resource name, description, or controller stdout.
//
// This direct zone writer predates the d52 DNS publication contract. e52 must
// replace it with the single DNSPublisher publication path; do not add another
// writer here or in DNSFailover.
func (MailDNS) NSD() {
	onFrontends(func() {
		flags := File("/etc/rc.conf.local", WithLine("nsd_flags="), WithName("rc-conf-nsd-flags"))
		key := strings.TrimSpace(MustSecret(paths.FrontendSecret("var/nsd/etc/nsd_key.txt")))
		data := TemplateData()
		contents := renderZones(data, time.Now().Unix())

		validationDir := Dir(nsdValidationDir, WithMode(0o700), WithOwner("root"), WithGroup("wheel"))
		validationZones := Dir(nsdValidationZones, WithMode(0o700), WithOwner("root"), WithGroup("wheel"))
		candidateKey := File(filepath.Join(nsdValidationDir, "key.conf"), WithContent(renderNSDKey(key)),
			WithMode(0o600), WithOwner("root"), WithGroup("wheel"), DependsOn(validationDir))

		zoneChecks := make([]resource.Dependency, 0, len(data.DNSZones))
		for _, zone := range data.DNSZones {
			candidate := File(filepath.Join(nsdValidationZones, zone+".zone"), WithContent(contents[zone]),
				WithMode(0o644), WithOwner("root"), WithGroup("wheel"), DependsOn(validationZones))
			zoneChecks = append(zoneChecks, Command("nsd-checkzone", List(zone, filepath.Join(nsdValidationZones, zone+".zone")),
				DependsOn(candidate), WithName("validate-nsd-zone-"+zone)))
		}
		candidateConfig := File(filepath.Join(nsdValidationDir, "nsd.conf"),
			WithContent(renderNSDConfig(filepath.Join(nsdValidationDir, "key.conf"), data.DNSZones, "gonf-validate")),
			WithMode(0o600), WithOwner("root"), WithGroup("wheel"), DependsOn(validationDir, candidateKey))
		checkDeps := append([]resource.Dependency{candidateConfig, candidateKey}, zoneChecks...)
		check := Command("nsd-checkconf", List(filepath.Join(nsdValidationDir, "nsd.conf")),
			DependsOn(checkDeps...), WithName("validate-nsd-config"))

		live := make([]resource.Dependency, 0, len(data.DNSZones)+2)
		liveKey := File("/var/nsd/etc/key.conf", WithContent(renderNSDKey(key)),
			WithMode(0o640), WithOwner("root"), WithGroup("_nsd"), DependsOn(check))
		live = append(live, liveKey)
		liveConfig := File("/var/nsd/etc/nsd.conf", WithContent(renderNSDConfig("/var/nsd/etc/key.conf", data.DNSZones, "master")),
			WithMode(0o640), WithOwner("root"), WithGroup("_nsd"), DependsOn(check))
		live = append(live, liveConfig)
		for _, zone := range data.DNSZones {
			live = append(live, File(filepath.Join("/var/nsd/zones/master", zone+".zone"), WithContent(contents[zone]),
				WithMode(0o644), WithOwner("root"), WithGroup("wheel"), DependsOn(check)))
		}
		for _, zone := range data.DNSZonesRemove {
			live = append(live, NoFile(filepath.Join("/var/nsd/zones/master", zone+".zone"), DependsOn(check)))
		}
		Service("nsd", WithRestart, OnChange(append([]resource.Dependency{flags}, live...)...))
	})
}

// DescDNSFailover returns the description for the DNS high-availability job.
func (MailDNS) DescDNSFailover() string {
	return "Install the frontend DNS failover script and root cron entry"
}

// DNSFailover uses a marker-managed cron resource instead of the Rex
// temporary-crontab surgery. The script's own lock remains defence in depth
// while an old unmanaged Rex entry exists during a staged migration. e52 will
// make this task invoke the sole DNSPublisher instead of editing zones itself.
func (MailDNS) DNSFailover() {
	onFrontends(func() {
		script := InstallFile(dnsFailoverCommand, legacyFrontendAsset("scripts/dns-failover.ksh"),
			WithMode(0o500), WithOwner("root"), WithGroup("wheel"))
		cleanup := Command("sh", List("-ceu", removeLegacyDNSFailoverCron),
			OnlyIf("sh", List("-c", legacyDNSFailoverCronPresent)),
			WithName("remove-legacy-dns-failover-cron"))
		Cron("frontend-nsd-failover", WithCommand("-ns "+dnsFailoverCommand), WithMinute("*"), DependsOn(script, cleanup))
	})
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

func renderZones(data Data, serial int64) map[string]string {
	contents := make(map[string]string, len(data.DNSZones))
	master := MustServer(Master)
	standby := MustServer(Standby)
	for _, zone := range data.DNSZones {
		contents[zone] = renderZone(zone, data.F3SHosts, master, standby, serial)
	}
	return contents
}

func renderZone(zone string, f3sHosts []string, master, standby Server, serial int64) string {
	content := mustReadFrontendAsset(filepath.Join("var/nsd/zones/master", zone+".zone.tpl"))
	if zone == "buetow.org" {
		content = replaceLegacyF3SZoneLoop(content, renderF3SZoneRecords(f3sHosts, master, standby))
	}
	replacer := strings.NewReplacer(
		"<%= time() %>", strconv.FormatInt(serial, 10),
		"<%= $ips->{current_master}{ipv4} %>", master.IPv4,
		"<%= $ips->{current_master}{ipv6} %>", master.IPv6,
		"<%= $ips->{current_standby}{ipv4} %>", standby.IPv4,
		"<%= $ips->{current_standby}{ipv6} %>", standby.IPv6,
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

func renderF3SZoneRecords(hosts []string, master, standby Server) string {
	var builder strings.Builder
	for _, host := range hosts {
		if !strings.HasPrefix(host, "ipv6.") {
			appendf(&builder, "%s.         300 IN A %s ; Enable failover\nwww.%s.     300 IN A %s ; Enable failover\nstandby.%s. 300 IN A %s ; Enable failover\n", host, master.IPv4, host, master.IPv4, host, standby.IPv4)
		}
		if !strings.HasPrefix(host, "ipv4.") {
			appendf(&builder, "%s.         300 IN AAAA %s ; Enable failover\nwww.%s.     300 IN AAAA %s ; Enable failover\nstandby.%s. 300 IN AAAA %s ; Enable failover\n", host, master.IPv6, host, master.IPv6, host, standby.IPv6)
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
