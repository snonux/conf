# Temporary router management during cable cutover

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

**k34 completion gate still open:** serialize helper start/stop before relying
on retirement. The current helper has no interprocess lock; a cron invocation
that passed the enabled-marker check can race retirement and start a client
after its control socket has been removed. Do not run retirement until that
race is fixed and a bounded native cleanup/restart canary has passed.

Task j24 owns retirement. First run `triggerfish_recovery_retire` through a
separate accepted management path. It disables the marker and reconnect job,
stops the client by its owned control socket, then removes only its own files.
Next run `frontends_router_recovery_retire` on blowfish to remove the exact
owned authorized-key line. Verify no listener or reconnect job remains, all
pre-existing authorized-key bytes and unrelated cron lines are unchanged, and
normal management still works. Then delete the ignored controller key material.
Do not delete the controller public-key reference before removing the gateway
line, and do not restore an old whole authorized-keys file over later changes.
