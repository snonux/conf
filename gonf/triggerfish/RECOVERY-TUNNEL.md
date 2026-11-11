# Temporary router management during cable cutover

**Retired 2026-11-11 (j24).** `triggerfish_recovery_retire` (via wg2) removed
the enabled marker, the cron block, the client and `/etc/gonf/router-recovery`;
a stale blowfish sshd session still holding `127.0.0.1:55322` (no router-side
client) was terminated; `frontends_router_recovery_retire` removed the key line
(the other authorized-key bytes are unchanged) and the controller references
`router-recovery.{key,pub,known_hosts}` were deleted. The Prepare/Authorize
tasks below now refuse (the public key reference is gone); this file is
history.

Task k34 closes one specific gap: Earth can use Mars11 and wg0 to reach f0/r0,
but a jump through those hosts still depends on the lab's normal gateway.
Moving that gateway or its cable can interrupt the jump itself.

The router initiates a separate SSH connection from its retained
`192.168.1.2` address directly to the selected public gateway. It requests
**only a gateway loopback listener**, `127.0.0.1:55322`, forwarded to the
router's SSH listener. This introduces no ISP port forward, public listener,
interface, route, PF rule, WireGuard change or second rollback controller.
The gateway data is attached in `cluster.Register()` to triggerfish and
blowfish. Fishfinger has no recovery-key task resources.

The temporary key has a forced failing command and denies agent/X11 forwarding
and PTYs. Forwarding is limited to the fixed reverse listener and loopback
port 1 for local forwarding; verify port 1 has no listener before authorization.
The client uses a dedicated identity and a pinned gateway host key, with no
agent or password fallback. SSH authentication on the inner management
connection remains the existing triggerfish authentication and host key.

## Prepare and start

The controller's ignored file-provider references are:

- `triggerfish/router-recovery.key`: the ephemeral SSH private key;
- `triggerfish/router-recovery.pub`: its public key with owned comment
  `conf-triggerfish-recovery-k34`;
- `triggerfish/router-recovery.known_hosts`: the previously trusted gateway
  ed25519 key, under alias `router-recovery-gateway` (also its port-2 spelling).

Generate these once with owner-only permissions, never print the private key,
and preserve them until retirement. The gateway identity comes from the
controller's existing trusted known-hosts entry, not an unauthenticated scan.
Check that the chosen loopback port is unused and back up the gateway's
authorized keys before applying the explicit operations:

```sh
./gonf.sh push -privilege=doas -- -p 2 rex@blowfish.buetow.org frontends_router_recovery_authorize
# Use the currently verified router SSH transport and -privilege=doas:
./gonf.sh push -privilege=doas -- [SSH options] paul@ROUTER triggerfish_recovery_prepare
./gonf.sh push -privilege=doas -- [SSH options] paul@ROUTER triggerfish_recovery_start
```

Preparation writes only `/etc/gonf/router-recovery/`; it does not enable the
tunnel. Start creates its enabled marker and a separately named root cron job.
The helper checks its private SSH control socket before starting a client;
the fixed reverse listener prevents duplicate successful clients. SSH detects
broken transport, and root cron retries once per minute. The cron entry also
provides reconnection after boot once port0 is available. **Actual router reboot
acceptance is separate; a persistent cron entry alone is not a boot test.**

The client log and control socket are under the root-private directory.
The link may briefly disappear during PF-state flush or physical repatching;
poll its existing client and reconnect job rather than launching competing
clients or changing the normal network configuration.

## Verify and use

From Earth, use the public gateway as a jump to its loopback listener, retaining
the router's existing host-key alias:

```sh
ssh -p 55322 -o HostKeyAlias=triggerfish.lan \
  -o 'ProxyCommand=ssh -4 -o ForwardAgent=no -W %h:%p rex@blowfish.buetow.org' \
  paul@127.0.0.1
```

Verify the authenticated router identity and inner SSH tuple, the router's
public-gateway route through igc0, the outbound source `192.168.1.2`, and the
gateway listener's loopback-only bind. Prove a reconnect using the owned control
socket and the existing cron job. Negative checks must reject a shell command,
an unrelated forwarding destination and a different reverse-listener port.
Existing authorized keys and root cron lines must be preserved exactly.
The explicit `triggerfish_recovery_interrupt` task stops just this client while
leaving its enabled marker and cron entry intact. Deliver it through a separate
accepted transport, then observe the listener disappear and return under cron.

This is a router management path. It does not prove physical isolation,
JetKVM DHCP acquisition, direct wg2 access from mobile internet or application
acceptance. The cable actions remain explicit human gates. During current
preparation even igc0's physical cable still traverses the lab switch; after
the confirmed repatch it goes directly to the ISP router.

## Retire after final topology acceptance

The helper uses base Perl's advisory file lock to serialize Start and Stop.
Start rechecks the enabled marker under that lock; Stop waits for an in-flight
Start, failing after 60 seconds rather than letting gonf delete credentials
while startup is still in progress. The daemonized SSH client does not inherit
the lock. A bounded native cleanup/restart canary is required for acceptance.

Offline regression checks exercise the production helper with real Perl
processes and advisory locks, rewriting only its paths, SSH transport and
timeout in a private fixture. The fixture timeout is one second; production
uses 60 seconds. Actual client/server and cron behavior is checked natively.

```sh
PYTHONDONTWRITEBYTECODE=1 python3 f3s/device-recovery/router-tunnel/check-offline.py -v
```

Task j24 owns retirement. First run `triggerfish_recovery_retire` through a
separate accepted management path. It disables the marker and reconnect job,
stops the client by its owned control socket, then removes only its own files.
The empty private directory is removed without pruning unexpected contents.
Next run `frontends_router_recovery_retire` on blowfish to remove the exact
owned authorized-key line. Verify no listener or reconnect job remains, all
pre-existing authorized-key bytes and unrelated cron lines are unchanged, and
normal management still works. Then delete the ignored controller key material.
Do not delete the controller public-key reference before removing the gateway
line, and do not restore an old whole authorized-keys file over later changes.
