# Pi-hole

Network-wide ad blocking for the f3s cluster.

## Deployment

Pi-hole is deployed via ArgoCD using a combination of a local Helm chart (for PVs/PVCs/Ingress) and the official upstream chart.

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

Pi-hole DNS is available on both the Wireguard mesh and LAN networks:
- **Wireguard mesh**: 192.168.2.120 (port 53 UDP/TCP)
- **LAN IPs**: 192.168.1.120, 192.168.1.121, 192.168.1.122 (port 53 UDP/TCP)

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
nmcli con mod "CONNECTION_NAME" ipv4.dns "192.168.1.120 192.168.1.121 192.168.1.122 192.168.1.1"
nmcli con mod "CONNECTION_NAME" ipv4.ignore-auto-dns yes
nmcli con up "CONNECTION_NAME"
```

Example for a WiFi connection named `www_irregular_ninja`:

```bash
nmcli con mod "www_irregular_ninja" ipv4.dns "192.168.1.120 192.168.1.121 192.168.1.122 192.168.1.1"
nmcli con mod "www_irregular_ninja" ipv4.ignore-auto-dns yes
nmcli con up "www_irregular_ninja"
```

DNS servers are tried in order:
1. Primary: 192.168.1.120 (r0)
2. Fallback: 192.168.1.121 (r1)
3. Fallback: 192.168.1.122 (r2)
4. Last resort: 192.168.1.1 (router)

#### Verify Configuration

```bash
# Check configured DNS servers
nmcli dev show | grep DNS

# Check /etc/resolv.conf
cat /etc/resolv.conf

# Test DNS resolution through Pi-hole
dig @192.168.1.120 google.com +short

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
- Primary DNS: 192.168.1.120
- Secondary DNS: 192.168.1.121 (or 192.168.1.1 for fallback to router)

## Storage

Configuration is persisted on NFS at:
- `/data/nfs/k3svolumes/pihole/config`
- `/data/nfs/k3svolumes/pihole/dnsmasq`

## Management

Use the provided `Justfile` for common operations:

```bash
just status    # Check pod and service status
just logs      # Follow logs
just sync      # Trigger ArgoCD refresh
```
