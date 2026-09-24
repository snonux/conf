// Package tasks wires recipe groups into the gonf CLI composition root.
package tasks

import (
	"fmt"
	"sort"
	"strings"

	"codeberg.org/snonux/conf/gonf/cluster"
	"codeberg.org/snonux/conf/gonf/freebsd"
	"codeberg.org/snonux/conf/gonf/frontends"
	"codeberg.org/snonux/conf/gonf/garage"
	"codeberg.org/snonux/conf/gonf/netbsd"
	"codeberg.org/snonux/conf/gonf/openbsd"
	"codeberg.org/snonux/conf/gonf/rnodes"
	"codeberg.org/snonux/conf/gonf/rocky"
	. "github.com/snonux/gonf/api"
	"github.com/snonux/gonf/resource"
)

// frontendTaskPrefix is the RegisterMethods prefix shared by every frontend
// task; checkFrontendMembership classifies each task carrying it.
const frontendTaskPrefix = "frontends_"

// frontendSetupTasks returns the explicit membership of the frontends setup
// aggregate, in recording order (alphabetical, the order the former regex
// recorded it in, so the aggregate's plan is unchanged). It deliberately
// leaves out the operational actions in frontendExcludedTasks. A new
// frontends_* task must be added either here (to join the aggregate) or to
// frontendExcludedTasks; checkFrontendMembership reports a declaration
// error at registration otherwise, so a new task can never be silently left
// out of setup runs.
// AggregateTasks additionally fails the record on a member name that is not
// registered, so a typo cannot silently shrink a setup run either.
func frontendSetupTasks() []string {
	return []string{
		"frontends_acme",
		"frontends_base",
		"frontends_cron",
		"frontends_d_tail",
		"frontends_dns_failover",
		"frontends_foostats",
		"frontends_gemtexter",
		"frontends_gogios",
		"frontends_goprecords",
		"frontends_httpd",
		"frontends_inetd",
		"frontends_myname",
		"frontends_newsyslog",
		"frontends_nsd",
		"frontends_pf",
		"frontends_pkg_repo",
		"frontends_relayd",
		"frontends_rsync",
		"frontends_script",
		"frontends_service_accounts",
		"frontends_services",
		"frontends_smtpd",
		"frontends_uptimed",
		"frontends_wire_guard_hosts",
	}
}

// frontendExcludedTasks returns the frontends_* tasks that deliberately stay
// out of the frontends aggregate: frontends_acme_invoke (requesting
// certificates), frontends_irc_bouncer (the existing ZNC deployment) and
// frontends_ping (the push-pipeline diagnostic, excluded on the owner's
// decision of 2026-09-22 so diagnostics stay separate from setup). All three
// are marked Operational (see their OptsX companions) and stay explicit,
// by-name actions.
func frontendExcludedTasks() []string {
	return []string{
		"frontends_acme_invoke",
		"frontends_irc_bouncer",
		"frontends_ping",
	}
}

// Register makes every current recipe group and its deployment aggregate
// available to the CLI. New recipe groups (Rex, the former engine, was
// retired in conf task v42) join this composition root rather than
// extending main directly.
func Register() {
	RegisterMethods(openbsd.Unattended{}, WithPrefix("frontends_"), WithCluster(cluster.NameFrontends))
	RegisterMethods(frontends.Maintenance{}, WithPrefix("frontends_"), WithCluster(cluster.NameFrontends))
	RegisterMethods(frontends.Monitoring{}, WithPrefix("frontends_"), WithCluster(cluster.NameFrontends))
	RegisterMethods(frontends.Web{}, WithPrefix("frontends_"), WithCluster(cluster.NameFrontends))
	RegisterMethods(frontends.MailDNS{}, WithPrefix("frontends_"), WithCluster(cluster.NameFrontends))
	RegisterMethods(netbsd.Unattended{}, WithPrefix("pis_netbsd_"), WithCluster(cluster.NameNetBSDPis))
	RegisterMethods(rocky.Unattended{}, WithPrefix("rocky_"), WithCluster(cluster.NameRockyAll))
	// Only the Pis boot the SIG AltArch kernel dnf updateinfo cannot audit.
	// Its rocky_kernel_audit_ prefix keeps it in the "rocky" aggregate;
	// rocky_kernel_audit_packages installs its own ksh interpreter, so it
	// also deploys on its own.
	RegisterMethods(rocky.KernelAudit{}, WithPrefix("rocky_kernel_audit_"), WithCluster(cluster.NameRockyPis))
	RegisterMethods(rnodes.Maintenance{}, WithPrefix("rnodes_"), WithCluster(cluster.NameRockyK3s))
	RegisterMethods(freebsd.Unattended{}, WithPrefix("freebsd_"), WithCluster(cluster.NameFreeBSD))
	RegisterMethods(garage.Deployment{}, WithPrefix("garage_"), WithCluster(cluster.NameGarage))

	// The frontends aggregate lists its setup members explicitly (see
	// frontendSetupTasks): ACME invocation is excluded because setup only
	// installs its config and daily hook, while requesting certificates
	// remains an explicit action; the IRC bouncer and the ping diagnostic
	// likewise stay by-name.
	// checkFrontendMembership then refuses any frontends_* task that is
	// neither a member nor explicitly excluded.
	AggregateTasks("frontends", "Install all frontend configuration except explicit ACME invocation", frontendSetupTasks()...)
	checkFrontendMembership()
	Aggregate("pis_netbsd", "Install all pis_netbsd_* configuration", "^pis_netbsd_")
	Aggregate("rocky", "Install all rocky_* configuration", "^rocky_")
	Aggregate("rocky_kernel_audit", "Install the Rocky Pi kernel CVE audit", "^rocky_kernel_audit_")
	Aggregate("rnodes", "Install all rnodes_* configuration", "^rnodes_")
	Aggregate("freebsd", "Install all freebsd_* configuration", "^freebsd_")
	Aggregate("garage", "Install all Garage configuration", "^garage_")
}

// checkFrontendMembership fails the invocation unless every registered
// frontends_* task is classified exactly once: listed in frontendSetupTasks
// or in frontendExcludedTasks. The aggregate itself is named "frontends" and
// so carries no prefix. Stale entries (a classified name that is no longer
// registered) fail too, so the lists cannot drift from the registrations.
//
// A misclassification is recipe misuse, so it is reported as a gonf
// declaration error through resource.Refuse (Task[frontends], the aggregate
// whose membership is wrong) instead of panicking. Register runs from main
// before the CLI, so gonf's CLI then refuses every invocation, -list
// included, printing the error and exiting 1: a forgotten task still fails
// loudly instead of being silently left out of setup runs, just without the
// stack trace a panic dumped.
//
// Tasks() activates the registry with the controller's facts; the CLI
// re-activates it after flag parsing (-profile), so calling it here changes
// nothing. It lists active tasks only: no frontend task has a When
// predicate today, so all of them are active. Adding one would make the
// stale check fire on controllers where it is inactive, loudly, not silently.
func checkFrontendMembership() {
	if err := frontendMembershipError(); err != nil {
		resource.Refuse("Task", "frontends", err)
	}
}

// frontendMembershipError returns the first frontends aggregate
// classification problem (see checkFrontendMembership), or nil.
func frontendMembershipError() error {
	classified, err := classifiedFrontendTasks()
	if err != nil {
		return err
	}
	var unclassified []string
	registered := map[string]bool{}
	for _, t := range Tasks() {
		if !strings.HasPrefix(t.Name, frontendTaskPrefix) {
			continue
		}
		registered[t.Name] = true
		if !classified[t.Name] {
			unclassified = append(unclassified, t.Name)
		}
	}
	if len(unclassified) > 0 {
		return fmt.Errorf("frontends aggregate: add %s to frontendSetupTasks or frontendExcludedTasks in gonf/tasks/tasks.go",
			strings.Join(unclassified, ", "))
	}
	var stale []string
	for name := range classified {
		if !registered[name] {
			stale = append(stale, name)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		return fmt.Errorf("frontends aggregate: %s listed in gonf/tasks/tasks.go but not registered",
			strings.Join(stale, ", "))
	}
	return nil
}

// classifiedFrontendTasks returns the set of names listed in
// frontendSetupTasks and frontendExcludedTasks, or an error when a name is
// listed twice (in both lists, or twice in one).
func classifiedFrontendTasks() (map[string]bool, error) {
	classified := map[string]bool{}
	for _, name := range append(frontendSetupTasks(), frontendExcludedTasks()...) {
		if classified[name] {
			return nil, fmt.Errorf("frontends aggregate: %q is both a member and excluded, or listed twice", name)
		}
		classified[name] = true
	}
	return classified, nil
}
