# Forgejo frontend certificate repair (ho3)

Prepared on 2026-10-23; **not deployed**. Fresh expert review and explicit
approval are required before certificate/key transfer, production-file changes,
relayd reloads or monitoring deployment. The f3 power-on and PF approvals do
not authorize this repair. Do not push evolving conf master.

## Verified problem and candidate

fishfinger serves a primary `code.f3s.buetow.org` certificate expiring at
**2026-10-24 07:34:23 UTC**, on both 443 and 2443. Its separate
`standby.code.f3s.buetow.org` certificate is valid until December 31, but the
2443 protocol currently loads only the primary keypair. A standby SNI request
therefore receives the expiring primary certificate. That explains Gogios's
standby-labelled critical. Changing the check port alone would hide a real
fallback-certificate problem.

blowfish's primary certificate is valid until **2026-12-31 22:32:17 UTC**.
A controller-side TLS connection to blowfish:2443, using Python's default
trusted context and SNI `code.f3s.buetow.org`, verified its chain/hostname and
observed public leaf SHA-256:

`46a54fab50c52b53fee77d46c6d1c565f90c8415045d2cc5e5278c1446ea34c5`

Repeat that verified observation immediately before an approved transfer;
abort if the source changed and refresh the reviewed pin. Never derive the
expected pin from the unvalidated incoming archive. Preparation inspected public
certificate metadata; no production private-key bytes were extracted,
transferred, displayed or saved. Native relayd parsing may load TLS material
internally, without exporting it.

The existing ACME script renews only names whose A answer equals the local
frontend's address. Invoking it on fishfinger while code resolves to blowfish
will skip that primary name. This repair copies the already-issued valid web
keypair over authenticated SSH; it does not issue a certificate or change DNS.
The private key remains outside Git and never lands on the controller's disk.

The prepared source also adds the standby keypair to the alternate listener
and two explicit expiry checks: primary code SNI pinned to each frontend's
inventory IPv4 and port 2443. Existing checks are retained. The HTTP Host
allowlist remains exactly code and code:2443; adding a TLS SNI certificate
does not authorize another HTTP Host.

## Approval scope and preflight

Request approval for the complete repair: copying this existing primary web
keypair from blowfish to fishfinger, root-private staging/backups and timed
rollback, the one-line relayd SNI addition on both frontends, and the two new
Gogios checks. Use the isolated ho3 source and targeted tasks only. No package
upgrade, SSH authentication change, reboot, CARP operation or Argo publication
is part of this procedure.

Before mutation, verify both SSH host-key pins, source public TLS identity,
both frontend services and current Git advertisement responses. Baseline:
the conf repository's `info/refs?service=git-upload-pack` returned HTTP200
through both frontends on 2443 with normal CA/hostname verification. The new
check_http arguments already produce OK on blowfish and CRITICAL on
fishfinger, proving they select the intended primary certificates.

On the source, inspect only metadata and require both exported paths to be
root-owned regular files, without symlinks. Require its private key mode to
be 0400 or 0600. Confirm the source public file's leaf matches the independently
verified TLS pin. On fishfinger, require the existing `.crt` symlink to target
`/etc/ssl/code.f3s.buetow.org.fullchain.pem`. Stop for a different layout.
Check that no ACME/configuration operation is running; perform no unrelated
frontend changes while these rollback guards are armed.

## Stage only the approved material

After approval, create private backups; these commands refuse directory reuse.
Leave the existing `.crt` symlink intact. These are live-operation commands,
not commands run during preparation:

```sh
for host in fishfinger blowfish; do
    ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes "rex@$host.buetow.org" \
      'doas -n sh' <<'SH'
set -eu
umask 077
work=/root/ho3-code-cert-20261023
rcctl check relayd
relayd -n -f /etc/relayd.conf
mkdir -m 700 "$work"
cp -p /etc/relayd.conf "$work/previous.relayd.conf"
cp -p /etc/gogios.json "$work/previous.gogios.json"
if [ "$(hostname -s)" = fishfinger ]; then
    test -L /etc/ssl/code.f3s.buetow.org.crt
    test "$(readlink /etc/ssl/code.f3s.buetow.org.crt)" = /etc/ssl/code.f3s.buetow.org.fullchain.pem
    for file in /etc/ssl/code.f3s.buetow.org.fullchain.pem /etc/ssl/private/code.f3s.buetow.org.key; do
        test -f "$file" && test ! -L "$file"
    done
    cp -p /etc/ssl/code.f3s.buetow.org.fullchain.pem "$work/previous.fullchain.pem"
    cp -p /etc/ssl/private/code.f3s.buetow.org.key "$work/previous.key"
    chmod 600 "$work/previous.key"
    mkdir -m 700 "$work/incoming"
fi
SH
done
```

From Bash on the controller, stream exactly two files. Keep xtrace disabled;
do not use tar's verbose flag, tee the stream, capture it in a variable or
redirect it to a controller file:

```bash
set -euo pipefail
ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes rex@blowfish.buetow.org \
  'doas -n tar -C /etc/ssl -cf - code.f3s.buetow.org.fullchain.pem private/code.f3s.buetow.org.key' \
  | ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes rex@fishfinger.buetow.org \
      'doas -n tar -C /root/ho3-code-cert-20261023/incoming -xf - code.f3s.buetow.org.fullchain.pem private/code.f3s.buetow.org.key'
```

A failed/unknown transfer is not permission to retry blindly: inspect the
owned staging directory and actual public TLS state first. No live files
have changed at this stage. Run the committed validator via stdin; it reads
the staged key in place, never copies/prints it, and uses only public-key
temporary files in a private temporary directory:

```sh
ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes rex@fishfinger.buetow.org \
  'doas -n sh -s -- /root/ho3-code-cert-20261023/incoming/code.f3s.buetow.org.fullchain.pem /root/ho3-code-cert-20261023/incoming/private/code.f3s.buetow.org.key 46a54fab50c52b53fee77d46c6d1c565f90c8415045d2cc5e5278c1446ea34c5 /etc/ssl/cert.pem' \
  < frontends/scripts/check-code-certificate.sh
```

It requires the trusted public leaf pin, a trusted server chain, at least 30
days remaining and a matching private/public key. Encrypted or malformed keys
fail without prompting. Stop on any failure; do not publish either file.

## Timed rollback and publication

The committed rollback helper stages every old file before changing any live
file, restores only configuration on blowfish, and also restores the old pair
on fishfinger. It uses same-directory rename, root ownership, modes0644 for
relayd configuration/chain,0744 for existing Gogios configuration and0600 for
the key. It leaves the `.crt` symlink intact, validates the restored native
configuration and reloads only an already-running relayd. Failure is reported
and the private backups remain. Install this public script after approval:

```sh
for host in fishfinger blowfish; do
    ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes "rex@$host.buetow.org" \
      'doas -n sh -c "cat > /root/ho3-code-cert-20261023/rollback; chmod 700 /root/ho3-code-cert-20261023/rollback"' \
      < frontends/scripts/rollback-code-certificate.sh
done
```

Arm each 15-minute independent watchdog immediately before changing that
frontend. Its rollback restores only files saved for that host. Do not arm the
blowfish guard while fishfinger is still being repaired.

Each watchdog uses the same bounded guard pattern as the reviewed
[PF rollout](../../f3s/freebsd-hosts/pf/README.md):

```sh
work=/root/ho3-code-cert-20261023
nohup sh -c 'sleep 900; [ -f /root/ho3-code-cert-20261023/accepted ] || exec /root/ho3-code-cert-20261023/rollback' \
  </dev/null >/dev/null 2>&1 &
echo "$!" >"$work/watchdog.pid"
```

Do not arm this command until the tested rollback script and old backups
exist. Do not overwrite those backups during retries. Preserve them after
acceptance for manual rollback; they include a private key and stay0700/0600.

On fishfinger, after validation and arming its watchdog, publish both files
before reloading. The daemon uses its loaded pair until reload. The EXIT trap
rolls back an ordinary command failure; the independent timer covers an
interrupted/killed session. Do not restart an unexpectedly stopped daemon:

```sh
ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes rex@fishfinger.buetow.org \
  'doas -n sh' <<'SH'
set -eu
umask 077
work=/root/ho3-code-cert-20261023
chain_tmp=
key_tmp=
finish() {
    status=$?
    trap - 0
    [ -z "$chain_tmp" ] || rm -f "$chain_tmp"
    [ -z "$key_tmp" ] || rm -f "$key_tmp"
    if [ "$status" -ne 0 ]; then
        "$work/rollback" || printf 'Rollback failed; inspect retained backups.\n' >&2
    fi
    exit "$status"
}
trap finish 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
chain_tmp=$(mktemp /etc/ssl/.ho3-new-chain.XXXXXXXXXX)
key_tmp=$(mktemp /etc/ssl/private/.ho3-new-key.XXXXXXXXXX)
install -o root -g wheel -m 0644 "$work/incoming/code.f3s.buetow.org.fullchain.pem" "$chain_tmp"
install -o root -g wheel -m 0600 "$work/incoming/private/code.f3s.buetow.org.key" "$key_tmp"
rcctl check relayd
mv "$key_tmp" /etc/ssl/private/code.f3s.buetow.org.key
key_tmp=
mv "$chain_tmp" /etc/ssl/code.f3s.buetow.org.fullchain.pem
chain_tmp=
relayd -n -f /etc/relayd.conf
rcctl reload relayd
rcctl check relayd
SH
```

If the two renames or rollback itself fail, stop and inspect actual files and
daemon state. Do not claim an atomic two-file transaction or repeat a failed
mutation blindly. Neither validator nor rollback prints private material.

Deploy the prepared source one frontend at a time, using explicit `doas`
privilege and existing SSH port2. Finish fishfinger's immediate acceptance and
accept its guard before arming the blowfish guard and proceeding to blowfish:

```sh
./gonf.sh push -privilege=doas -- -p2 -oBatchMode=yes \
  -oConnectTimeout=7 rex@fishfinger.buetow.org \
  frontends_relayd frontends_d_tail_checks
./gonf.sh push -privilege=doas -- -p2 -oBatchMode=yes \
  -oConnectTimeout=7 rex@blowfish.buetow.org \
  frontends_relayd frontends_d_tail_checks
```

The existing d_tail_checks action publishes only Gogios configuration; it
does not install/upgrade packages or alter cron/services. The relayd task
includes existing ACME setup prerequisites, not ACMEInvoke. Inspect the exact
fresh plan/dry run and stop if anything outside the two intended configuration
changes per host would change. Do not run frontends_gogios or the aggregate.

## Acceptance and renewal ownership

Before accepting either watchdog, require:

- Normal trusted TLS connections to both frontends, on443 and2443, with code
  SNI serve the verified primary leaf. Standby code SNI on2443 must now verify
  the standby hostname/chain instead of falling back to the primary leaf.
  blowfish's existing standby certificate is valid until November6; it is
  not the transferred primary certificate and is not claimed to have renewed.
- Real conf Git advertisement requests return200 through both frontends on
  2443. Unknown/LAN/missing/duplicate Host rejection and public443 code denial
  retain the existing policy; canonical HSTS/forwarding behavior is preserved.
- The exact new check_http commands select the primary SNI on both pinned
  endpoints and report OK. Both Gogios configurations contain both checks.
- No unexpected process, configuration, certificate permission or service
  change. Record public certificate fingerprints/dates, command statuses,
  backup locations and final monitoring state, never key/session/cookie bytes.

After those immediate checks on a frontend pass, create its `accepted` marker
before proceeding. If time is running out or a test fails, roll back instead
of disarming the timer. A fresh normal monitoring cycle with no remaining TLS
critical is a separate **task-completion** gate: an hourly cached old result
must not keep an otherwise verified repair's rollback timer armed past its
deadline. Capture that pending state honestly; do not force a whole Gogios run
or claim all-green acceptance from direct plugin output alone. Do not remove
a backup or secret staging file outside this operation's own directories.

For the verified host only, acceptance is the explicit command below (choose
the host whose checks just passed):

```sh
ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes rex@fishfinger.buetow.org \
  'doas -n touch /root/ho3-code-cert-20261023/accepted'
# Run the equivalent on blowfish only after its own checks pass.
```

This is a bounded emergency synchronization, **not automatic certificate
replication**. ACME ownership remains determined by live DNS. After a primary
renewal or role change, refresh the inactive frontend using this reviewed
procedure before its monitored certificate expires. The new checks warn20
days ahead independently of DNS ownership; retain them. A future automated
renewal/distribution design needs its own review and approval.

## Verification performed before deployment

Real fixture tests cover RSA/EC pairs, multi-PEM chains, mismatched keys,
wrong pins, untrusted chains, near expiry, malformed/missing inputs and
noninteractive encrypted-key refusal, with unchanged inputs and no secret
output. Rollback fixtures cover restoration on each host, staging failure
before publication, missing/symlinked backups, parser failure, stopped daemon,
reload failure and wrong-host/non-root refusal. Render integration tests cover both SNI bindings and both pinned
primary checks, with previous-single-keypair/missing-endpoint/wrong-SNI/
unpinned-address/duplicate-endpoint negative controls. Baseline JSON comparison
finds exactly two added checks and no changed/removed checks or other fields
on either frontend. Native relayd candidates passed on both OpenBSD hosts in
temporary private files, subsequently removed; stdin-only candidate parsing
was refused, so it is not claimed as successful. Native shell syntax passes;
native staged-material crypto validation remains an approved-rollout gate.

Primary references: [OpenBSD openssl](https://man.openbsd.org/openssl.1),
[relayd TLS keypairs/SNI](https://man.openbsd.org/relayd.conf.5),
[check_http options](https://www.monitoring-plugins.org/doc/man/check_http.html).
The expiry plugin does not replace the hostname/chain-verified TLS acceptance
above. No production key transfer, certificate issuance, configuration
publication or service reload has occurred during preparation.
