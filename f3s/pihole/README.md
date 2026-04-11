# Pi-hole

Network-wide ad blocking for the f3s cluster.

## Deployment

**Production DNS** runs on the Raspberry Pis **`pi2.lan.buetow.org`** and **`pi3.lan.buetow.org`**: Docker Compose with `network_mode: host` (see `f3s/docs/pi-phase-2-2.md`). Tracked extras live under **`docker-pi/`**:

- `docker-pi/dnsmasq.d/99-f3s-lan-wildcard.conf` — resolves `*.f3s.lan.buetow.org` to the CARP VIP **192.168.1.138** (on pi2/pi3 this file lives in **`~/pihole/etc-dnsmasq.d/`**, which is bind-mounted to `/etc/dnsmasq.d` in compose; then `docker compose restart`).
- `docker-pi/docker-compose.example.yml` — reference `volumes` snippet to merge with your host-local compose.

An ArgoCD Application for Pi-hole on k3s remains in **`f3s/argocd-apps/services/pihole.yaml`** but sync is disabled; the chart values are kept aligned with the Pis’ dnsmasq wildcard.

### Manual Secret Requirement

The admin password is not stored in Git. Before deployment, create the following secret in the `services` namespace:

```bash
kubectl create secret generic pihole-admin-password \
  -n services \
  --from-literal=password='REPLACE_WITH_YOUR_PASSWORD'
```

## Access

- **External**: [https://pihole.f3s.buetow.org](https://pihole.f3s.buetow.org)
- **LAN**: [https://pihole.f3s.lan.buetow.org](https://pihole.f3s.lan.buetow.org)

## DNS Service

Pi-hole answers on **`pi2` / `pi3`** (LAN **192.168.1.127**, **192.168.1.128**, port 53 UDP/TCP). Older docs referred to k3s LoadBalancer IPs on r0–r2; those are not the live Pi-hole path anymore.

### Client Configuration

#### Linux (Fedora/NetworkManager)

##### Quick Toggle (Recommended)

If you have the dotfiles repository, use the toggle script:

```bash
# Toggle Pi-hole DNS on/off
pihole-dns-toggle

# Or use specific commands
pihole-dns-toggle on      # Enable Pi-hole DNS
pihole-dns-toggle off     # Disable Pi-hole (use DHCP DNS)
pihole-dns-toggle status  # Show current status
```

The script is located at `~/git/dotfiles/scripts/pihole-dns-toggle` and automatically detects your active network connection.

##### Manual Configuration

Configure your network connection to use Pi-hole with automatic failover:

```bash
# First, identify your active connection name
nmcli connection show --active

# Configure DNS servers (replace CONNECTION_NAME with your actual connection name from above)
nmcli con mod "CONNECTION_NAME" ipv4.dns "192.168.1.127 192.168.1.128 192.168.1.1"
nmcli con mod "CONNECTION_NAME" ipv4.ignore-auto-dns yes
nmcli con up "CONNECTION_NAME"
```

Example for a WiFi connection named `www_irregular_ninja`:

```bash
nmcli con mod "www_irregular_ninja" ipv4.dns "192.168.1.127 192.168.1.128 192.168.1.1"
nmcli con mod "www_irregular_ninja" ipv4.ignore-auto-dns yes
nmcli con up "www_irregular_ninja"
```

DNS servers are tried in order:
1. Primary: 192.168.1.127 (pi2)
2. Fallback: 192.168.1.128 (pi3)
3. Last resort: 192.168.1.1 (router)

#### Verify Configuration

```bash
# Check configured DNS servers
nmcli dev show | grep DNS

# Check /etc/resolv.conf
cat /etc/resolv.conf

# Test DNS resolution through Pi-hole
dig @192.168.1.127 google.com +short

# Test ad blocking (should return 0.0.0.0)
dig doubleclick.net +short
```

#### Firefox Configuration

If using Firefox, ensure DNS over HTTPS (DoH) is disabled:
1. Open Firefox → Settings → Privacy & Security
2. Scroll to "DNS over HTTPS"
3. Set to "Off" or "Default Protection"

This allows Firefox to use the system DNS (Pi-hole) instead of bypassing it with DoH.

#### Router Configuration (Alternative)

For network-wide Pi-hole usage, configure your router's DHCP server to hand out Pi-hole as the DNS server:
- Primary DNS: 192.168.1.127 (pi2)
- Secondary DNS: 192.168.1.128 (pi3) or 192.168.1.1 (router)

## Storage

On **pi2 / pi3**, Pi-hole state is in the Docker volumes / bind mounts under each host’s `~/pihole` (not NFS). The historical k3s NFS paths (`/data/nfs/k3svolumes/pihole/…`) apply only if the cluster chart is used again.

## Management

On the Pis: `cd ~/pihole && docker compose ps|logs|restart`.

For the dormant k3s deployment, use the `Justfile` (`just status`, `just logs`, `just sync`).
