# PI Phase 2.2 Pi-hole on pi2/pi3

Task 2.2 for the Raspberry Pi cluster was completed on:

- `pi2.lan.buetow.org`
- `pi3.lan.buetow.org`

Completed actions:

- Created `~/pihole` on each host
- Deployed `pihole/pihole:latest` with `docker compose`
- Used `network_mode: host` so Pi-hole binds directly to the host network stack
- Set `TZ=Europe/Sofia`
- Set `DNS1=1.1.1.1` and `DNS2=1.0.0.1`
- Stored `WEBPASSWORD` in a host-local `~/pihole/.env` file instead of committing it to git
- Installed `bind-utils` so `dig` was available for verification
- Applied firewall changes only after confirming `firewalld` was running

Firewall changes applied on each host:

- `53/udp`
- `53/tcp`
- `http`

Verification:

- `docker compose ps` showed the Pi-hole container running and healthy on both hosts
- `curl -fsI http://localhost/admin/` returned `HTTP/1.1 302 Found`, confirming the admin UI was reachable
- `dig @localhost google.com +short` returned an A record on both hosts

Notes:

- The admin password is intentionally not stored in this repository.
- The same strong host-local password was used on both Pi-hole nodes for simpler failover handling.
- Firewall rule application followed the plan note: `firewall-cmd --state` was checked first, then the port and service rules were added with separate commands.
