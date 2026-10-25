# Pi-hole

DNS and ad blocking. Production runs on pi2 (192.168.1.127) and pi3
(192.168.1.128), port 53 UDP/TCP, admin UI on port 80
(`http://piN.lan.buetow.org/admin/`, also `pihole.f3s.buetow.org` and
`pihole.f3s.lan.buetow.org`).

## On the Pis

Docker Compose in `~/pihole` on each Pi. The desired image is
`pihole/pihole:2026.10.0`. Both Pis run that version as of 2026-10-25.
Pi3 was upgraded after the approved logind repair restored prompt SSH/sudo;
no rack power cycle or reboot was needed. See the
[recovery record](pi3-recovery-plan.md) for scope, rollback and verification.
The pre-upgrade pi2 state archive is root-readable at
`/home/paul/pihole-qf3-pre-upgrade-20261017.tar.gz`.
The stopped-service backup before pi2's 2026.10.0 upgrade is
`/var/backups/f3s-qf3-20261019/pihole-before-2026.10.0.tar.gz` on pi2
(root-only directory, archive mode 0600). It includes the previous compose
file, `.env`, and both state directories; restore the old state as well as
the old image if rolling back. The archive listing was checked before deployment.

For an upgrade, change the tracked image pin, pre-pull it, and back up the
stopped service state before applying only `pihole_compose` to one Pi. Verify
DNS, UI and the native authenticated API before touching the other resolver.
Restore both the stopped state and old image if rollback is required. The containers use
`network_mode: host` and `TZ=Europe/Sofia`.
`WEBPASSWORD` is in the host-local `~/pihole/.env` (same on both, not in
git). The legacy `DNS1`/`DNS2` variables do not describe pi2's effective v6
configuration: its persisted `dns.upstreams` is `8.8.8.8`/`8.8.4.4`.
State lives under `~/pihole`, not NFS. firewalld allows 53/udp, 53/tcp
and http.

Verified on 2026-10-25: both containers are healthy with Core 6.5, Web 6.6.1
and FTL 6.8. Both Pis pass public DNS, ad blocking, wildcard and apex records
over UDP/TCP, plus `.invalid` NXDOMAIN. Each admin URL redirects to an HTML
Pi-hole login page with final HTTP 200; unauthenticated version APIs return 401.
Pi3 fresh SSH/sudo now completes in under five seconds. Pi2 has the same
approximately 120-second session delay and missing login1 bus owner; it was
kept read-only during this maintenance and no Pi2 repair is claimed.

Authenticated `pihole api info/version` passes on both Pis using the temporary
CLI credential. Interactive admin login remains unverified: the legacy `.env`
password and the saved vault password return 401 on pi2, and the vault password
also returns 401 on the unchanged pi3. Pi2's persisted API password hash matches
the stopped pre-upgrade backup exactly; no password was changed. Pi3's `.env`,
persisted API password hash and upstream settings also match its stopped backup.
Its root-private archive is
`/var/backups/f3s-1g3-20261025.Lml32C/pihole-before-2026.10.0.tar.gz` on Pi3
(parent 0700, archive 0600). No application state was recursively chowned.

Tracked here:

- `docker-pi/dnsmasq.d/99-f3s-lan-wildcard.conf`: `*.f3s.lan.buetow.org` ->
  CARP VIP 192.168.1.138. On the Pis it goes into `~/pihole/etc-dnsmasq.d/`
  (bind-mounted to `/etc/dnsmasq.d`), then `docker compose restart`.
- `docker-pi/docker-compose.yml`: the live compose file, deployed with the
  wildcard to `~/pihole` on pi2/pi3 by gonf (`./gonf.sh cluster rocky-pis pihole`).

```sh
cd ~/pihole && docker compose ps | logs | restart
dig @192.168.1.127 google.com +short
dig @192.168.1.127 doubleclick.net +short     # 0.0.0.0
curl -fsI http://pi2.lan.buetow.org/admin/    # 302
```

If one Pi is down the other still answers; clients list both.

## Clients

Fedora: `pihole-dns-toggle [on|off|status]` from `~/git/dotfiles/scripts/`,
or by hand:

```sh
nmcli connection show --active
nmcli con mod "<conn>" ipv4.dns "192.168.1.127 192.168.1.128 192.168.1.1"
nmcli con mod "<conn>" ipv4.ignore-auto-dns yes
nmcli con up "<conn>"
```

Firefox: DNS over HTTPS off (or Default Protection), else it bypasses
Pi-hole. Router DHCP: hand out 192.168.1.127 and 192.168.1.128.

## k3s chart (dormant)

`helm-chart/` and `argocd-apps/services/pihole.yaml.disabled` are kept
(disabled: the Pis serve DNS), with `syncPolicy` commented out, and the values
match the Pis' wildcard. Rename back to `.yaml` and apply to revive it. It
needs secret `pihole-admin-password` (key `password`) in `services`, and
would use `/data/nfs/k3svolumes/pihole/`. `just status | logs | sync`.
