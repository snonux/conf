package frontends

// Certificate kinds of an acmeCertificate. They are rendered into acme.sh,
// which keys its consumer reload policy off them, so the values are part of
// the script's contract.
const (
	// acmeSite is a public site certificate, with www. and any ipv4./ipv6.
	// single-family names as alternative names. relayd serves it.
	acmeSite = "site"
	// acmeStandby is the standby.<site> certificate. relayd serves it; the
	// script first removes the pre-Gonf symlinked standby files.
	acmeStandby = "standby"
	// acmeHost is the frontend's own FQDN certificate, without www./standby
	// variants. relayd and smtpd use it.
	acmeHost = "host"
)

// acmeTemplateData feeds acme-client.conf.tmpl and acme.sh.tmpl. Both render
// the same Certificates list, so the script requests exactly the domains the
// client configuration declares, in the same order.
type acmeTemplateData struct {
	Certificates []acmeCertificate
}

// acmeCertificate is one acme-client "domain" entry and one certificate the
// renewal script requests under that name.
type acmeCertificate struct {
	Name             string
	Kind             string
	AlternativeNames []string
}

// acmeData returns the certificate list for server: every site from the ACME
// host topology with its standby twin, then the server's own FQDN.
func acmeData(server Server) acmeTemplateData {
	var certificates []acmeCertificate
	for _, site := range acmeSites(Sites()) {
		certificates = append(certificates, site, acmeCertificate{Name: "standby." + site.Name, Kind: acmeStandby})
	}
	certificates = append(certificates, acmeCertificate{Name: server.FQDN, Kind: acmeHost})
	return acmeTemplateData{Certificates: certificates}
}

// acmeSites returns the site certificates among sites. The frontend FQDNs
// are host certificates, requested only by their own frontend, and an
// ipv4./ipv6. name (Site.Family) is not a certificate of its own but an
// alternative name of its parent site's certificate: acme-client has no
// domain entry for it, so requesting it by name could only fail. Relayd's
// keypair list (web.go) is derived from the same sites.
func acmeSites(sites []Site) []acmeCertificate {
	certificates := make([]acmeCertificate, 0, len(sites))
	for _, site := range sites {
		if site.FrontendHost || site.Family != 0 {
			continue
		}
		alternativeNames := []string{"www." + site.Name}
		for _, candidate := range sites {
			if candidate.Name == "ipv4."+site.Name || candidate.Name == "ipv6."+site.Name {
				alternativeNames = append(alternativeNames, candidate.Name)
			}
		}
		certificates = append(certificates, acmeCertificate{Name: site.Name, Kind: acmeSite, AlternativeNames: alternativeNames})
	}
	return certificates
}
