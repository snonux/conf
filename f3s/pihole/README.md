# Pi-hole

DNS and ad blocking. Production runs on pi2 (192.168.1.127) and pi3
(192.168.1.128), port 53 UDP/TCP, admin UI on port 80
(`http://piN.lan.buetow.org/admin/`, also `pihole.f3s.buetow.org` and
`pihole.f3s.lan.buetow.org`).

## On the Pis

Docker Compose in `~/pihole` on each Pi. The desired image is
`pihole/pihole:2026.10.0` (2026-10-19). Pi2 runs that version; pi3 could not
be upgraded or inspected because SSH
stalls after authentication (its last recorded image is 2026.04.0). Both
still answer DNS.
The pre-upgrade pi2 state archive is root-readable at
`/home/paul/pihole-qf3-pre-upgrade-20261017.tar.gz`.
The stopped-service backup before pi2's 2026.10.0 upgrade is
`/var/backups/f3s-qf3-20261019/pihole-before-2026.10.0.tar.gz` on pi2
(root-only directory, archive mode 0600). It includes the previous compose
file, `.env`, and both state directories; restore the old state as well as
the old image if rolling back. The archive listing was checked before deployment.

To upgrade, change the tag on pi3, `sudo docker compose pull && sudo
docker compose up -d`, check `dig @127.0.0.1`, then repeat on pi2. The containers use
`network_mode: host` and `TZ=Europe/Sofia`.
`WEBPASSWORD` is in the host-local `~/pihole/.env` (same on both, not in
git). The legacy `DNS1`/`DNS2` variables do not describe pi2's effective v6
configuration: its persisted `dns.upstreams` is `8.8.8.8`/`8.8.4.4`.
State lives under `~/pihole`, not NFS. firewalld allows 53/udp, 53/tcp
and http.

Verified on 2026-10-19: pi2's container is healthy with Core 6.5, Web 6.6.1,
and FTL 6.8. Both Pis pass public DNS, ad blocking, and the LAN wildcard over
UDP and TCP; the admin UI loads, unauthenticated version API requests return
401, and an `.invalid` lookup returns NXDOMAIN. Pi3 still stalls after SSH
authentication, including a 60-second retry; DNS health does not establish
that its image is current.

Authenticated `pihole api info/version` also passes on pi2 using its temporary
CLI credential. Interactive admin login remains unverified: the legacy `.env`
password and the saved vault password return 401 on pi2, and the vault password
also returns 401 on the unchanged pi3. Pi2's persisted API password hash matches
the stopped pre-upgrade backup exactly; no password was changed.

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
