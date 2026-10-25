# Remaining sm3 proposal — 2026-10-25

Owner approved both remaining proposals on 2026-10-25: the existing
IPv4/ULA-only Pi firewall (Pi3 then Pi2, explicitly denying GUA-source and
link-local services), and serial r0/r1/r2 rpcbind cleanup with timed rollback.
Execution preparation is rebased onto conf `5ec0fa3`. Earlier r-node firewall
approvals did not cover this scope; this is the subsequent explicit approval.
Per-host production-lane release is still required from the parent coordinator
before mutation because DTail and ArgoCD route work are in progress.
No live files were staged, service state changed, rules loaded, or source
pushed during the read-only preparation recorded below.
The reviewed helper and boot unit remain unchanged. The profile now uses
an explicit boolean to preserve the exact approved rendered policy, as
recorded below.

Private read-only receipts are in
`~/.local/state/conf-worktrees/sm3-remaining-prep-20261025/evidence/`.
DTail owns the production lane; coordinate its release and refresh listeners
before any approved operation. Its dserver changes can make these snapshots
stale independently of this proposal. Do not borrow another worker's SSH
transport. Preserve both existing r-node guards and all other firewall tables.

## Pi policy and observed baseline

The existing `pihole_firewall_stage` installs the unchanged `eth0` profile:
`inet f3s_guard`, prerouting priority -150, local destination only. It returns
IPv4 sources `192.168.1.0/24`, IPv6 sources `fd22:c702:acb7::/64`, established
or related traffic, ICMP/ICMPv6 and DHCPv6 replies; other new traffic drops.
There is no WireGuard transport exception on Pis. Existing firewalld retains
its independent decisions. No cockpit removal, firewalld reload, route change,
Pi-hole setting/image change, logind repair or SSH policy change is proposed.

At 16:23 UTC Pi3 has eth0 `192.168.1.128`, ULA
`fd22:c702:acb7::cb8` (also an SLAAC ULA), and GUA
`2a01:5a8:304:bc3:95eb:7ac8:3563:a88c`. Firewalld is active with an nftables
public zone on eth0: SSH, HTTP, cockpit, DHCPv6-client, explicit TCP22/53/2222
and UDP53. Docker uses iptables-nft compatibility tables; its DOCKER NAT
chains have no published-port DNAT. Pi-hole 2026.10.0 is healthy, host network,
zero restarts, started 16:09:26 UTC. TCP22/53/80/443 and UDP53/123 listen on
IPv4 and IPv6. TCP443 listens but firewalld does not currently allow it;
TCP9090/2222 were not listening at this snapshot. Do not treat these baseline
failures as proof of source filtering or require them to become reachable.
No source guard table or boot unit exists on Pi3.

Pi2 metadata at 16:27 UTC confirms the same firewall stack and listener
set, host-network healthy Pi-hole 2026.10.0 with zero restarts (started
October 21 at 21:12:20 UTC), and no source guard/boot unit. Its eth0 address
is `192.168.1.127`, ULA `fd22:c702:acb7::4aa` (plus SLAAC ULA), GUA
`2a01:5a8:304:bc3:e42c:b73:3ac6:193f`. Ordinary SSH plus sudo took about
241 seconds before the first command; allow both PAM stages before arming
rollback. Neither login policy nor logind changed in this preparation.

The actual Go text/template-rendered Pi candidate passed native
`nft -c -f -` on Pi3 at 16:24 UTC and Pi2 at 16:28 UTC over stdin.
Candidate SHA256:
`4276353bd7ef3e92aeaa46975b89d235f0ac5756426566f022f9ad9e90b42df1`. No remote candidate file,
Gonf preview, staging operation or native apply was used. A parser pass is
not runtime acceptance.

At 16:28 UTC, fresh blowfish public-IPv6 connections routed through vio0
succeeded to both Pis on TCP22/53/80; TCP443/9090/2222 failed. Actual UDP DNS
queries already timed out, and TCP DNS queries reset after connecting.
Therefore the change must close the exposed TCP handshakes; it cannot be
credited with newly blocking recursive DNS answers that were absent before.
Use guard counters plus fresh connection attempts for positive deny evidence.

### Explicit resolver/source policy choice

Fresh controller UDP and TCP wildcard queries succeed against both Pis'
IPv4 addresses and both Pis' ULAs and GUAs. The GUA query selects the controller's
GUA source; the ULA query selects its ULA source. The proposed policy will
intentionally stop the previously working GUA-source service path, even
when that client is physically on the LAN. Link-local service sources are
also not trusted. ICMP exception does not imply DNS access.

Controller NetworkManager/systemd-resolved currently uses `192.168.1.1`,
not the older runbook's `.101`. Both Pis have resolver addresses
`192.168.1.1`, `fd22:c702:acb7::1` and `fe80::1`. Pi3's read-only query DB
aggregate for the preceding 24 hours contained 102 queries from two LAN IPv4
clients and 42 loopback queries. Pi2's persisted preceding-24h aggregate at
16:28 UTC had 386 queries from two LAN IPv4 clients and 56 loopback queries,
with no GUA category observed. Persisted DB data may lag live queries. A finite log window does not establish every household device's DNS
settings, IPv6 RDNSS advertisement, or upstream forwarding path.

Recommended owner choice: accept IPv4/ULA-only Pi services using this
existing reviewed policy, with `.127/.128` or the Pis' current ULAs as DNS
endpoints. The observed working IPv4/ULA paths and finite client aggregates
support that choice; a household-wide device inventory is not required.
Approval must explicitly acknowledge that GUA-source and link-local service
access will be denied, including a LAN client querying a Pi's global address.
If legitimate LAN global-IPv6 service access must remain, defer activation
and review an alternative source policy instead. Do not silently trust the
ISP global prefix, administer the ISP router, change the AP, or alter client
routes. During rollout, preserve the verified IPv4/ULA paths and investigate
any concrete affected-client failure before confirmation.
Actual phone/laptop WireGuard tunnel checks remain outstanding; LAN laptop
DNS and frontend mesh checks do not replace them.

## Proposed Pi rollout after explicit approval

Prefer Pi3 first because its accepted logind repair restored prompt access.
Pi2 remains the untouched serving resolver. Pi2 may follow only after Pi3
acceptance and a fresh Pi2 baseline; never activate both simultaneously.

1. Confirm the owner selected the IPv4/ULA-only policy above, then refresh
   current addresses, listener/firewalld/
   Docker policy, container identity/health/restarts and normal monitoring.
   Check UDP/TCP public DNS, wildcard/apex VIP, blocking and NXDOMAIN through
   both healthy Pis. Retain private full ruleset and permanent/runtime
   firewalld snapshots as evidence, not as a ruleset-wide restore script.
2. Open two independent ordinary `paul` SSH sessions with sudo, including
   all slow PAM work before the timer. Retain the initialized privileged
   command channels so subsequent helper calls do not open another sudo PAM
   session. On Pi2 allow roughly 241 seconds for SSH plus sudo before arming.
   Run fresh SSH-only IPv4/ULA authentication in parallel with other gates,
   with a 150-second bound; preserve the 240-second confirmation cutoff.
   Fresh normal SSH plus sudo is a post-confirmation gate bounded at
   330 seconds, with helper revert available on failure. Rehearse these
   timings read-only before staging; abort if fresh SSH cannot meet the
   guarded deadline. No PAM/logind repair is in this scope.
3. Run one-host destination Gonf strict stage preview, then
   `./gonf.sh push -privilege=sudo -- -p22 -oBatchMode=yes paul@pi3.lan.buetow.org pihole_firewall_stage`.
   Preview must show only candidate/helper/unit/directories, restorecon and
   daemon-reload. Verify exact reviewed hashes/root modes and no table or
   boot enablement before calling the helper.
4. Inside the already-initialized sudo-granted shell, channel A calls
   `/usr/local/sbin/f3s-firewall check`, then helper `apply` with its recorded
   original SSH connection tuple. Do not invoke a new sudo session here.
   Verify the real PID1 five-minute rollback timer is armed. The helper
   snapshots only its own table/boot policy. Keep channel B initialized.
5. During the guarded window require fresh IPv4 and ULA SSH, UDP/TCP DNS
   (public, wildcard/apex, blocked, NXDOMAIN), HTTP admin HTML and existing
   unauthenticated API response from trusted LAN/ULA sources. Probe the
   untouched partner concurrently. From blowfish's actual public IPv6 path,
   fresh TCP22/53/80 handshakes must fail and UDP/TCP DNS must return no
   service response; the guard drop counter must rise. UDP DNS was already
   unanswered publicly before activation, so a timeout alone proves nothing. Probe TCP443/9090/2222 separately but preserve their measured
   pre-change status. Refresh dserver expectations after DTail rollout.
   Verify no container restart and no unrelated nft policy changes.
6. Confirm from channel B only after every required gate passes and before
   240 seconds; otherwise run helper `rollback` from B and stop. Automatic
   five-minute rollback remains independent of the controller. Verify
   confirmed0600, boot enabled, pending absent, timer inactive and strict
   restaging zero changes. Repeat DNS/SSH/UI checks and wait for ordinary
   fresh healthy monitoring before proceeding to Pi2.

Rollback during the window is the existing helper `rollback`; after
confirmation use its `revert "$SSH_CONNECTION"` from two initialized admin
sessions under the approved reversal scope. Preserve firewalld/Docker and
never restore an entire nft/iptables snapshot. The previous own-table state
is absence at first activation. No reboot, timer-expiry production experiment,
network restart or Pi-hole restart is needed.

## Separate rpcbind proposal

All three r-nodes were inspected at 16:23 UTC. Every host-visible NFS mount,
including kubelet subpaths, is `nfs4,vers=4.2`, through `127.0.0.1:2323`.
An additional read-only `/proc` mount-namespace scan inspected 54 namespaces
on r0, 40 on r1 and 53 on r2, with no vanished-process gaps; every observed
NFS filesystem was nfs4 vers4.2 over the same loopback endpoint. Refresh this
snapshot before stopping anything. Each rpcbind service/socket is enabled and active. `rpcinfo -p localhost`
reports only program100000 (portmapper); no registered statd/mountd/NFS
service. `nfs-client.target` is enabled/active with no Requires; its Wants
include rpc-statd-notify, which is active/exited and does not require
rpcbind. `rpc-statd` is inactive/static; although it requires rpcbind.socket,
it is not running. nfs-server/mountd/idmapd reverse dependencies are inactive.
Stunnel is enabled/active. Both existing source and co3 guards are present
and boot-enabled.

Fresh bounded reads of the existing public NFS package index returned the
same 636 bytes on all three nodes, SHA256
`1f2cf6757f1b42e0dce3d8059e102c8c143e5de0686389cd038c5066778cd933`.
k3s, stunnel and nfs-mount-monitor.timer were active. These observations
support a separate one-node cleanup; they are not proof that stopping
rpcbind has already passed acceptance.

Proposed scope: r0 canary first, then r1 and r2 serially only if included in
explicit approval and each prior host passes. This is independent of the Pi
IPv6 endpoint policy choice. No FreeBSD rpcbind, mount, NFS/stunnel/k3s restart, CARP,
watchdog alteration, package removal or firewall policy change is included.

1. Freshly reject any non-v4 mount, extra RPC registration, active rpc-statd,
   NFS server, or new active dependency. Repeat the workload mount-namespace scan; host `findmnt` alone cannot
   prove an unseen private mount namespace is v4. Refresh pod readiness,
   UID/restarts, NFS consumer health and normal monitoring. Record exact
   service/socket enablement/activity and unit symlinks before action.
2. Hold two authenticated admin sessions. With the observed enabled/active
   baseline unchanged, arm a separately named five-minute PID1 transient
   rollback before cleanup:

   ```sh
   sudo -n systemd-run --unit=sm3-rpcbind-rollback --on-active=5m \
     /bin/bash -euc 'systemctl unmask rpcbind.service rpcbind.socket; systemctl enable rpcbind.service rpcbind.socket; systemctl start rpcbind.socket rpcbind.service'
   ```

   Verify the timer and its exact command. If baseline differs, stop and
   prepare state-specific rollback instead of applying this command.
3. Execute the existing one-host Operational `rnodes_firewall_rpcbind_off`
   task through normal Gonf. Its immediate guard verifies the expected nfs4
   mount and refuses any host-visible `nfs` mount, then invokes
   `systemctl mask --now rpcbind.service rpcbind.socket`. Its guard does not
   replace the dependency/namespace preflight above. No source code change
   is needed; ordinary NFSClientUnits does not re-enable these units.
4. Require both units masked/inactive, TCP/UDP111 and `/run/rpcbind.sock`
   no longer serving, unchanged mount source/options, repeated bounded NFS
   reads and NFS consumer health, unchanged k3s/stunnel PIDs and guards,
   3 Ready nodes, no new workload restarts and healthy monitoring. Check
   access through the existing mount only; never remount or force failover.
   By 240 seconds, cancel the rollback timer only if all immediate gates
   pass; otherwise execute its rollback service and stop the fleet rollout.
   Observe a fresh normal watchdog/monitor cycle before the next host.
5. Preserve receipts of masked state and unchanged ordinary NFS recipe.
   A later approved reversal unmask/enables/starts only these two units,
   restoring the recorded baseline. It does not restart NFS or stunnel.

The transient rollback command has been prepared as text, not installed or
armed. No rpcbind stop/restore or post-stop workload acceptance has run.
Full sm3 remains open, including actual device-side WireGuard checks.


## Destination-render correction before activation — 2026-10-25

After owner approval and Pi3 lane release, native Gonf staging installed the
candidate/helper/unit and reloaded systemd. The controller stopped before
activation because its exact installed-policy hash gate failed. The typed
standalone Go render omitted the optional WireGuard branch, but the actual
Gonf serialized-template renderer treated `json.Number("0")` as truthy and
rendered an unintended `udp dport 0 return` exception. That staged candidate
had SHA256 `9a6d8e2bd2ae834cef754dee732057a6555e27882ef0474bcc5f5ed18d623707`.
It was never applied: native inspection confirmed no source table, pending
state, confirmed policy, rollback timer or boot enablement. Both owned admin
channels were closed; Pi-hole, firewalld and the newly accepted DTail listener
were unchanged.

The narrow correction uses an explicit `EnableWireGuard` boolean for the
conditional; only the r-node profile enables it. The existing numeric port
remains 56709 for r-nodes. This restores the exact approved Pi candidate
SHA256 `4276353bd7ef3e92aeaa46975b89d235f0ac5756426566f022f9ad9e90b42df1`
and preserves the deployed r-node rendered policy SHA256
`794a9c28c4bbfa174eaa7f0edb77e958568e490f701ffc08eede45641ea435e7`.
Helper and boot-unit bytes are unchanged. No library semantic change or
Gonf version upgrade is included; numeric-template behavior is a separate
library follow-up.

`f3s/tests/linux-firewall-render.sh` records the actual two recipe plans,
keeps their serialized template bytes/data, redirects just the candidate
file's destination/ownership to a local temporary directory, and applies
through the real Gonf destination renderer. It asserts both exact policy
hashes and absence of UDP port0. This covers the failed transport/render
path; standalone typed rendering or textual template substitution alone did
not. Native packet/lifecycle suites continue to check rule behavior and
ownership, with only their conditional-token spelling updated.

Private failed-stage/native-state evidence:
`live/pi3-rollout/` and `live/pi3-stage-inspection/` under the worktree parent.
These paths preserve the initial attempt; a corrected rollout records its
own receipts. Corrected native staging must match the approved Pi hash before
any activation. Staging alone is not rollout acceptance.
