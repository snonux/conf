# Legacy Joern HTTPS repair (qo3, preparation 2026-10-23)

This change is local preparation. No issuance, production key access/copy,
configuration deployment, DNS change, daemon activation or push was performed.
Fresh review and concrete owner approval are still required. Do not mark qo3
complete until verified public acceptance and the renewal custody decision below
are resolved.

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
for the same separately approved certificate-custody work; this Joern change
does not silently renew or transfer them.

The change adds Joern to the existing topology and adds `check_curl -D` checks
for all nine Joern/Dory/Ecat aliases on each frontend's own IPv4 and IPv6
addresses. They verify trust, hostname, 200 content routing and expiry (warning
20 days, critical 7 days), even when public DNS points to the peer. These checks
run every five minutes and remain active when f3s is deliberately off.
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
   private keys remain root:wheel 0600. Any symlink must resolve to the scoped
   matching certificate, not a legacy unrelated placeholder.

**Durable peer renewal is an unresolved deployment condition.** The source
change deliberately does not add a new credential, key-reading cron or transfer
policy. Existing owner-side daily ACME renewal will not refresh its peer copy.
Before accepting the repair, the owner must approve either scoped automatic
certificate replication with separately reviewed authentication, or a documented
operator procedure that repeats the two-pair transfer after every renewal and
responds to the new 20-day peer-expiry warnings. A one-off successful transfer
must not be described as automatic HA renewal. No DNS rotation or broad SSH
policy change is part of this proposal.

## Scoped deployment proposal (not executed)

The published base 613310e predates the live reviewed 4n3 web policy. Its full
`frontends_httpd` / `frontends_relayd` plans would overwrite that policy. **Do not
deploy those plans from this branch.** First integrate the topology/monitoring
change into the exact source for the reviewed live configuration and compare
rendered candidates to both live files. Alternatively prepare an explicitly
reviewed incremental candidate from each current live file. Expected web delta:
three exact Joern port-80 HTTP-01/HTTPS redirect blocks, two TLS keypair names,
and three exact `pass request header "Host" ... forward to <localhost>` rules
after the existing default deny. Keep all other rules and listener/backend
settings byte-for-byte, including forwarded-header sanitation and public/LAN
isolation. Never replace the default deny with a broad match or wildcard pass.

After separate approval covering issuance, named-pair custody and activation:

1. Save root-private local rollback copies of the two frontend web files, ACME
   configuration and only the named Joern certificate material. Preserve modes,
   owners and symlink structure. Prevent an overlapping daily ACME run during
   the bounded issuance/activation window; preserve its prior scheduling state.
2. Install only the two Joern domain declarations in each frontend's existing
   ACME configuration and the three HTTP-01 blocks in its existing httpd file.
   Validate `httpd -n -f <candidate>` and
   `acme-client -n -f <candidate> joern.buetow.org` (also standby) before live
   replacement. Reload only httpd if needed. Confirm challenge location mapping
   and HTTP redirects on both families without creating real CA challenges.
3. Request the bare/www pair on blowfish using
   `acme-client -v joern.buetow.org`; request the standby pair on fishfinger
   using `acme-client -v standby.joern.buetow.org`. Use the reviewed scoped
   configuration via `-f` if needed. These commands contact the CA and are
   **not authorized by preparation or parser approval**. Treat exit 0 as changed,
   2 as unchanged, and 1 as failure. Do not force unrelated renewals.
4. Complete the approved custody transfer, then validate the final relayd
   candidate on each frontend. Activate one frontend at a time and perform
   strict certificate/SNI/Host checks before proceeding to the other. Reload
   only relayd; if a restart is needed instead, that must be included in the
   explicit activation approval. No smtpd action is needed for site certificates.
5. Install the reviewed monitoring candidate and restore the unchanged renewal
   schedule. Record the approved ongoing peer-renewal procedure.

For acceptance, strictly verify all nine legacy aliases on both frontend IPv4
and IPv6 addresses with the correct SNI and Host. Require Joern 200 and the
expected body hash, usable HTTP redirects preserving path/query, identical
mirrored content, and successful existing `_joernsync` cron/authentication
checks. Retest known public Hosts and unknown/LAN/suffix/case/port/missing/duplicate
Host and WebSocket bypass negatives. Keep f3s service placement, nonroot mirror
accounts, filesystem ownership and authentication unchanged. A parser pass
cannot establish live TLS or routing acceptance.

Rollback: restore only the changed web/ACME/monitoring files from each host's
own private rollback set, validate natively, then apply only the previously
approved daemon activation. Restore previous Joern certificate material only
when required to restore a consumer; do not delete or revoke newly issued
material. Leave DNS, other certificates, mirror data/accounts and credentials
untouched. Restoring the baseline also restores its known Joern outage; record
that accurately. No automatic restart, DNS failover or power action is implied.

## Validation completed

`python3 frontends/scripts/tests/legacy-tls.py` runs the real Gonf plan and
production template engine. It checks HTTP-01, SAN/standby/SNI coverage, Host
routes and 36 strict monitoring checks. Missing keypair, missing www SAN and
removed verification negative controls must fail. No `*_test.go` is added.

On both OpenBSD 7.9 frontends, bounded `httpd -n`, `relayd -n` and
`acme-client -n` passed for incremental live-policy candidates; an invalid
relayd-directive negative failed as expected. Every candidate keypair reference
was redirected into an isolated synthetic fixture, with a fresh synthetic key,
so no production private key was read or copied. Parser fixtures were retained
under `/tmp/qo3-native.rDE0XMdg` (blowfish) and
`/tmp/qo3-native.L5cS9WtT` (fishfinger). The local preparation evidence and
reproduction script are in the worktree sibling `../ops/` directory.

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
