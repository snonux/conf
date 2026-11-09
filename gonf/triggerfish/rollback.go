package triggerfish

import (
	"fmt"
	"regexp"

	"github.com/snonux/conf/gonf/paths"
	. "github.com/snonux/gonf/api"
)

const rollbackCommand = "/etc/gonf/router-rollback"

// Rollback installs and operates the separately gated router recovery journal.
// Every operation is explicit; routine router aggregates cannot activate it.
type Rollback struct{ RequiresRoot }

// RouterTrial selects one reviewed canary. Its zero value refuses mutations.
// Token identifies a single trial and is never a password or tunnel key.
type RouterTrial struct {
	Profile       string `json:"profile"`
	Token         string `json:"token"`
	WindowSeconds uint   `json:"window_seconds"`
	// BootCanary stages the fixed removable router-only reboot canary.
	BootCanary bool `json:"boot_canary"`
	// ServicesVerified is supplied by the guarded controller after service checks.
	ServicesVerified bool `json:"services_verified"`
	// ProofRole and SSHConnection are measured by the controller on a fresh
	// authenticated session; the helper verifies its live socket and trial.
	ProofRole     string `json:"proof_role"`
	SSHConnection string `json:"ssh_connection"`
	// ReadinessReviewed records the controller's bounded inventory review.
	// CandidateSHA256 must come from Stage's frozen candidate, never a template.
	ReadinessReviewed bool   `json:"readiness_reviewed"`
	CandidateSHA256   string `json:"candidate_sha256"`
}

// OptsSetup excludes recovery installation from routine deployment.
func (Rollback) OptsSetup() TaskOptions { return TaskOptions{Operational()} }

// Setup installs the durable timer and early boot recovery hook.
func (Rollback) Setup() {
	EnsureDir("/etc/gonf", RootPrivate)
	guard := Sh("test ! -e /etc/gonf/router-state/pending")
	script := InstallFile(rollbackCommand, paths.GonfAsset("triggerfish", "router-rollback"), RootExec,
		WithValidation("/bin/ksh", List("-n", CandidatePath)), DependsOn(guard))
	// The helper preserves unrelated rc text and validates its candidate before
	// replacing rc. A changed upstream anchor is a refusal, never a best guess.
	Sh(rollbackCommand+" install-boot", DependsOn(script))
	CronAt("triggerfish-router-rollback", "* * * * *", rollbackCommand+" tick")
}

// OptsStage excludes canary staging from routine deployment.
func (Rollback) OptsStage() TaskOptions {
	return TaskOptions{Operational(), Needs(Rollback.Setup, Profiles.Candidates)}
}

// Stage validates and freezes a selected inactive profile without applying it.
func (Rollback) Stage() { trialCommand("stage", true) }

// OptsReadiness excludes inventory acceptance from routine deployment.
func (Rollback) OptsReadiness() TaskOptions { return TaskOptions{Operational()} }

// Readiness binds the measured VPN and inventory review to the frozen candidate.
func (Rollback) Readiness() {
	EachHost(func(t RouterTrial) {
		if !t.ReadinessReviewed || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(t.CandidateSHA256) {
			Refuse("File", "router-readiness", fmt.Errorf("reviewed inventory and measured frozen digest required"))
			return
		}
		switch t.Profile {
		case "shared-igc0", "wifi-prepare", "wifi-first", "post-rack", "post-ap", ipv6UplinkProfileName, ipv6NAT66ProfileName:
		default:
			Refuse("File", "router-readiness", fmt.Errorf("physical migration profile required"))
			return
		}
		EnsureDir("/etc/gonf/router-readiness", RootPrivate)
		File("/etc/gonf/router-readiness/"+t.Profile, WithContent(
			"candidate-sha256="+t.CandidateSHA256+"\n"+
				"provisioning-clients=captured-inventory-no-known-clients\n"+
				"dependencies=r14-reviewed-current-stage-only\n"+
				"wg2=provisioned-and-tested\n"+
				"old-management=fresh-authenticated\n"+
				"current-cabling="+t.Profile+"\n"), RootPrivate)
	})
}

// OptsArm excludes arming from routine deployment.
func (Rollback) OptsArm() TaskOptions { return TaskOptions{Operational()} }

// Arm captures saved and running state and arms the independent recovery deadline.
func (Rollback) Arm() { trialCommand("arm", true) }

// OptsActivate excludes activation from routine deployment.
func (Rollback) OptsActivate() TaskOptions { return TaskOptions{Operational()} }

// Activate applies only the frozen, armed and explicitly reviewed trial.
func (Rollback) Activate() { trialCommand("activate", false) }

// OptsRebootCanary excludes the bounded reboot test from routine deployment.
func (Rollback) OptsRebootCanary() TaskOptions { return TaskOptions{Operational()} }

// RebootCanary reboots only a proved, unconfirmed boot canary of a canary-capable profile.
func (Rollback) RebootCanary() {
	trialCommand("reboot", false)
}

// OptsStatus excludes status from routine deployment.
func (Rollback) OptsStatus() TaskOptions { return TaskOptions{Operational()} }

// Status reports the journal and timer without exposing saved configuration.
func (Rollback) Status() { Sh(rollbackCommand + " status") }

// OptsProof excludes connection measurement from routine deployment.
func (Rollback) OptsProof() TaskOptions { return TaskOptions{Operational()} }

// Proof records a fresh authenticated management connection's measured socket.
func (Rollback) Proof() { trialCommand("proof", false) }

// OptsConfirm excludes confirmation from routine deployment.
func (Rollback) OptsConfirm() TaskOptions { return TaskOptions{Operational()} }

// Confirm accepts a trial only after separate fresh management proof receipts.
func (Rollback) Confirm() { trialCommand("confirm", false) }

// OptsRestore excludes explicit restoration from routine deployment.
func (Rollback) OptsRestore() TaskOptions { return TaskOptions{Operational()} }

// Restore queues an independent restore attempt with a private diagnostic log.
func (Rollback) Restore() {
	// Restoring PF flushes the SSH session that delivered this task. Keep the
	// attempt independent of that session; the existing journal/cron still owns
	// retries, and Status must prove completion rather than this launch returning.
	driver := "\"$1\" restore; restore_status=$?; printf 'restore exit=%s\\n' \"$restore_status\"; exit \"$restore_status\""
	Command("sh", List("-c", "umask 077; /usr/bin/nohup /bin/sh -c \"$1\" router-restore \"$2\" </dev/null >/etc/gonf/router-restore.log 2>&1 &", "router-restore", driver, rollbackCommand))
}

// OptsRestorePrevious excludes journal recovery from routine deployment.
func (Rollback) OptsRestorePrevious() TaskOptions { return TaskOptions{Operational()} }

// RestorePrevious recovers a named restored journal after a later stage refused.
func (Rollback) RestorePrevious() {
	EachHost(func(t RouterTrial) {
		if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{7,63}$`).MatchString(t.Token) {
			Refuse("Command", "recover", fmt.Errorf("the previous restored trial token is required"))
			return
		}
		Command("sh", List("-c", "umask 077; /usr/bin/nohup "+rollbackCommand+
			" recover "+t.Token+" </dev/null >/etc/gonf/router-restore.log 2>&1 &"))
	})
}

func trialCommand(action string, profile bool) {
	EachHost(func(t RouterTrial) {
		if action == "reboot" && (!t.BootCanary || !bootCanaryStage(t.Profile)) {
			Refuse("Command", "reboot", fmt.Errorf("explicit boot canary of a canary-capable profile required"))
			return
		}
		if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{7,63}$`).MatchString(t.Token) {
			Refuse("Command", action, fmt.Errorf("a reviewed trial token is required"))
			return
		}
		args := List(action, t.Token)
		if profile {
			switch t.Profile {
			case "vpn-bootstrap", "shared-igc0", "wifi-prepare", "wifi-first", "post-rack", "post-ap", ipv6UplinkProfileName, ipv6NAT66ProfileName:
			default:
				Refuse("Command", action, fmt.Errorf("a retained-recovery migration profile is required"))
				return
			}
			if t.WindowSeconds < 180 || t.WindowSeconds > 1800 {
				Refuse("Command", action, fmt.Errorf("trial window must be 180–1800 seconds"))
				return
			}
			args = append(args, t.Profile, fmt.Sprint(t.WindowSeconds))
		}
		if action == "proof" {
			switch t.ProofRole {
			case "old", "new", "recovery", "vpn":
			default:
				Refuse("Command", action, fmt.Errorf("management proof role is required"))
				return
			}
			args = List(action, t.ProofRole, t.Token, t.SSHConnection)
		}
		if action == "confirm" {
			if !t.ServicesVerified {
				Refuse("Command", action, fmt.Errorf("controller service checks are required"))
				return
			}
			args = append(args, "services-checked")
		}
		Command(rollbackCommand, args)
	})
}
