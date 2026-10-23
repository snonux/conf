# Four frontend peer certificate refreshes (ro3)

Prepared 2026-10-23 from published `613310e`, reviewed at `1bb8be3`, then
explicitly approved and deployed on 2026-10-23. See the live acceptance record
below. The preparation itself read public TLS/DNS, configuration and file
metadata only; it performed no production key transfer or service activation.
Synthetic fixture keys are disposable and unrelated to production. Future
refreshes still require the concrete approval described by this runbook.

## Named material and public identity

All four current DNS owners have valid replacements. Strict public TLS using
Python's default CA/hostname verification confirmed these leaves on **both
IPv4 and IPv6**, including `www.gogios.buetow.org`, on 2026-10-23. Port is443.
Refresh these observations immediately before an approved transfer; if the
owner pin changed, stop and review the new public metadata before transferring.

| Pair / exact SANs | Owner → stale destination | Stale expiry UTC, 2026-10-23 | Owner expiry UTC |
|---|---|---|---|
| standby.ecat.buetow.org | fishfinger → blowfish | 22:31:51 | 2027-01-12 22:32:55 |
| standby.alt.irregular.ninja | fishfinger → blowfish | 22:32:48 | 2027-01-12 22:34:14 |
| standby.gogios.buetow.org | fishfinger → blowfish | 22:33:04 | 2026-12-15 22:31:56 |
| gogios.buetow.org + www.gogios.buetow.org | blowfish → fishfinger | 22:32:05 | 2026-12-15 22:35:17 |

Owner public leaf SHA256 pins (DER leaf, lowercase hex):

```text
standby.ecat.buetow.org          567fc8880dca99d274a18d0aad5814e87e911ae2dedc1142b7ab0a1706a84701
standby.alt.irregular.ninja      d5baa12feb85607960a88d25c49e4fa4582d7e3586dbe610c33ff54155051266
standby.gogios.buetow.org        6c9f917268a4bf74593a7544af733067f958af45725681d7b7464bde3aac4bd4
gogios.buetow.org + www          dfa7af738d6717c67645fe8d72d1fab92eda6ca7f2ea8a1e5f1da90e2b763e46
```

blowfish addresses: `23.88.35.144`, `2a01:4f8:c17:20f1::42`.
fishfinger addresses: `46.23.94.99`, `2a03:6000:6f67:624::99`.
DNS A/AAAA select fishfinger for the three standby names, blowfish for Gogios
bare/www. Preserve that topology and every owner pair.

For every scoped name, both hosts have regular
`/etc/ssl/NAME.fullchain.pem` root:wheel0444 and
`/etc/ssl/private/NAME.key` root:wheel0400, with
`/etc/ssl/NAME.crt` an absolute symlink to its matching fullchain. Preparation
inspected key metadata only. Keep these modes and aliases; refuse another
layout. No certificate in this task needs CA issuance.

Public leaf SPKI comparison also found owner and stale peer keys differ for
**all four pairs**. A chain-only refresh cannot reuse the existing peer key.
The comparison used only the strict-TLS public leaves; no private key was read.

Both relayd daemons were running. Observed `/etc/relayd.conf` SHA256:

```text
blowfish    907dcf31693d29e9c28fbf019275d2653b53ff64ae31fe88111bfc550c687b6b
fishfinger  cb6677bce67c0c7415249e393a59a76b83396b79ef5224d67e905fde54da696f
```

These are observations, not permanent deployment pins: re-baseline after any
separately approved qo3/ho3 activation. The current policy lineage is `3cd0355`.
**Never apply this worktree's published-old full Web/Gonf plans.** No relayd,
httpd, ACME, DNS, route, authentication, SSH or signing-key change belongs here.

## Review and approval boundary

Review the three helper scripts, strict monitoring delta and fixtures. Obtain
explicit approval for exactly the four pair transfers, root-private staging
and backups, bounded relayd reload on each destination and scoped Gogios delta.
Approval must also cover rollback restoring only that host's named pairs and
Gogios snapshot, with a bounded start only if preflight recorded relayd running.
Do not execute the mutation examples below until that approval exists.

Coordinate with qo3 and ho3: finish and accept one host before touching the
next. Do not allow ACME, another deployment, reload or certificate operation
while a guard is armed. Start with blowfish's three standby pairs, then
fishfinger's Gogios pair. The unchanged DNS owners keep serving throughout.

Before mutation verify existing SSH host-key pins with BatchMode and
StrictHostKeyChecking=yes (frontend port2 from existing SSH policy), source
leaf identities, exact file metadata/layout, normal daemon and monitoring
state and current native configuration validity. Record public metadata only.
No new SSH credential, agent policy or key cache is part of this task.

## Private backup and staging (approval required)

The following root-shell setup is run on **one selected destination**, over
the existing pinned SSH connection. It refuses operation-directory reuse.
`relayd -n` and the staged validator can internally read keys only after
the concrete live-operation approval. Backups never leave the destination.

```sh
set -eu
umask 077
work=/root/ro3-peer-certs-20261023
case "$(hostname -s)" in
    blowfish) names='standby.ecat.buetow.org standby.alt.irregular.ninja standby.gogios.buetow.org' ;;
    fishfinger) names=gogios.buetow.org ;;
    *) exit 1 ;;
esac
timeout 10 rcctl check relayd
timeout 30 relayd -n -f /etc/relayd.conf
mkdir -m 700 "$work"
mkdir -m 700 "$work/incoming"
touch "$work/relayd-was-running"
sha256 -q /etc/relayd.conf >"$work/relayd-config.sha256"
test -f /etc/gogios.json && test ! -L /etc/gogios.json
cp -p /etc/gogios.json "$work/previous.gogios.json"
for name in $names; do
    test -L "/etc/ssl/$name.crt"
    test "$(readlink "/etc/ssl/$name.crt")" = "/etc/ssl/$name.fullchain.pem"
    test -f "/etc/ssl/$name.fullchain.pem" && test ! -L "/etc/ssl/$name.fullchain.pem"
    test -f "/etc/ssl/private/$name.key" && test ! -L "/etc/ssl/private/$name.key"
    test "$(stat -f '%Su:%Sg:%Lp' "/etc/ssl/$name.fullchain.pem")" = root:wheel:444
    test "$(stat -f '%Su:%Sg:%Lp' "/etc/ssl/private/$name.key")" = root:wheel:400
    cp -p "/etc/ssl/$name.fullchain.pem" "$work/$name.fullchain.pem"
    cp -p "/etc/ssl/private/$name.key" "$work/$name.key"
done
```

On the source, before each named export require regular, non-symlink files,
chain root:wheel0444, key root:wheel0400 (stop for changed metadata), and
fingerprint the public fullchain leaf to match the independently observed TLS
pin. Use `openssl x509 -in /etc/ssl/NAME.fullchain.pem -outform DER | openssl
dgst -sha256`, with pipefail in the invoking Bash. Never derive the expected
pin from the incoming archive. No source file is modified.

Controller Bash streams exactly two named members, with xtrace off. Select
source/destination/name from the approved table; no controller file, tee,
verbose tar or shell variable may contain the stream. Example for the first
pair (repeat only the other two standby names before blowfish activation):

```bash
set -euo pipefail
name=standby.ecat.buetow.org
ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes rex@fishfinger.buetow.org \
  "doas -n tar -C /etc/ssl -cf - $name.fullchain.pem private/$name.key 2>/dev/null" \
  | ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes rex@blowfish.buetow.org \
      "doas -n tar -C /root/ro3-peer-certs-20261023/incoming -xf - $name.fullchain.pem private/$name.key 2>/dev/null"
```

For fishfinger's later phase reverse source/destination and set
`name=gogios.buetow.org`. Check extraction success and metadata before using
either file. Stop after a failed or unknown transfer and inspect owned
staging plus public state before retrying. No live file has yet changed.

Validate **every** incoming pair on its destination using the reviewed
`check-peer-certificate.sh` over stdin, e.g. for the first pair:

```sh
ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes rex@blowfish.buetow.org \
  'doas -n sh -s -- standby.ecat.buetow.org /root/ro3-peer-certs-20261023/incoming/standby.ecat.buetow.org.fullchain.pem /root/ro3-peer-certs-20261023/incoming/private/standby.ecat.buetow.org.key 567fc8880dca99d274a18d0aad5814e87e911ae2dedc1142b7ab0a1706a84701 /etc/ssl/cert.pem' \
  < frontends/scripts/check-peer-certificate.sh
```

It refuses unapproved names/symlinks, checks the public pin, exact SAN set,
trusted server chain, >=30 days and key correspondence. Only public-key
derivatives enter its private temporary directory. Encrypted or malformed
keys fail without a prompt or key diagnostics. Do not publish after any failure.

## Monitoring delta and timed rollback

Normal DNS checks missed these copies. Prepared Go source adds ten
`Check Peer HTTPS ... on HOST` checks per frontend: five SANs × two own
numeric addresses. `check_curl -D` verifies CA and hostname, `-C 20,7` warns
at20days and is critical at7; `Local:true` and no dependencies keep checks
active on the passive frontend. HTTPS must still return200.

The existing full monitoring renderer reflects published source. **Publish
only these ten added definitions into the latest live Gogios JSON**, retaining
every existing check and top-level field, including qo3/ho3 changes if deployed.
The fixture exporter renders real Go source without recording any recipe plan:

```sh
python3 frontends/scripts/tests/peer-certificate-monitoring.py --output /tmp/ro3-checks
```

For each selected host, read its latest public `/etc/gogios.json` over pinned
SSH, merge only `HOST-checks.json` into `Checks` using controller Python,
refuse any existing name collision, and prove removing the ten added names
recovers the parsed original exactly. Keep that exact original as the private
`previous.gogios.json` snapshot above; compare the current live file against
the snapshot immediately before publication, to refuse concurrent changes.
Stage the merged candidate in the private operation directory as
`incoming.gogios.json`, root:wheel0744. Validate JSON before publishing; source
and destination metadata/layout plus the ten check definitions must match the
review. This operation installs no package, plugin, cron or service.
The parent must finish this scoped merge during integration; no full Gogios
or DTail task from another prep is an implementation dependency.

After all staged pairs and the monitoring candidate pass, install the public
rollback script only on the current destination:

```sh
ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes rex@blowfish.buetow.org \
  'doas -n sh -c "cat > /root/ro3-peer-certs-20261023/rollback; chmod 700 /root/ro3-peer-certs-20261023/rollback"' \
  < frontends/scripts/rollback-peer-certificates.sh
```

Arm a15-minute independent guard only after that host's complete backups,
baseline-running marker, validated incoming files and tested rollback exist:

```sh
work=/root/ro3-peer-certs-20261023
nohup sh -c 'sleep 900; [ -f /root/ro3-peer-certs-20261023/accepted ] || [ ! -f /root/ro3-peer-certs-20261023/publication-started ] || exec /root/ro3-peer-certs-20261023/rollback' \
  </dev/null >"$work/watchdog.log" 2>&1 &
echo "$!" >"$work/watchdog.pid"
```

The publication-started marker is written immediately before the first rename;
the timer does nothing if staging/preconditions failed before live mutation.
The watchdog log contains status only in the root-private directory. Preserve
the independent PID, verify it is alive before publication, and leave enough
time for immediate acceptance. Do not mutate near the watchdog deadline or
run an immediate rollback concurrently with the timer. On an uncertain timeout,
inspect the owned PID, files and public state; do not repeat mutation blindly.

The root-shell publication on the **current** host stages all files beside
their destinations before any rename. The loaded daemon retains its old keys
until reload. An ordinary failure invokes rollback; the timer covers lost
sessions. No config replacement or restart belongs to normal publication:

```sh
set -eu
umask 077
work=/root/ro3-peer-certs-20261023
case "$(hostname -s)" in
    blowfish) names='standby.ecat.buetow.org standby.alt.irregular.ninja standby.gogios.buetow.org' ;;
    fishfinger) names=gogios.buetow.org ;;
    *) exit 1 ;;
esac
stage=$(mktemp -d /etc/ssl/.ro3-publish.XXXXXXXXXX)
key_stage=$(mktemp -d /etc/ssl/private/.ro3-publish.XXXXXXXXXX)
gogios_tmp=
published=no
finish() {
    status=$?
    trap - 0
    rm -rf "$stage"
    rm -rf "$key_stage"
    [ -z "$gogios_tmp" ] || rm -f "$gogios_tmp"
    if [ "$status" -ne 0 ] && [ "$published" = yes ]; then
        "$work/rollback" || printf 'Rollback failed; inspect private backups.\n' >&2
    fi
    exit "$status"
}
trap finish 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
for name in $names; do
    install -o root -g wheel -m 0444 "$work/incoming/$name.fullchain.pem" "$stage/$name.fullchain.pem"
    install -o root -g wheel -m 0400 "$work/incoming/private/$name.key" "$key_stage/$name.key"
done
gogios_tmp=$(mktemp /etc/.ro3-gogios.XXXXXXXXXX)
install -o root -g wheel -m 0744 "$work/incoming.gogios.json" "$gogios_tmp"
test "$(sha256 -q /etc/relayd.conf)" = "$(cat "$work/relayd-config.sha256")"
cmp -s /etc/gogios.json "$work/previous.gogios.json"
timeout 10 rcctl check relayd
kill -0 "$(cat "$work/watchdog.pid")"
touch "$work/publication-started"
published=yes
for name in $names; do
    mv -f "$key_stage/$name.key" "/etc/ssl/private/$name.key"
    mv -f "$stage/$name.fullchain.pem" "/etc/ssl/$name.fullchain.pem"
done
mv -f "$gogios_tmp" /etc/gogios.json
gogios_tmp=
timeout 30 relayd -n -f /etc/relayd.conf
timeout 30 rcctl reload relayd
timeout 10 rcctl check relayd
```

Multiple renames are not an atomic transaction. On a partial failure stop and
inspect actual state after rollback. The scoped rollback restores three pairs
on blowfish or one on fishfinger plus that host's Gogios snapshot, preserving
ownership/modes and every `.crt` symlink. It never restores or overwrites
relayd.conf, httpd, ACME, DNS or owner material. It stages every backup before
the first live replacement, native-validates, bounded-reloads a running daemon,
or bounded-starts only from a recorded running baseline. Unknown daemon status
fails without a start. Backups remain private after success/failure.

## Immediate acceptance, then combined completion

Before accepting the current host's guard, run:

```sh
python3 frontends/scripts/check-peer-certificate-public.py --host blowfish --expect-refreshed
# Later, after fishfinger publication: use --host fishfinger.
```

It verifies all five SANs on both numeric address families, chain/hostname,
reviewed new leaf pins, >=30days, HTTP200 and existing HSTS. It also verifies
unknown/LAN/missing Host403 and duplicate Host400. Compare stable Ecat/album
public content hashes against the pre-operation baseline and expected source;
Gogios content is dynamic, so verify the normal monitoring page rather than a
frozen hash. Keep the unchanged relayd.conf hash, mode/alias checks and unrelated
public routes (including dedicated code2443 and LAN denials) in acceptance.
Run each new native `check_curl` command on the current host: all ten must beOK.
Confirm the ten local definitions/no dependencies and every prior config field.

Only after these immediate checks pass on that host, create its acceptance
marker over pinned SSH as root:

```sh
touch /root/ro3-peer-certs-20261023/accepted
```

Then repeat the entire setup/staging/guard/publication/acceptance sequence for
fishfinger. Do not arm both hosts' rollback guards together. A failed check or
insufficient acceptance time means rollback, not acceptance. Record public
pins/expiry, command status and private backup paths only, never key values.

After both hosts pass, require a fresh normal Gogios cycle with the new checks
present on both frontends and no unexplained TLS failures. Cached hourly
results are a separately recorded pending combined gate; do not leave a
verified host's15-minute guard armed waiting for an hourly cache. Do not force
a whole monitoring run or call direct plugin success normal-cycle acceptance.
Task completion still requires approved native/live acceptance and parent signoff.

## Renewal policy

This is **operator-assisted peer refresh after each owner ACME renewal**, not
automatic replication. DNS keeps owner-only issuance. After renewal, obtain an
independent strict public owner pin, refresh only the peer's named pair through
the reviewed procedure, and verify all SANs/families before acceptance. Respond
to a20-day warning by refreshing and accepting the peer while >=30days remain
on the owner candidate; otherwise investigate owner renewal first. Record
owner/destination, public identity/date and acceptance. Retain backups under
the operation's root-private directory until separately approved cleanup.
For a later refresh, select a fresh operation directory and update the fixed
helper/guard paths together during review; never reuse this operation's backups.

## Preparation verification and limits

`peer-certificate.py`:19 real synthetic RSA/EC/fullchain cases, four valid scoped
names and wrong/missing/extra SAN, pin, short lifetime, CA, mismatched key,
scope, symlink, missing, malformed and encrypted-key negatives; native blowfish
LibreSSL runs the same fixtures in disposable `/tmp`, then removes them.
No production key was used. `peer-certificate-rollback.py` exercises host-specific
scope, all-backup staging, metadata, aliases, failures, reload/start recovery and
unknown status through shims, not production activation. Monitoring fixtures
use Go overlays to exercise the actual renderer, exactly10 additions per host
against613310e, and Local/dependency/trust/address/expiry/SNI negatives. No
`*_test.go` is added under gonf. Native check_curl probes select the expected
off-DNS stale leaves and ownerOK, proving the address/SNI path.

At preparation time, native relayd parsing/publication with transferred
material and real rollback activation had not run. No full Web plan, CA
request, production key validation or service reload/restart ran during
preparation. The approved live results follow below.

Targeted checks passed: `go build -p 1 ./...`, `go vet -p 1 ./frontends` and
`errcheck ./frontends` in gonf; shellcheck for both shell helpers; native
fishfinger `sh -n` for both helpers. The rollback suite also parses every
documented shell block and exercises the documented timer's four marker
decisions with a zero-delay synthetic shim. This is not a production timer test.


## Approved live acceptance, 2026-10-23

The user explicitly approved source publication, the four exact pair transfers,
twenty monitoring additions, serial relayd reloads, 15-minute guards and live
acceptance. Blowfish's three standby pairs were accepted at 14:56:40 UTC;
fishfinger's Gogios pair was accepted at 15:00:17 UTC. The public owner pins,
SANs and expiry dates in the table above remained unchanged. Only the four
named chain/key pairs and each host's ten Gogios definitions were published.

Every pair passed native LibreSSL validation of the independent public pin,
exact SANs, trusted server chain, at least 30 days of lifetime and matching
private key. Transfers streamed the two named archive members between pinned
SSH connections directly into destination staging. No production private-key
bytes entered controller files, shell arguments, logs or tool output. Original
backups were preserved on their destination throughout.

Each host passed native relayd validation, bounded reload and running checks;
all five SANs on both address families passed strict TLS/SNI, reviewed leaf
identity, lifetime, HTTP 200 and HSTS checks. All twenty native check_curl
commands returned OK. Ecat/album body hashes matched their pre-operation
baseline. Normal Gogios pages, unknown/LAN/missing Host 403, duplicate Host 400,
dedicated code 2443 routing, public 443 code denial and dedicated-port LAN
denial were preserved. The code certificate's identity and expiry were
unchanged. Relayd, HTTPD and ACME configuration hashes, certificate aliases,
ownership and modes matched the live baseline.

Fishfinger's first immediate extra route check failed an HTTP-status assertion
after reload. The original assertion did not retain the row or actual status,
so its exact cause is unknown; reload/backend settling is only a possibility.
The approved scoped rollback ran successfully, native-validated and reloaded
relayd, and restored both the original pair and Gogios file byte-for-byte.
The original watchdog wrapper was terminated after verified rollback, its PID
retained, and its publication marker cleared. Original backups were never
overwritten. The same validated staging was then republished under a fresh
15-minute guard after baseline revalidation. Bounded readiness probes returned
200 before the complete acceptance suite passed. Neither host was accepted
on a failed assertion.

Each live Gogios merge preserved all 531 existing checks and every other
configuration field, adding exactly ten Local checks without dependencies.
The normal scheduled 15:00 UTC run produced all twenty new own-host results
with Status 0 and no federation source: blowfish at 15:00:02–15:00:11 UTC,
fishfinger at 15:00:01–15:00:39 UTC. This includes execution on the passive
frontend; no forced Gogios run was used. Observed totals were 548 OK on
blowfish and 558 OK on fishfinger, with one critical and one warning on each;
the difference reflects peer results available during that cycle. Remaining
non-OK states were the pre-existing standby.code TLS critical (current code
certificate expires 2026-10-24 07:34:23 UTC) and KubeJobFailed warning. The
separate code-certificate task ho3 was not activated or repaired here.

Both acceptance markers disarmed their guards promptly. Private backups,
incoming staging, rollback helper and status files remain under root:wheel
0700 `/root/ro3-peer-certs-20261023` on each destination for separately approved
cleanup; publication/rollback scratch directories were removed. Accepted
guard wrappers may finish their original sleep and then exit without action.
No full Gonf deployment, CA issuance or unrelated configuration change ran.
A real scoped rollback/reload was exercised on fishfinger; automatic watchdog
expiry and baseline-stopped daemon recovery were not deliberately triggered.
Renewal custody remains the operator-assisted procedure above.
