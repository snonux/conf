# Pi-hole source guard (pi2/pi3)

`f3s-guard.nft` is the live host policy of both resolvers (pi2
`10.20.0.127`, pi3 `10.20.0.128`, `eth0`). It owns only `inet f3s_guard`,
hooked at prerouting -150, and leaves firewalld and Docker alone.

- Trusted: lab `10.20.0.0/24` and lab ULA `fd02:ca0b:f1cb:20::/64` to the
  host's own lab addresses, and the old physical ULA `fd22:c702:acb7::/64`
  (IPv6 compatibility).
- wg2 (routed by triggerfish): laptop `10.99.0.2` and phone `10.99.0.3` get
  TCP/UDP 53; the laptop also TCP 22/80/443, the phone TCP 443. These match
  the router's own wg2 permits.
- ICMP/ICMPv6 and DHCPv6 replies pass; everything else new from `eth0` to a
  local address is dropped. Wi-Fi and camera subnets get no trust.

Pi-hole's `LOCAL` listening mode rejects routed wg2 clients, so the compose
file (`f3s/pihole/docker-pi/docker-compose.yml`) uses `SINGLE` on `eth0` and
leaves source authorisation to this guard.

## Changing the policy

A change is a guarded operation, one Pi at a time, never both resolvers at
once:

1. `./gonf.sh cluster rocky-pis pihole_firewall_stage` installs the file as
   `/etc/f3s-firewall/candidate.nft` (Operational; it loads nothing).
2. On the Pi, with two independent SSH sessions open: `f3s-firewall check`,
   `f3s-firewall apply "$SSH_CONNECTION"`, then `f3s-firewall confirm
   "$SSH_CONNECTION"` from a fresh connection before the rollback timer
   fires. Confirm persists it as `confirmed.nft`, the boot policy.
3. Check UDP/TCP DNS on that Pi before moving to the other one.

Never load the file with bare `nft -f` on a host.

`python3 check-firewall.py` runs packet checks against the real file in
disposable user/network namespaces; it contacts no host.

## History

The guard and the resolvers' move to the lab were task 524 (accepted
2026-11-07). i24 and j24 retired the old `192.168.1.127/.128` addresses and
the `10.98.0.127/.128` recovery aliases (2026-11-11), 074 dropped the old
`192.168.1.0/24` trust. The staging tasks (`pihole_migration_*`,
`pihole_firewall_migration_stage`) went in 774, the NetworkManager migration
helper in c74; this directory was `network-migration/` (file `firewall.nft`)
until f74. The full shared-wire record, with its receipts under
`524-pis-20261107/`, is in conf history
(`git log -- gonf/pihole/assets/network-migration/README.md`) and in IQ
`topics/infra/openbsdrouter-plan/HANDOFF.md`.
