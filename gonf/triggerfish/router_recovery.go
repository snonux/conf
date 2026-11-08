package triggerfish

import (
	"fmt"

	"github.com/snonux/conf/gonf/paths"
	"github.com/snonux/conf/gonf/routerrecovery"
	. "github.com/snonux/gonf/api"
)

// RecoveryTunnel maintains temporary router access across the lab cable gap.
type RecoveryTunnel struct{ RequiresRoot }

// OptsPrepare keeps temporary SSH credentials out of routine maintenance.
func (RecoveryTunnel) OptsPrepare() TaskOptions { return TaskOptions{Operational()} }

// Prepare installs the private SSH client and its pinned public gateway identity.
func (RecoveryTunnel) Prepare() {
	EachHost(func(g routerrecovery.Gateway) {
		const d = routerrecovery.Directory
		EnsureDir(d, RootPrivate)
		File(d+"/identity", WithContent(MustSecret(paths.TriggerfishSecret("router-recovery.key"))), RootPrivate, WithSensitive)
		File(d+"/known_hosts", WithContent(MustSecret(paths.TriggerfishSecret("router-recovery.known_hosts"))), RootPrivate)
		File(d+"/ssh_config", WithContent(fmt.Sprintf(`Host gateway
    HostName %s
    User %s
    Port %d
    AddressFamily inet
    BindAddress 192.168.1.2
    IdentityFile %s/identity
    IdentityAgent none
    IdentitiesOnly yes
    BatchMode yes
    HostKeyAlias router-recovery-gateway
    HostKeyAlgorithms ssh-ed25519
    UserKnownHostsFile %s/known_hosts
    GlobalKnownHostsFile /dev/null
    StrictHostKeyChecking yes
    ForwardAgent no
    ForwardX11 no
    RequestTTY no
    ControlPersist no
    ExitOnForwardFailure yes
    ServerAliveInterval 15
    ServerAliveCountMax 3
    ConnectTimeout 10
    ConnectionAttempts 1
    RemoteForward 127.0.0.1:%d 127.0.0.1:22
`, g.Address, g.User, g.SSHPort, d, d, g.ListenPort)), RootPrivate,
			WithValidation("/usr/bin/ssh", List("-G", "-F", CandidatePath, "gateway")))
		InstallFile(d+"/run", paths.GonfAsset("triggerfish", "router-recovery-tunnel"), RootExec,
			WithValidation("/bin/ksh", List("-n", CandidatePath)))
	})
}

// OptsStart requires preparation and makes activation a by-name operation.
func (RecoveryTunnel) OptsStart() TaskOptions {
	return TaskOptions{Operational(), Needs(RecoveryTunnel.Prepare)}
}

// Start enables the dedicated reconnect job and starts the outbound tunnel.
func (RecoveryTunnel) Start() {
	const d = routerrecovery.Directory
	enabled := File(d+"/enabled", WithContent("k34\n"), RootPrivate)
	CronAt(routerrecovery.CronName, "* * * * *", d+"/run start", DependsOn(enabled))
	Sh(d+"/run start", DependsOn(enabled))
}

// OptsInterrupt keeps the reconnect canary out of every routine aggregate.
func (RecoveryTunnel) OptsInterrupt() TaskOptions { return TaskOptions{Operational()} }

// Interrupt stops the owned client while leaving its reconnect job enabled.
func (RecoveryTunnel) Interrupt() {
	const d = routerrecovery.Directory
	enabled := Sh("test -f " + d + "/enabled")
	Sh(d+"/run stop", DependsOn(enabled))
}

// OptsRetire keeps tunnel retirement outside every routine aggregate.
func (RecoveryTunnel) OptsRetire() TaskOptions { return TaskOptions{Operational()} }

// Retire disables reconnects, stops the owned SSH process and removes its files.
func (RecoveryTunnel) Retire() {
	const d = routerrecovery.Directory
	disabled := NoFile(d + "/enabled")
	cron := NoCron(routerrecovery.CronName, DependsOn(disabled))
	stop := Sh(d+"/run stop", DependsOn(cron))
	NoFile(List(d+"/identity", d+"/known_hosts", d+"/ssh_config", d+"/run", d+"/client.log", d+"/control"), DependsOn(stop))
}
