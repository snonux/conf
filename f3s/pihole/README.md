# Pi-hole

DNS and ad blocking. Production runs on pi2 (192.168.1.127) and pi3
(192.168.1.128), port 53 UDP/TCP, admin UI on port 80
(`http://piN.lan.buetow.org/admin/`, also `pihole.f3s.buetow.org` and
`pihole.f3s.lan.buetow.org`).

## On the Pis

Docker Compose in `~/pihole` on each Pi, `pihole/pihole:latest`,
`network_mode: host`, `TZ=Europe/Sofia`, upstreams `1.1.1.1`/`1.0.0.1`.
`WEBPASSWORD` is in the host-local `~/pihole/.env` (same on both, not in
git). State lives under `~/pihole`, not NFS. firewalld allows 53/udp, 53/tcp
and http.

Tracked here:

- `docker-pi/dnsmasq.d/99-f3s-lan-wildcard.conf`: `*.f3s.lan.buetow.org` ->
  CARP VIP 192.168.1.138. On the Pis it goes into `~/pihole/etc-dnsmasq.d/`
  (bind-mounted to `/etc/dnsmasq.d`), then `docker compose restart`.
- `docker-pi/docker-compose.example.yml`: the volume snippet to merge.

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
