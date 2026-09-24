# PI Phase 3.2 Repository Updates

Task 3.2 originally recorded the Raspberry Pi role split. The current state is:

- `pi0.lan.buetow.org` and `pi1.lan.buetow.org` run NetBSD 11.0 and serve static HTTP content on port 80 with `bozohttpd`
- `pi2.lan.buetow.org` and `pi3.lan.buetow.org` serve Pi-hole DNS on port 53 and the admin UI on port 80

Monitoring inventory:

- HTTP checks should target `http://pi0.lan.buetow.org` and `http://pi1.lan.buetow.org`
- Pi-hole checks should verify DNS resolution of `google.com` against `pi2` and `pi3`
- Pi-hole admin checks should target `http://pi2.lan.buetow.org/admin/` and `http://pi3.lan.buetow.org/admin/`

Runbook notes:

- `bozohttpd` is provided by the NetBSD base system and serves the static vhosts; pi0 remains the content source and pi1 pulls hourly
- On NetBSD pi0/pi1, validate NPF and its active TCP 22/80/2222 rules. On
  Rocky pi2/pi3 only, check `firewall-cmd --state` and skip firewalld changes
  when it is not running.
- DNS and admin access on the Pi-hole nodes are intentionally exposed on the host network, so the operational checks should use direct LAN hostnames rather than Kubernetes ingress paths

Verification commands used during the phase:

```bash
curl -fsI http://pi0.lan.buetow.org
curl -fsI http://pi1.lan.buetow.org
curl -fsI http://pi2.lan.buetow.org/admin/
curl -fsI http://pi3.lan.buetow.org/admin/
dig @pi2.lan.buetow.org google.com +short
dig @pi3.lan.buetow.org google.com +short
```

For current static-node service, synchronization, and failover acceptance, see
[`../pi-netbsd/NETBSD-11-DUAL-NODE-ACCEPTANCE.md`](../pi-netbsd/NETBSD-11-DUAL-NODE-ACCEPTANCE.md).
