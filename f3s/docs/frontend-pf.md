# Frontend PF hardening (wm3): rolled out 2026-10-26

Default-deny PF plus internal httpd 8080 / inetd rsync 873 was applied serially
to fishfinger then blowfish on 2026-10-26 (CET). Fishfinger ZNC WireGuard-only
landed earlier the same calendar day. Acceptance evidence:
`~/.local/state/conf-worktrees/wm3-approved-20261025/live/wm3-acceptance-20261026.md`.
Owner approval and rollback procedure below remain authoritative. Do not reboot,
change SSH authentication, or restart WireGuard/relayd for this task.
Per-host rollback backups remain at `/root/wm3-before` until cleaned up.

## Policy and outstanding decision

Public `vio0` defaults to deny. Existing public TCP 2, 25, 53, 80, 443, 1965,
2022 and 2443, UDP 53/56709, DHCP client replies to UDP68 and required ICMP/ICMPv6
remain allowed. ICMPv6 includes RA/ND and packet-too-big; redirects are omitted.
Outbound connections, IPv4/IPv6 WireGuard NAT, inbound mesh access and existing
service/backend accounting labels remain. No public 8080, 873 or 2222 allowance
remains. Task **3n3** separately manages dserver's bindings and permissions.

HTTPD's entire 8080 backend binds to `127.0.0.1` and `::1`; port 80 remains public
for ACME and redirects. Inetd rsync binds to each frontend's WireGuard IPv4 and
IPv6 addresses, derived from the existing inventory. Gemini's loopback vger
listener is unchanged. Relayd already uses both loopback families for its backend.

**Approved ZNC policy (2026-10-25):** fishfinger TLS TCP4242 is WireGuard-only.
`frontends_wg_znc` preserves the current IPv4-only listener, TLS, users and IRC
networks, setting only its Host to `192.168.2.111`. No public4242 pass remains.
The operational task reads private state only on the host, preflights its known
single-listener layout, stops ZNC before its shutdown config write, then backs up
that final config under a root-private `/var/znc/configs/.wg-backup.*` directory.
It changes Host, starts ZNC and verifies the running bind. Failure stops ZNC
before restoring the backup and starting again. Credentials never enter a plan.
Unknown listener layouts fail closed before stopping; this is a deliberate
migration, not a general-purpose ZNC configuration manager.

SSH permits up to 100 established connections/source and a moving rate of 30/60s.
Exceeding either adds that source to `<bruteforce>` and **flushes its existing
states globally**, as requested; this can disconnect that source's SSH, HTTPS and
other sessions. The quick table block only denies subsequent public SSH. An
hourly-age expiry runs every five minutes, bounding a ban to roughly 60–65 minutes.
HTTPS permits 200 established connections/source and 100/10s; excess new states
are denied without adding the source to the SSH table. Shared NAT users can hit
these limits: acceptance requires real client checks and conservative observation,
not deliberately exhausting production limits. Authenticated mesh inbound rules
have no connection limits. Existing PF states bypass new rule evaluation; loading
rules alone does not close already established public backend/dserver sessions.
See [PF state and connection limits](https://man.openbsd.org/pf.conf).

## Preparation evidence (2026-10-22)

Read-only inventory on both live frontends found the listed public services plus
wildcard IPv4/IPv6 HTTPD8080, IPv4 rsync873 and dserver2222. Node exporter listens
only on the correct mesh IPv4. On both hosts, `getent hosts` resolves
`blowfish.wg0.wan.buetow.org` and `fishfinger.wg0.wan.buetow.org` to their
correct mesh IPv4 and ULA addresses. The checked-in Joern rsync job already
uses the blowfish mesh hostname. Fishfinger ZNC listens on IPv4 `*:4242`, with TLS
and active upstream IRC sockets; its config metadata says IPv6 is disabled.
Gorum is running with no Internet socket in `fstat`; no NRPE listener was present.
Both hosts run dhcpleased and slaacd; dropping DHCP or ND would break addressing.
No private keys, passwords, ZNC users/networks, or session tables are recorded here.

Targeted verification passed:

- `go generate ./...` (no generated description changes),
  `go build -p 2 ./frontends ./cmd/gonf`, `go vet -p 2 ./frontends`,
  `errcheck ./frontends` in `gonf/`.
- `sh -n frontends/scripts/frontend-activate.sh` and
  `shellcheck -s sh frontends/scripts/frontend-activate.sh`.
- `python3 frontends/scripts/tests/frontend-activation.py`: five local fault
  injection tests covering failed-load/restart retry after file convergence,
  parser failure before load, disabled-PF load-before-enable without flush,
  changed flags/class/helper, missing inputs and failed service-health checks.
- A combined local Gonf plan recorded all three tasks without dependency errors.
  Per-host PF candidates were rendered using Gonf's `RenderTemplate`, then passed
  both live native `doas pfctl -nf /dev/stdin` parsers, with no load.
- Both rendered HTTPD candidates passed native `httpd -n -f` using private
  temporary files, removed immediately afterward. HTTPD rejects `/dev/stdin`;
  that failed transport attempt did not change configuration or start a daemon.
- Inetd has no parse-only switch. Its host-prefix, bracketed IPv6 and separate
  tcp4/tcp6 entries were checked against [inetd(8)](https://man.openbsd.org/inetd.8)
  and the [native parser](https://github.com/openbsd/src/blob/master/usr.sbin/inetd/inetd.c).
  Successful binds remain a rollout acceptance check; `inetd -d` was not run.

These checks do not establish live filtering, rate-limit behavior, daemon binds,
DHCP renewal, IPv6 router lifetime, or post-rollout service health. Those require
the approved canary below. Parent orchestration owns the broader suite/review.

## Activation and retry behavior

`frontend-activate.sh` records `/var/db/gonf-frontend-{pf,httpd,inetd}.sha256`
only after successful activation. Its fingerprint includes the configuration,
the helper itself, daemon flags and inetd's login-class fragment. Validation and
activation failure leave the old success marker, so a repeated apply retries even
when the managed file is already current. PF is parsed before load, loads with
`pfctl -f` without flushing states, and is enabled only after a successful load
if needed. HTTPD and inetd receive one restart for initial activation; unchanged
healthy subsequent applies skip it. HTTPD validation happens before restart.
The helper's `check` mode is read-only and also checks PF/service health.

Markers record the last successful managed activation, not an attestation against
manual rule reloads. If an operator loads another PF file, remove its success
marker before reconciling; never use `pfctl -F all` as a reload procedure.

## Approved canary procedure

1. Confirm the parent has released the live lane for the approved candidate.
   Check that a shared public NAT is not already near the connection limits. Inspect current listeners again; stop
   if another approved public service has appeared. Coordinate task **3n3**
   first: its Gogios DTail checks must target the mesh before public2222 is
   denied, or old public-address checks will report failure. No recovery boot is part of
   this procedure. Keep two independent SSH sessions open on port 2 and a tested
   WireGuard route/session throughout. Do not depend solely on an existing state:
   test a new port-2 login after loading. Keep the second frontend untouched.
2. On the chosen canary, as root after approval, create a private backup and
   verify its PF syntax. Also record pre-change service and Gogios health:

   ```sh
   umask 077
   mkdir -p /root/wm3-before
   cp -p /etc/pf.conf /etc/httpd.conf /etc/inetd.conf /root/wm3-before/
   pfctl -nf /root/wm3-before/pf.conf
   rcctl check sshd httpd relayd smtpd nsd inetd
   ```

3. From this checkout, push only these tasks to the canary, for example:

   ```sh
   ./gonf.sh push -privilege doas -- -p 2 rex@fishfinger.buetow.org \
       frontends_wg_znc frontends_httpd frontends_inetd frontends_pf
   ```

   `--` is required before SSH flags. Do not use the cluster aggregate for the
   canary. Gonf may install/sync its remote binary as part of a push; a preview
   push is therefore also approval-gated. The selected tasks include the existing
   PF exporter cron/service reconciliation; inspect that part of the plan too.
4. Keep both sessions open and verify every acceptance item below. Any failure
   means stop and use the approved rollback through the still-open second
   session. Once the canary is accepted and the observation interval covers
   ordinary traffic and addressing health, repeat for blowfish with the same
   single-host command and acceptance. Record the passive observation interval; do
   not force DHCP renewal, restart networking, or flood a production listener.
5. Reapply once to the accepted host and confirm no HTTPD/inetd restart and no
   PF load occurs. `doas /usr/local/sbin/frontend-activate.sh check pf` (and
   `check httpd`, `check inetd`) must succeed. Review final actual rules and
   labels, not just `/etc/pf.conf` contents. Keep wm3 open until acceptance.

## Acceptance record to fill after rollout

- Fresh SSH port-2 logins over both available public families succeed. Existing
  sessions stay up, and SSH authentication behavior is unchanged. Mesh access
  stays available; WG recent-handshake metadata and IPv4/IPv6 routing are healthy.
- From the other frontend **and an external source**, new public IPv4/IPv6 TCP
  connections to 8080/873/2222 fail. Use `nc -4 -z -w 3 ADDRESS PORT` and the
  corresponding `nc -6` check against actual public IPs, not a mesh-resolving
  hostname. Do not claim preexisting connections were revoked by the reload.
- TCP2/25/53/80/443/1965/2022/2443 succeed on both families. DNS queries succeed
  over UDP and TCP; SMTP/Gemini/Forgejo and representative relayd sites pass their
  functional checks, including k3s, Pi static, Garage and fallback routes.
- Fishfinger public IPv4/IPv6 4242 fails; WireGuard IPv4 TCP and an
  authentication-free TLS handshake succeed. IPv6 ZNC remains disabled as before.
- `netstat -an` shows HTTPD8080 only on 127.0.0.1/::1 and rsync873 only on the
  host's mesh IPv4/IPv6. Vger11965 and relayd18080 remain loopback only.
  `rcctl check httpd inetd relayd smtpd nsd sshd` succeeds. No gorum/NRPE Internet
  listener appears; a running gorum process alone is not a listener.
- Read-only rsync module listing between frontends succeeds on their mesh IPv4
  and IPv6 addresses (`rsync rsync://ADDRESS/`, IPv6 URL host in brackets).
  Verify the normal scheduled frontend sync succeeds without forcing a transfer.
- Gogios DNS, SMTP, Gemini and HTTPS checks on both frontends stay green after
  their normal cycles. Confirm the task3n3 DTail checks target mesh addresses;
  old public2222 checks cannot be accepted as green under this policy.
  PF public/backend/WG labels still accumulate expected
  traffic; node-exporter metrics remain scrapeable through WireGuard. Check
  `<bruteforce>` size and errors privately; do not dump client/state data into
  the task log. Observe connection-limit counters and ordinary client sessions
  before accepting the thresholds. No production abuse/flood test is authorized.
- Addressing remains healthy over the agreed observation window: DHCP lease
  metadata, IPv6 routes/RA lifetime, ND neighbors and PMTU-dependent traffic.
  Passive checks cannot prove a future lease renewal; record that limitation.

## Approved rollback through the second session

Run as root within the approved rollback scope. This restores the prior PF file
without a state flush or reboot, then restores the backend bindings and restarts
only the affected daemons:

```sh
pfctl -nf /root/wm3-before/pf.conf
cp -p /root/wm3-before/pf.conf /etc/pf.conf
pfctl -f /etc/pf.conf
httpd -n -f /root/wm3-before/httpd.conf
cp -p /root/wm3-before/httpd.conf /etc/httpd.conf
cp -p /root/wm3-before/inetd.conf /etc/inetd.conf
rcctl restart httpd
rcctl restart inetd
rm -f /var/db/gonf-frontend-pf.sha256 \
    /var/db/gonf-frontend-httpd.sha256 /var/db/gonf-frontend-inetd.sha256
```

The old login-class fragment is unchanged by this task. The added expiry cron
can remain harmlessly installed while the prior ruleset is active. A retained
`<bruteforce>` table can still hold addresses: if one has been accidentally banned,
remove only that confirmed address (`pfctl -t bruteforce -T delete ADDRESS`) after
approval; do not clear all states. Restore/reconcile the previous checkout before
another automatic apply can reintroduce the candidate. Recheck SSH, service and
Gogios health after rollback.

## Refreshed candidate evidence (2026-10-25)

Based on canonical `6738ecd`, applying only reviewed `71e671c` changes plus the
approved ZNC migration. Both HTTPD renderings match the current live files
byte-for-byte. Existing private Gogios configuration permissions and all routing
assets are preserved; this task does not deploy relayd or Gogios.
Native PF and HTTPD parser-only checks passed on both hosts; temporary HTTPD
parser files were private and removed. Native ZNC shell syntax and awk synthetic
cases passed. The actual private ZNC config passed a silent native preflight;
only listener metadata was inspected. Go generate/build/vet and frontend
errcheck passed. The local plan records 80 operations. Five activation-helper
fault tests and six ZNC helper tests pass (including stop-time config saves,
failed startup, listener-probe failure, unknown layouts and convergence).

No live rules, configs or services were changed. Public2222 denial remains
strictly gated on 3n3 working WireGuard checks and parent lane release. These
parser/local results do not establish rollout acceptance.

For an approved ZNC rollback use the exact root-private backup path emitted by
the migration: stop ZNC, `cp -p BACKUP/znc.conf /var/znc/configs/znc.conf`, then
start ZNC and check its listener/TLS handshake. Always stop before restoring,
because ZNC writes config on shutdown. The pre-change PF backup must also be
restored if restoring public access. Never print or download the private backup.

## ZNC-only acceptance (2026-10-25, 16:35–16:36 UTC)

Remote Gonf logs reported 17:35–17:36 CET (UTC+01:00), verified against
fishfinger's Europe/Amsterdam timezone; the UTC interval above is corrected.

The approved fishfinger-only `frontends_wg_znc` task was applied from the exact
reviewed helper/recipe content of canonical `a7cc252` (isolated `47a0ca0`). Two
independent port-2 administration channels were checked beforehand. Gonf applied
8 operations: two helper assets and the migration changed; no other frontend
task was selected. No PF, HTTPD, relayd, inetd, Gogios or DTail change occurred.

- ZNC is healthy and listens only on IPv4 `192.168.2.111:4242`; no IPv6 listener
  was added. The destination helper's `check` succeeds.
- From blowfish, WireGuard TCP4242 and a TLS connection carrying only an
  unauthenticated IRC `CAP LS 302` negotiation succeeded, returning a CAP
  response. No credentials or user IRC messages were sent.
- From both blowfish and the external controller, new connections to fishfinger
  public IPv4 `46.23.94.99:4242` and IPv6 `[2a03:6000:6f67:624::99]:4242` failed.
  The external controller reported connection refused on both families; its
  direct WireGuard route is unavailable, so the positive mesh test used blowfish.
- A silent on-host comparison of the transformed post-stop backup against the
  entire current config matched exactly: only the intended listener Host change
  was introduced. Credentials and remaining private configuration were preserved.
  Config remains `_znc:_znc` mode0600. The backup directory is mode0700, root-owned:
  `/var/znc/configs/.wg-backup.1dGxI2ou`. Its contents remain only on fishfinger.
- A second scoped apply reported `4 ok, 0 changed, 1 skipped`: the migration
  command was skipped and ZNC was not restarted.

Rollback was not needed. If required within the approved scope, stop ZNC before
copying that directory's `znc.conf` back to `/var/znc/configs/znc.conf`, then start
and verify ZNC. Do not print the config. PF remains unchanged at this stage.
Full wm3 remains pending the coordinated default-deny/rsync rollout and its
acceptance; working frontend DTail WireGuard checks from task3n3 still gate
public2222 denial.
