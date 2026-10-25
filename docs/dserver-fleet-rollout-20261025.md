# Remaining DTail fleet rollout — 2026-10-25

This is a preparation record. No remaining-fleet mutation has yet run. The owner
approved BSD identity-preserving cache/config hardening and dserver-only restarts,
and Linux recovery first followed by new DTail identities/client trust if recovery
fails. System SSH identities and authentication are outside scope. F3 is already
accepted under 3n3/mp3 and is excluded. Deployment waits for the Pi3 maintenance
lane; FreeBSD package migration needs the separately submitted extension answer.

## Fresh read-only findings

The isolated checkout starts at published `6738ecd`, retaining the reviewed DTail
integration and later accepted changes. Its local fleet plan has 110 operations.
Original core R1 and final integration review remain the two terminal reviews;
this operation does not repeat them.

Both OpenBSD frontends, f0–f2 and pi0/pi1 run dserver with daemon-owned 0755
runtime/cache and wildcard binding. All seven persistent keys exist as regular,
single-link 0600 files under 0700 daemon-owned `/var/db/dserver`. Their live public
keys, read through strict administrator SSH, match existing controller trust.
No private key was copied or transferred. The FreeBSD binaries all match
`3f146b14d293df15ff019817e30f6be4f3bbf88f2bc60e5ca3bd4683cb5f8573`.

F0–f2 still have `dtail-4.3.2-ng`, own the live config as an ordinary package file,
and have no package locks. Revision 1 remains unpublished in the normal package
repository. A scoped config-only hardening can retain this installed version,
with root-private backups and no package operation, but does not remove the
future package-ownership risk. Normal repository publication must remain gated.
The concrete durable alternative is the reviewed revision-1 local artifact from
mp3, installed with RC/script handling disabled and explicit exact config restore
before startup. That package extension has been submitted for approval; do not
install it before the answer. No package lock is inferred or added.

R0–r2 have inactive, disabled dserver services, no configured volatile key and no
persistent key directory. A bounded metadata search in `/var/lib/dserver`,
`/var/run/dserver`, `/etc/dserver`, `/var/backups`, `/home`, `/root` and `/opt` found
no `ssh_host_key`. This is a search of conventional on-host recovery locations,
not proof that no historical backup anywhere could contain a key.

All three have these root-owned regular 0644 legacy drop-ins:

| File under `/etc/systemd/system/dserver.service.d` | SHA256 |
| --- | --- |
| `runtime-dir.conf` | `67114ed5a8037539c8ba167bbfcc76128f01e3f7bb3c5bd8067a9e1d4f682a02` |
| `runtime-dir-workaround.conf` | `ea71d20cdce1e76d8fb0209090952ce5b6646ce0823bf3bbdc4c6bf9f0d0d2d5` |

Their effective `ExecStartPre` includes recursive daemon-ownership changes to
`/run/dserver`. Leaving them in place would undo the protected cache after the
new base unit's safe pre-start helper. The scoped migration must validate these
exact bytes and metadata, then rename only these files into a root-private
same-filesystem backup. Preserve any unrelated drop-ins. After daemon-reload,
require the effective unit to use `/var/lib/dserver`, `StateDirectory=dserver`,
no runtime-directory daemon ownership, and exactly the preserved-key check plus
safe cache helper as pre-start commands. Unexpected extra commands stop rollout.

Initial short Pi2/Pi3 SSH probes did not establish state. Pi3 is in separate
maintenance and is not probed again until that worker hands off. A short SSH
failure is not evidence that either host is down; prior Pi3 authentication is slow.

## Serial execution and acceptance

1. Once the lane is released, take native strict previews for each scoped task.
   Preserve two administrator connections, record updater state, and verify the
   current public pin again immediately before migration.
2. Use fishfinger, pi0 and f0 as OS canaries. Back up root-controlled config,
   helper and rc files privately. Stop only dserver, quarantine the old runtime
   by same-filesystem rename without recursively repairing it, then apply the
   scoped task. Keep the persistent identity unchanged. The host-specific JSON
   and safe helper may recreate protected directories; native startup recreation
   is then checked separately with the already protected runtime absent.
3. On frontends use `frontends_d_tail_hardening` and config-only
   `frontends_d_tail_checks`; compare the complete live/rendered Gogios objects
   and require only the two intended DTail WG target changes. WG monitoring must
   be healthy before wm3 closes public 2222. No PF action belongs to this task.
4. If FreeBSD package migration is approved, verify the exact mp3 artifact,
   preserve configuration before installing with script/RC handling disabled,
   restore the saved config byte for byte, then apply the scoped hardening.
   No repository refresh/publication or unrelated package is included.
5. Linux pauses only its key-cache timer during migration, records its previous
   enablement, preserves old runtime/log state, and quarantines only validated
   obsolete drop-ins. Recover an existing key if available; otherwise provision
   an approved new RSA DTail key host-locally in an exclusive root-private stage,
   validate its public key, then hand off 0600 key/0700 persistent directory to
   dserver. Update only corresponding port-2222 DTail trust entries after
   administrator-attested public-pin verification. Never touch system SSH keys.
6. After every host, require listener restriction, pinned dcat/dgrep real-log
   content matched against a stable native prefix, readable nonlog policy denials,
   wrong-pin rejection, daemon-readable 0640 cache with denied cache writes, two
   safe refreshes, same identity across startup recreation, administrative access,
   service/timer health and a converged preview. Only then continue to its peer.

Service starts use a detached launcher and an independently bounded observer.
Never wrap FreeBSD daemon startup in default `timeout`, which can reap the daemon.
An observer timeout triggers read-only inspection, never an automatic restart.
Actual log payloads stay in memory; receipts retain only lengths, hashes and
structural assertions. Root-private backups and quarantines are retained.

## NetBSD canary executed

Pi0's approved scoped deployment applied five changes across 17 recorded
operations. Its original trusted public identity was preserved. Only
`192.168.1.125:2222` listens, runtime/cache are root:dserver 0750, and the paul
cache is root:dserver 0640. Current dcat and dgrep each returned 184,354 bytes with
stable native 4096-byte prefix matches. OS-readable fstab and a synthetic nonlog
file were denied with client exit 2 and one new native policy-denial record each;
a wrong pin failed with exit 1. The daemon reads its cache but cannot create a
symlink there. A root-private sentinel survived two refreshes unchanged.

A separate native start recreated deliberately absent protected runtime/cache
with a distinct runtime inode and the same identity. The detached start completed
successfully; all content, denial, cache and pin checks passed again afterward.
The final native preview has no file/config/service changes; only the expected
unconditional cache refresh would run. Private backups/quarantines remain on Pi0.

## Frontend compatibility finding

Both frontends' current binary is
`e06fcbd4a4e39325317f0754320f4897288fd2f28898b8f0b0220301bf4b7a94`,
Go 1.26.2, CGO enabled, module `github.com/mimecast/dtail` without embedded revision.
Both authenticate the current client but return zero log bytes despite a
nonempty OS-readable log. This is not accepted as successful log access.

A read-only copy contains the historical `handleAckUserCommand` and
`shouldShutdownOnCommandCompletion` symbols. Disassembly shows the acknowledgement
handler calling `handleAckCommand` followed by its completion callback, matching
the mechanism already reproduced under kp3. The retained “turbo” log wording is
not causal evidence: current code intentionally retains that wording too.
Configuration hardening cannot remove the old dispatcher behavior. A distinct
package artifact is being prepared for a separate, concrete approval; no frontend
binary/config/service/package change has been executed.

Pi1 subsequently passed the same scoped migration and native startup-recreation
sequence, preserving its trusted identity and listening only at
`192.168.1.126:2222`. Dcat/dgrep returned 101,613 bytes with stable native prefix
matches, and all denial/cache/pin checks passed. Both NetBSD hosts' final previews
show only the unconditional refresh; fresh admin access passed after their owned
control sessions were closed.

## Linux runtime execution

R0, r1 and r2 retained their installed modern executable, all SHA256
`ea99e3fdb99217fad7a113df76b0ddaf8b31a714ba0378914eb76938455c785b`.
This exactly matches the retained binary used in kp3's successful current-client
1.2 MB payload fixture; no rnode package change was needed. Their approved new
3072-bit RSA DTail keys were generated only on-host after the bounded recovery
search found no key. Private key material never left its host. Exact validated
legacy drop-ins were quarantined, and effective-unit assertions ran before start.

All three passed native startup recreation and current dcat/dgrep content checks
against stable OS prefixes (611,116 / 101,158 / 290,330 bytes respectively), readable
nonlog policy denials, wrong-pin rejection, protected cache read/write checks,
unchanged root sentinels and safe refreshes. Their listeners are only the intended
LAN addresses. The reviewed service resource enabled and started their previously
disabled/inactive DTail services. Key-cache timers returned to their prior active,
enabled state, k3s remained active, final previews converged, and fresh administrator
access passed after closing owned control sessions.

Only each host's port-2222 DTail known_hosts aliases were replaced, preserving
all unrelated entries and mixed-line aliases. Private local backups remain.
Ordinary clients using global known_hosts then passed real-log/native-prefix
checks, not merely the temporary pinned test client. Four synthetic trust-update
fixtures cover plain, mixed, hashed and marked-entry refusal behavior.

Linux's systemd PrivateTmp makes a host-created `/tmp` probe invisible to the
service. The corrected denial probe lives in root-controlled shared `/run`, is
OS-readable, and must cause a counted native policy denial. Denial-log writes
are asynchronous; the observer waits at most 15 seconds for the one new record,
without repeating the request or restarting the service. These were acceptance
harness corrections, not production daemon repairs.

Pi3's existing arm64 executable is
`015eae7939924ba7a9f499acc83c3584b18466aa5c58d14c1506952eab208cb5`,
with the modern command dispatcher, Go 1.26.8/nozstd and x/crypto 0.57.0.
Its approved new persistent identity, protected cache and LAN-only
`192.168.1.128:2222` passed startup recreation and repeated real-content checks
(374,555 bytes), policy denials, wrong pins, safe refreshes, global client trust,
converged preview and fresh administrator access. Its cache timer was restored;
no uptime/uploader setting or service was changed. Owned admin channels closed.
The generic runner's console label mentioned k3s, but Pi3's actual checked services
were only DTail and its cache timer; no k3s-on-Pi claim is made.

On Pi3, the original dgrep `.` oracle disagreed only because the client retained
blank lines while native grep removed them. Comparing line lengths/indexes without
retaining payload isolated that mismatch. Both filters were changed to `^`, which
matches every line including blanks; stable native prefixes then passed before
and after startup. A trailing blank line from the held-shell observer also needed
stripping before parsing a final digest. Neither harness correction restarted
DTail or reprovisioned its identity.

## FreeBSD package extension (2026-10-25)

Owner-approved `dtail-4.3.2-ng_1` (SHA256
`5a66d78126b2b1f46ac14e4b3ec211ec0d5bd15356263185765cbf9ab11ed97f`) was installed
serially on f0, then f1, then f2 with RC/script handling disabled and byte-for-byte
config restore before scoped `freebsd_dtail_config`. Each host preserved its trusted
public identity, listens only on its LAN `:2222`, passed native content/denial/
wrong-pin and startup-recreation acceptance, and converged with only the expected
unconditional cache refresh remaining. Private backups/quarantines remain on-host.

## OpenBSD compatibility package (2026-10-25)

Owner-approved `dtail-4.3.2p1-ng` (SHA256
`bbe76c1e1a9fe7678ca9123c8045f71c3832dd3cf3cc4e71b110007f94ba95bf`) was installed
on fishfinger then blowfish using `PKG_PATH` restricted to the private artifact
directory and per-command `-D unsigned`. Live configs were restored before
`frontends_d_tail_hardening`. Both now listen only on their WireGuard addresses
(`192.168.2.111` / `192.168.2.110`); public TCP 2222 is closed. Fingerprints were
preserved. Client acceptance used SSH local-forwards over admin sessions because
the laptop has no F3S WireGuard route to those addresses; that is not claimed as a
direct device-WG packet test. `_dserver` cannot `su` (invalid `nologin` class);
daemon read/write checks used perl POSIX privilege drop instead.

## Gogios WireGuard DTail targets (2026-10-25)

Both frontends' `/etc/gogios.json` keep 572 checks at `0640 root:_gogios`. Only the
two DTail Args now target `*.wg0.wan.buetow.org:2222`. An initial
`frontends_d_tail_checks` apply from the fleet worktree also injected eleven
undeployed `eye.f3s.*` checks; those were reverted immediately so this task does
not publish unrelated inventory. Native `dtailhealth` against both WG targets
returns OK.

## Pi2 Linux completion (2026-10-25)

Pi2 received an approved new on-host RSA DTail identity after the bounded recovery
search found none. Legacy ownership drop-ins were quarantined. The managed unit
now sets `TimeoutStartSec=600` because each `su`-to-user cache read hits ~2 minutes
of `pam_systemd` latency on this host (default 90s start timed out once).

Accepted: LAN-only `192.168.1.127:2222`, fingerprint
`SHA256:v7s1bGtbQ8rzyf8qouQRKYuR7QXNYkyAEv+P+kvSPxM`, protected cache, dcat/dgrep
251216 B with stable native prefixes, policy denials, wrong-pin rejection,
startup recreation with a distinct runtime inode, key-cache timer restored,
controller port-2222 trust updated, global known_hosts client PASS, and
convergence preview with only the expected unconditional refresh/key checks
(no file updates). Fresh admin SSH/sudo after closing owned masters PASS.

## Fleet status

All planned remaining-fleet hosts for this 3n3 scope are accepted: OpenBSD
frontends, FreeBSD f0–f2, NetBSD pi0/pi1, Linux r0–r2/pi2/pi3, plus the earlier
f3 work. Phone/laptop WireGuard device-packet tests were not part of this
acceptance path where SSH local-forwards were used.
