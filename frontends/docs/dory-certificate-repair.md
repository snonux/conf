# Dory standby certificate repair (b04)

On 2026-11-03, Gogios warned for both address families of
`standby.dory.buetow.org` on blowfish. HTTPS still returned 200, but its local
certificate was due to expire on 2026-11-20. Fishfinger, the current DNS owner, already
had a renewed certificate expiring on 2027-01-12 at 22:32:39 UTC.

The daily ACME script deliberately skips certificates whose DNS points to the
other frontend. Its renewal therefore did not update this off-DNS copy.
Gogios's host-local legacy checks correctly caught the stale certificate before
a DNS failover. No monitoring threshold or check was removed.

## Repair

Only fishfinger's `standby.dory.buetow.org` full chain and matching private key
were streamed over authenticated SSH into root-only staging on blowfish. No
private material was written to the controller. Before publication, both the
owner and staged pair passed exact SAN, trusted server chain, more than 30 days
remaining, key-match and independent public leaf fingerprint checks:

`8666f97d7c196969af3a96a18a15e344c95a0835d84e38a20fae43f832cfe94d`

Blowfish's previous chain, key and `.crt` symlink were archived under
`/root/b04-dory-20261103/original.tar` (root-only). The new regular chain/key
files retain root:wheel ownership and modes 0444/0400. The existing `.crt`
alias still points to its matching full chain. Native `relayd -n`, reload and
service checks passed. DNS, ACME configuration, fishfinger's files and other
certificate pairs were unchanged.

## Future renewal

This was a targeted manual repair, not automatic certificate replication.
After the DNS owner renews this certificate, refresh its peer copy before the
20-day warning boundary. The named-pair staging, validation, backup and reload
procedure in [legacy TLS](legacy-tls.md#selected-operator-assisted-renewal-custody)
also describes the custody model; its Joern-only validator must not be used
unchanged for Dory. For Dory, require exactly the SAN
`DNS:standby.dory.buetow.org` and obtain the expected public fingerprint anew
from the DNS owner. Transfer only that explicitly named chain/key pair, never
wildcard archives. Retain the old pair until strict HTTPS checks pass on both
address families; restore it and reload relayd if acceptance fails.
