# Legacy Joern HTTPS repair (qo3, preparation 2026-10-23)

This change is local preparation. No issuance, production key access/copy,
configuration deployment, DNS change, daemon activation or push was performed.
Fresh review and concrete owner approval are still required. Do not mark qo3
complete until public acceptance passes and the manual custody procedure below
is approved. The selected renewal procedure is operator-assisted custody, as
documented below; it still requires explicit deployment approval.

## Evidence and intended behavior

The strict hostname failure was recorded before the approved 9n3 mirror rollout.
On 2026-10-23 both frontend IPv4 addresses reject verified TLS for all of
`joern.buetow.org`, `www.joern.buetow.org` and `standby.joern.buetow.org`.
Public HTTP redirects each alias to the same alias over HTTPS. Local HTTP
backends serve the same Joern content (9n3 baseline SHA256
`aa74e99cf73020129fd7b102a3c7583421929388f07d0d20af94c168e325064e`).

Joern is absent from `acmeHosts`, so generated ACME configuration, SNI keypairs,
explicit public Host admission/routing and normal topology monitoring omit it.
The existing httpd document roots and manual DNS CNAMEs already include it.
The live Host allowlist rejects Joern while permitting Dory and Ecat. Fixing TLS
alone would therefore leave a 403 after the TLS handshake. This observation
establishes the necessary routing addition; it does not date the original TLS
failure or attribute it to task 4n3.

Public certificate metadata on disk:

| Frontend | Certificate | Observed expiry / SAN |
| --- | --- | --- |
| blowfish | joern | 2026-04-01; joern + www.joern |
| blowfish | standby.joern | 2025-10-21; foo.zone placeholder |
| fishfinger | joern | 2026-01-06; standby.buetow.org placeholder |
| fishfinger | standby.joern | 2026-03-07; standby.joern |

Existing Dory and Ecat bare/www/standby names strictly verify on both frontend
IPv4 addresses. Dory and Ecat bodies match between both frontends. The
**off-DNS standby.ecat copy on blowfish expires 2026-10-23 22:31:51 UTC**;
the copy served at its current DNS destination fishfinger expires 2027-01-12.
That stale peer copy needs separately approved renewal/custody handling.

Other fixed-root legacy sites (`irregular.ninja`, `alt.irregular.ninja` and
`gogios.buetow.org`) and their www/standby aliases also verify and serve 200 on
both IPv4 frontends. Further off-DNS peer copies expire tonight UTC:
standby.alt.irregular.ninja on blowfish at 22:32:48, standby.gogios.buetow.org
on blowfish at 22:33:04, and bare/www.gogios.buetow.org on fishfinger at
22:32:05. Their current DNS destinations have newer certificates. Record these
under separate urgent task **ro3**; this Joern change
does not silently renew or transfer them.

The change adds Joern to the existing topology and adds `check_curl -D` checks
for all nine Joern/Dory/Ecat aliases on each frontend's own IPv4 and IPv6
addresses. They verify trust, hostname, 200 content routing and expiry (warning
20 days, critical 7 days), even when public DNS points to the peer. These checks
run every five minutes and remain active when f3s is deliberately off. They are
`Local: true`, so the passive Gogios frontend runs its own 18 checks too. They
have no non-local ping dependency: curl itself checks public reachability.
`check_http` explicitly does not verify hostname or certificate trust.

## DNS and certificate custody

Keep the existing preview policy: Joern and www.joern CNAME to blowfish;
standby.joern CNAME to fishfinger. No NSD/DNS task is needed. Joining the ACME
list does not generate these DNS records; the manual zone entries remain
unchanged. HTTP-01 requires the correct challenge on the DNS destination for
**every** SAN; requests on the other frontend cannot validate locally.

The current renewal script requests a certificate only on the frontend owning
its A record. It may copy an unrelated placeholder to the other frontend. That
placeholder does not provide valid HTTPS. Running `frontends_acme_invoke` would
also process unrelated certificates and may reload/restart consumers, so it is
not the scoped action for this repair.

Proposed initial custody, requiring explicit approval:

1. Issue `joern.buetow.org` with SAN `www.joern.buetow.org` on blowfish.
2. Issue `standby.joern.buetow.org` on fishfinger.
3. Securely install each resulting chain and matching private key on the other
   frontend. Transfer only these two named pairs over authenticated encrypted
   SSH between private root-owned staging locations. Never route material
   through logs, git, stdout plans, the Joern content mirror or rsync credentials.
   Preserve the owner copy; validate key/certificate correspondence privately
   and verify the public chain/SANs before installation. Do not print key data
   or key fingerprints. Do not reuse an unrelated placeholder as the remedy.
4. Both frontends must hold both valid pairs before admitting the new HTTPS
   route. Each `/etc/ssl/<name>.crt` must reference its matching full chain;
   private keys remain root:wheel 0400 or 0600. Any symlink must resolve to the scoped
   matching certificate, not a legacy unrelated placeholder.

## Selected operator-assisted renewal custody

This repair uses **manual custody**, requiring explicit approval for each
production transfer. It creates no automatic replication, new SSH credential,
SSH authentication rule or certificate platform. Existing owner-side daily ACME
continues unchanged. After **every** owner renewal of either Joern pair, the
operator refreshes that named pair on the peer using the same private staging,
public pin, validator, backup, publication and acceptance steps below. Owner-side
ACME reloads its existing consumers; peer publication requires its own approved
relayd reload. Do not rerun ACME on the non-owner or wait for DNS to fail over.

The 18 local checks per frontend alert at 20 days remaining (critical at 7),
verify chain/name and require HTTP 200. Inspect owner/peer certificate expiry
and public leaf pins after renewal and respond to any peer warning while at
least 20 days remain. The operator records transfer time, certificate name,
source/destination, public leaf pin and expiry in qo3; private material is never
an annotation. A one-off successful copy is not automatic HA renewal. The
unrelated four pairs expiring tonight are tracked in **ro3**, outside qo3's
issuance, key-transfer and activation scope.

## Publishable source and exact runtime delta

The source remains based on published `613310e`, with preparation `6689272`
and a narrow integration of the already reviewed `3cd0355` lineage. Only its
`relayd.conf.tmpl` and relayd data builders from `web.go` are adopted. Published
recipe activation, PF, inetd, httpd listener layout and all 9n3 authentication,
nonroot mirror and documentation files are retained. The unused 3cd loopback
httpd change and all other canonical preparation are excluded.

Fresh read-only comparison on both frontends proves that removing exactly the
following additions from the final rendered files reproduces the live files
byte-for-byte (monitoring equality uses parsed JSON):

| File, per frontend | Only change from live |
| --- | --- |
| httpd.conf | Three Joern port-80 HTTP-01/HTTPS redirect blocks, 33 lines |
| relayd.conf | Two Joern SNI keypair rows and three exact localhost Host passes |
| acme-client.conf | Joern bare/www and standby domain entries, 12 lines |
| acme.sh | Two Joern rows in the existing certificate list |
| gogios.json | 11 standard Joern checks + 18 local strict legacy checks |

Every existing monitoring check and all other configuration fields are equal.
The default Host deny, LAN boundaries, forwarded-header sanitation, dedicated
Forgejo port and direct Traefik NodePort remain identical to live. No DNS,
content, mirror account, rsync credentials, cron schedule or package change is
part of deployment. Source publication and production activation are separate
approvals; do not run the full frontend aggregate or full Gogios setup task.

## Approval scope and private preflight (not executed)

Approve initial HTTP-01 configuration, issuance of exactly the two Joern
certificates on their DNS owners, named-pair transfer in both directions,
root-private staging/backups, publication of the five reviewed configuration
files and two named pairs, bounded httpd/relayd reloads, monitoring publication,
and rollback recovery of a daemon recorded running beforehand. No smtpd
activation is needed. This manual procedure uses reloads; running the Gonf
HTTPD/relayd tasks instead would **restart** changed daemons and requires that
additional interruption in the explicit approval.

Immediately before approved work, repeat the read-only comparisons and normal
SSH host-key verification. Stop for changed live files, an unexpected TLS
layout, an active ACME/configuration mutation, insufficient space or an
unhealthy baseline. Work outside the daily renewal window and allow no other
certificate/configuration writer until acceptance. Keep existing cron unchanged.
The new Local monitoring checks will expose the known ro3 stale peer copy;
that warning is not permission to transfer or renew an unrelated pair.

After approval, on each frontend as root, create an exclusive 0700 directory.
These are examples of **future live-operation commands**, not preparation:

```sh
set -eu
umask 077
work=/root/qo3-legacy-tls-20261023
rcctl check httpd
rcctl check relayd
mkdir -m 700 "$work"
mkdir -m 700 "$work/previous" "$work/material" "$work/material/private"
touch "$work/httpd-was-running" "$work/relayd-was-running"
paths='etc/httpd.conf etc/relayd.conf etc/acme-client.conf usr/local/bin/acme.sh etc/gogios.json'
for name in joern.buetow.org standby.joern.buetow.org; do
    for file in /etc/ssl/$name.fullchain.pem /etc/ssl/private/$name.key; do
        test -f "$file" && test ! -L "$file"
    done
    metadata=$(stat -f '%Su:%Sg:%Lp' /etc/ssl/private/$name.key)
    case "$metadata" in root:wheel:400|root:wheel:600) ;; *) exit 1 ;; esac
    if test -L /etc/ssl/$name.crt; then
        target=$(readlink /etc/ssl/$name.crt)
        case "$target" in "/etc/ssl/$name.fullchain.pem"|"$name.fullchain.pem") ;; *) exit 1 ;; esac
    fi
    paths="$paths etc/ssl/$name.crt etc/ssl/$name.fullchain.pem etc/ssl/private/$name.key"
done
for path in $paths; do
    test -f "/$path" || test -L "/$path"
    mkdir -p "$work/previous/$(dirname "$path")"
    cp -Pp "/$path" "$work/previous/$path"
done
```

Stop if a `.crt` symlink points outside the two expected Joern full-chain
paths, a chain/key is symlinked, or ownership/modes differ from the reviewed
preflight. Do not overwrite backups or reuse this directory on a retry. Keep
all private staging on the frontends, outside the content mirror and controller
disk. Retain backups after acceptance for an explicitly approved rollback.

## Initial issuance and named-pair transfer (not executed)

First install only the reviewed httpd candidate, validate with
`timeout 30 httpd -n -f <candidate>`, then perform the approved
`timeout 30 rcctl reload httpd` on each frontend. Confirm HTTP challenges map
to `/var/www/acme` and aliases redirect to their own HTTPS authority, preserving
path/query. This step does not admit Joern in relayd yet.

Prepare a **private scoped ACME configuration** from the two rendered Joern
domain entries and the existing letsencrypt authority. On blowfish include only
the bare/www entry; on fishfinger include only standby. Keep the existing
`/etc/acme/letsencrypt-privkey.pem` account path and challenge directory; stop
if the existing account is absent. Change only each domain's output paths to
`$work/material/<name>.fullchain.pem` and
`$work/material/private/<name>.key`, using literal absolute paths in the file.
That keeps issuance out of live certificate files. Validate and issue only the
owner's handle:

```sh
# On blowfish: name=joern.buetow.org
# On fishfinger: name=standby.joern.buetow.org
name=joern.buetow.org
work=/root/qo3-legacy-tls-20261023
acme-client -n -f "$work/scoped-acme.conf" "$name"
acme_status=0
acme-client -v -f "$work/scoped-acme.conf" "$name" || acme_status=$?
case "$acme_status" in 0|2) ;; *) exit 1 ;; esac
```

The issuance command contacts the CA and may create the scoped domain key;
it is not authorized by syntax-check approval. No forced renewal or full
`frontends_acme_invoke` is part of this repair. On each owner, derive the public
leaf pin from its issued chain independently of the transfer: run
`openssl x509 -in "$work/material/$name.fullchain.pem" -outform DER -out
"$work/public-leaf.der"`, then `openssl dgst -sha256 "$work/public-leaf.der"`
over pinned SSH. Capture the final 64 lowercase hex digits as `pin`. The initial
owner certificate is not loaded into relayd yet, so a verified live Joern TLS
connection cannot supply the initial pin. Trusted chain and exact SAN validation
below authenticate the freshly issued identity; repeat the owner observation
before transfer. On later renewals, additionally compare a verified public TLS
observation when that owner is serving the renewed pair.

From Bash on the controller, with xtrace disabled and `set -euo pipefail`, stream
only the selected chain/key. For each of the following **two explicit cases**,
set `owner`, `peer` and `name` as listed; do not use a wildcard archive:

| owner | peer | name |
| --- | --- | --- |
| blowfish | fishfinger | joern.buetow.org |
| fishfinger | blowfish | standby.joern.buetow.org |

```bash
ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes "rex@$owner.buetow.org" \
  "doas -n tar -C /root/qo3-legacy-tls-20261023/material -cf - $name.fullchain.pem private/$name.key 2>/dev/null" \
  | ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes "rex@$peer.buetow.org" \
      "doas -n tar -C /root/qo3-legacy-tls-20261023/material -xf - $name.fullchain.pem private/$name.key 2>/dev/null"
```

Never tee, log, capture in a shell variable or redirect the archive to a
controller file. A failed or ambiguous transfer requires inspection of private
staging and public certificate metadata before another attempt. After both
transfers, run the committed validator on each frontend for **both** names,
with that name's independently observed owner pin. Each staged chain and key
must remain a regular file, never a symlink; the only live symlink is its
matching `.crt` alias. The existing standby renewal helper removes symlinked
key/full-chain pairs before renewal, so using such links for peer custody would
lose the installed material:

```sh
ssh -T -oBatchMode=yes -oStrictHostKeyChecking=yes "rex@$frontend.buetow.org" \
  "doas -n sh -s -- $name /root/qo3-legacy-tls-20261023/material/$name.fullchain.pem /root/qo3-legacy-tls-20261023/material/private/$name.key $pin /etc/ssl/cert.pem" \
  < frontends/scripts/check-joern-certificate.sh
```

The validator allows only the two named certificates, requires exactly the
bare/www or standby DNS SANs, the public pin, a trusted server chain, over 30
days remaining and a matching private key. Symlinked, encrypted, malformed,
untrusted, wrong-name, wrong-pin and mismatched inputs fail without printing
key material or prompting. Every validator invocation reads only staged keys
**after approval**; preparation uses isolated synthetic fixtures.

## Publication, acceptance and rollback (not executed)

Publish one frontend at a time. Both named pairs must already be validated on
both frontends. Stage each chain beside `/etc/ssl/<name>.fullchain.pem` and key
beside `/etc/ssl/private/<name>.key` with `mktemp`, then `install -o root -g
wheel -m 0444 <staged-chain> <chain-temp>` and `install -o root -g wheel -m 0400
<staged-key> <key-temp>`. Rename the key and chain to their two named live paths
with `mv -f`; this is **not** an atomic two-file transaction. Stage the `.crt`
symlink to its own absolute full-chain path beside its final destination, then
rename it into place. Existing loaded relayd material remains in use until the
approved reload. Stop and use rollback after any partial publication failure.

Validate the reviewed final candidates against the now-published valid pairs:
`timeout 30 httpd -n -f <httpd-candidate>`,
`timeout 30 relayd -n -f <relayd-candidate>`, and
`acme-client -n -f <acme-candidate> <name>` for both names. Stage all five
configuration files beside their destinations, preserving expected root:wheel
modes (httpd/ACME config 0644, relayd 0600, renewal script 0755, Gogios 0744),
then rename. Reload only changed httpd/relayd with `timeout 30 rcctl reload
<daemon>` and confirm `rcctl check <daemon>`. Do not invoke package, cron,
inetd, PF, SMTPD, DNS or aggregate tasks. A failed reload is not permission for
an unapproved restart; inspect the daemon and execute the approved rollback.

For acceptance, strictly verify all nine mirrored legacy aliases on both
frontend IPv4 and IPv6 addresses with correct SNI and Host. Require Joern 200
and the expected body hash, usable HTTP redirects preserving path/query,
identical mirror content, unchanged `_joernsync` cron/authentication and all 18
Local checks per frontend present without dependencies. Retest known public
Hosts and unknown/LAN/suffix/case/port/missing/duplicate Host and WebSocket
negatives. Confirm the passive frontend actually runs its local checks, and
record manual renewal custody acceptance. The known ro3 peer-expiry finding is
separate and must be reported accurately.

If acceptance fails, restore only the files backed up above, on that same
frontend. The following root-shell rollback stages **all** backups before any
replacement and preserves `.crt` symlinks with `cp -Pp`. It restores the known
Joern outage as the baseline; it does not revoke or delete issued material:

```sh
set -eu
umask 077
work=/root/qo3-legacy-tls-20261023
test -f "$work/httpd-was-running"
test -f "$work/relayd-was-running"
manifest=$(mktemp "$work/restore-manifest.XXXXXXXX")
paths='etc/httpd.conf etc/relayd.conf etc/acme-client.conf usr/local/bin/acme.sh etc/gogios.json'
for name in joern.buetow.org standby.joern.buetow.org; do
    paths="$paths etc/ssl/$name.crt etc/ssl/$name.fullchain.pem etc/ssl/private/$name.key"
done
for path in $paths; do
    source=$work/previous/$path
    test -f "$source" || test -L "$source"
    temporary=$(mktemp "$(dirname "/$path")/.qo3-restore.XXXXXXXX")
    cp -Pp "$source" "$temporary"
    printf '%s %s\n' "$temporary" "/$path" >> "$manifest"
done
while read -r temporary destination; do
    mv -f "$temporary" "$destination"
done < "$manifest"
timeout 30 httpd -n -f /etc/httpd.conf
timeout 30 relayd -n -f /etc/relayd.conf
for daemon in httpd relayd; do
    status=0
    timeout 10 rcctl check "$daemon" >/dev/null 2>&1 || status=$?
    case "$status" in
        0) timeout 30 rcctl reload "$daemon" ;;
        1) test -f "$work/$daemon-was-running"; timeout 30 rcctl start "$daemon" ;;
        *) printf 'Unknown daemon state; stop for inspection.\n' >&2; exit 1 ;;
    esac
    timeout 10 rcctl check "$daemon"
done
```

The backup/manifest paths are root-private and contain fixed controlled names;
no key bytes enter stdout. Failed staging leaves live files untouched. A failed
rename, validation or activation requires inspection of retained files and
daemon state; do not claim an atomic multi-file rollback or retry blindly.
Starting a stopped daemon is allowed only by explicit rollback approval and a
recorded running baseline. No restart, DNS failover, SSH change, power action or
destructive cleanup is implied. Preserve private backups and newly issued
material for the operator.

## Validation completed

`python3 frontends/scripts/tests/legacy-tls.py` runs the real Gonf plan and
production template engine. It checks HTTP-01, SAN/standby/SNI coverage, Host
routes and 36 strict monitoring checks. Missing keypair, missing www SAN,
removed verification, non-local checks,
non-local dependencies, removed default deny and unauthorized Host admission
negative controls must fail. No `*_test.go` is added.

The committed validator has synthetic success cases for both certificates and
negative cases for scope, pin, SANs, short lifetime, untrusted CA, mismatched key,
symlinked input, encrypted key and malformed key. ShellCheck passes.

On both OpenBSD 7.9 frontends, bounded `httpd -n`, `relayd -n` and
`acme-client -n` passed for the **actual final rendered configurations**; an
invalid relayd-directive negative failed as expected. Every candidate keypair reference
was redirected into an isolated synthetic fixture, with a fresh synthetic key,
so no production private key was read or copied. Parser fixtures were retained
under `/tmp/qo3-rendered-native.QBKivi4v` (blowfish) and
`/tmp/qo3-rendered-native.EtYxsYES` (fishfinger). The committed validator also
passed native LibreSSL validation and rejected the wrong scoped SAN on both
hosts. All ACME account/domain key paths were isolated synthetic paths too.
The local preparation evidence and reproduction script are in the worktree sibling `../ops/` directory.

Read-only HTTPS requests used verified Dory SNI on both public addresses of both
families: Dory/Ecat 200, Joern/unknown/LAN/suffix/case/port/missing Host 403,
duplicate Host 400, and WebSocket/Connection headers did not bypass denial.
Native installed check_curl 3.0.3 accepted Dory with `-D -C 20,7
--continue-after-certificate` and rejected Joern with certificate hostname error
60. This is a pre-deployment baseline, not a candidate runtime test.

Bounded `-p=1` Go build/vet and frontend errcheck pass. Existing ACME shell
regressions pass. No task method descriptions changed, so no generation delta.

Protocol references: [OpenBSD acme-client](https://man.openbsd.org/acme-client.1)
and [acme-client.conf](https://man.openbsd.org/acme-client.conf.5).
