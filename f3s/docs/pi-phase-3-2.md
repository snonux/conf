# PI Phase 3.2 Repository Updates

Task 3.2 records the final Raspberry Pi role split used by the f3s cluster:

- `pi0.lan.buetow.org` and `pi1.lan.buetow.org` serve static HTTP content on port 80 with `lighttpd`
- `pi2.lan.buetow.org` and `pi3.lan.buetow.org` serve Pi-hole DNS on port 53 and the admin UI on port 80

Monitoring inventory:

- HTTP checks should target `http://pi0.lan.buetow.org` and `http://pi1.lan.buetow.org`
- Pi-hole checks should verify DNS resolution of `google.com` against `pi2` and `pi3`
- Pi-hole admin checks should target `http://pi2.lan.buetow.org/admin/` and `http://pi3.lan.buetow.org/admin/`

Runbook notes:

- `lighttpd` was chosen for the HTTP nodes because the Pis have limited RAM and the workload is static-only
- Firewall rules on the Pis are conditional: check `firewall-cmd --state` first and skip `firewall-cmd` changes entirely if `firewalld` is not running
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
