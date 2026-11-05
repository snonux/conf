# First endpoint slice: r2 referring to f3 — root review checkpoint

Prepared 2026-11-05 at the pre-installation checkpoint. The initial-state
observations below preceded root's scoped installation/expiry approval;
task y14 annotations record subsequent actual execution. Its prerequisites were
both guest former-deadline observations and FreeBSD's graceful return to normal
stopped state. This is a bounded
canary, with no source promotion or production manifest.

Subsequent native results, including task 134's resolved strict-admission
projection, are recorded in [Y14-ENDPOINT-NATIVE.md](Y14-ENDPOINT-NATIVE.md).

## Verified read-only inputs

At 13:43 UTC r2 had Python 3.9.25 owned by python3 RPM, enforcing SELinux,
wg-quick@wg0 startup, no address journal and none of the six proposed endpoint
files. Its updater was inactive/next 13:45; recheck after that execution before
install/activation. f3/r2 identities matched generator public identity files.
r2's saved/runtime non-Endpoint semantics matched exactly. f3 retains existing
gateway AllowedIPs drift described below; it is not changed by this r2-only
slice. Both hosts' native UDP56709 rules permit new-source transport, and r2
routes .20.133 directly with source .20.122. f3's reverse route is the same
connected /24. f3's prepared outbound primary-source guard remains undeployed.

Private controller evidence and exact file manifest:
`~/.local/state/conf/y14-preparation/endpoint/r2-f3-install-manifest.json`.
That manifest contains all source/destination paths, modes and SHA256 values;
source files remain generator-owned, not duplicated into conf.

Exact nonsecret scope:

```json
{
  "id": "y14_r2_f3_expiry_20261105",
  "protocol": "canary",
  "role": "peer",
  "interface": "wg0",
  "peer": "hEYrNMTlXLfhnmSdTgbH5FjYZTHUUFJ1VoPzVwu/Tz8=",
  "old": "192.168.1.133:56709",
  "new": "10.20.0.133:56709",
  "source_sha256": "41386d059b09101a4a4f8e2a5315345a2d6180ae111fb483ada527d9a9e653d0",
  "saved": "/etc/wireguard/wg0.conf",
  "wg": "/usr/bin/wg"
}
```

Endpoint helper SHA256 is
`670cd63c8175623f2c19b40cab47967646d0d6cec358a4ba05c1476c5b663af1`.
Revalidate all source digests and the YAML receipt immediately before install.
Fresh IDs/configs are required for later confirmation/boot trials. Absolute
authorization deadlines are generated on r2 only at the approved arm time;
no dated placeholder is an authorization.

## Proposed exact provisioning and activation

SSH root@192.168.1.122, port 22, using existing trusted host keys. Refuse if any
proposed file unexpectedly exists or any address/endpoint journal is pending.
Capture before-images/absence receipts privately. Install root:root:

- generator `endpoint/endpoint.py` to
  `/usr/local/sbin/gonf-endpoint-rollback` mode 0755;
- the four generator `endpoint/linux/gonf-endpoint-rollback*` units to the
  same basenames under `/etc/systemd/system`, mode 0644;
- private reviewed scope config to `/usr/local/etc/gonf-endpoint.json`, 0600;
- state parent `/var/lib/gonf-endpoint` and empty `pending`, 0700.

Run restorecon on introduced files/state, compare deployed bytes/modes/owners,
verify native Python syntax without network actions and systemd-analyze verify.
Inspect enabled-unit dependency links against actual wg-quick@wg0 and k3s.
Then daemon-reload; enable boot/runtime/timer units and start the timer. No
wg0/k3s restart, key change or address change occurs during provisioning.

Before trusting this as recovery, execute the helper's harmless `scope` action
as a PID1-owned transient oneshot with the same executable/PATH as the watchdog.
Verify success and inspect native journal/SELinux audit for denial; the real
watchdog's absent-record condition alone would skip execution and prove little.
No canary record or assertion exists during that execution test.

Reverify fresh authenticated old/new f3 and r2 literal SSH. Prove actual
r2 .20.122 UDP56709 datagram arrival at f3 .20.133 with a bounded passive
capture, plus explicit reverse route/source and existing port/rule ownership.
WireGuard discards an unauthenticated diagnostic datagram; no response is
assumed. Actual authenticated handshake/transfer acceptance follows the real
bounded endpoint apply. Make no permanent source-route or firewall change.

## Bounded expiry run after provisioning approval

Refresh private saved/runtime semantic snapshots, public identity binding,
source digest, health and scheduler checks. Write exactly the reviewed canary
scope and root-owned 0600 permission JSON bound by canonical config digest:
inner now+120, outer now+240. Issue `apply-approved` with the unique ID only
after the fresh path proofs. Do not create production recovery/accept markers.

Run helper snapshot/apply. Immediately compare saved/runtime semantic delta:
only f3's Endpoint IP changes on r2, with unchanged source YAML. Check fresh
bidirectional overlay IPv4/IPv6, authenticated handshake/counter progress,
old/new management, three etcd members/Ready/API, NFS consistency, cross-node
TCP/UDP DNS and verified-TLS ingress. The private health runner owns only
`y14-native-*` pods/UID receipts with label `ops.buetow.org/task=y14-20261105`;
cleanup uses those exact UIDs and never root's or sibling tasks' probes.

Close trial SSH connections and let the native inner timer restore. Reconnect
through old management after deadline+watcher cadence. Verify phase restored,
saved file OLD/exact original bytes, selected actual runtime endpoint restored,
all other saved/live fields unchanged and service checks passing. Capture the
reverse peer's naturally learned endpoint too; initiate fresh OLD transport
from r2 after restoration and prove both directions converge to their captured
old transport. Do not modify another peer by hand to hide unexpected roaming.
Repeat explicit restore twice; archive the entire journal privately only after
successful reconciliation. Reapply exact scoped dormant tooling with no record,
and verify no mutation/new authorization.

Later confirmation/outer-expiry and actual r2 boot trials need fresh scopes and
fresh timing checks. They remain separate coordinated steps in Y14-NEXT.md;
do not infer their success from the first expiry trial.

## Recovery and provisioning rollback

Any acceptance failure: use only helper `restore-runtime` for this scope;
native timer/boot hooks remain independent. Both destination addresses stay
configured. Never setconf/syncconf/reload a whole tunnel. Preserve failed
records for retry; do not remove a pending scope to make a check pass.

If the scoped installation itself must be undone, first prove no active record
or pending restore and reconcile/archive any canary. Disable/stop only its
introduced timer and disable its introduced boot/runtime units, remove only
the six introduced file paths and their dependency links, daemon-reload and
verify original wg0/k3s units remain active. Restore any actual before-images
instead if an approved reapply found pre-existing files. Keep private evidence;
do not remove a journal/state tree containing recovery evidence.

## Separate reverse-direction blocker

On f3 the saved public-gateway .110 peer declares .200/32 and ::200/128 in
addition to .110/::110; the running peer has only .110/::110. A separate
running peer owns .200. The current helper's per-peer saved/runtime comparison
will refuse a reverse f3-to-r2 snapshot. No AllowedIPs change is authorized.
Record the pre-existing difference and resolve its correct interpretation in
the generator tooling owner before that direction; do not weaken the clean-r2
preflight or quietly normalize the live host. Sanitized evidence is
`endpoint/semantic-drift-summary.json` beside the private snapshots.
