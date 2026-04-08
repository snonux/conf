# PI Phase 0.2 Hostname Verification

Task 0.2 for the Raspberry Pi cluster was verified on:

- `pi0.lan.buetow.org`
- `pi1.lan.buetow.org`
- `pi2.lan.buetow.org`
- `pi3.lan.buetow.org`

Verification performed:

- `hostnamectl status --static` returned the expected FQDN on each host.
- `hostname -f` returned the same FQDN on each host.

Outcome:

- No hostname changes were required.
- No reboot was needed because the static hostnames were already correct and resolvable as FQDNs.
