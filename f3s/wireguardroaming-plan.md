# Plan: Add Fedora Laptop (earth) and Android Phone (pixel7pro) as WireGuard VPN Clients

## Overview
Add two new roaming clients (earth - Fedora laptop, pixel7pro - Android phone) to the existing WireGuard full-mesh VPN, connecting them to all 8 existing hosts (f0-f2, r0-r2, blowfish, fishfinger). 

## Background
- Current VPN: Full mesh of 8 hosts (3 FreeBSD, 3 Rocky Linux, 2 OpenBSD)
- VPN network: 192.168.2.0/24
- Mesh generator: `/home/paul/git/wireguardmeshgenerator/`
- Generator creates configs and deploys via SSH to remote hosts
- Reference: https://foo.zone/gemfeed/2025-05-11-f3s-kubernetes-with-freebsd-part-5.html

## Challenge: Roaming Client Support
The current generator doesn't properly handle roaming clients (devices behind NAT that need PersistentKeepalive to all peers). The existing logic only sets PersistentKeepalive for LAN-to-internet connections, but roaming clients need it for ALL connections to maintain NAT traversal.

## Implementation Steps

### 1. Modify WireGuard Mesh Generator to Support Roaming Clients

**File:** `/home/paul/git/wireguardmeshgenerator/wireguardmeshgenerator.rb`

**Changes needed in the `WireguardConfig#peers` method (lines 149-163):**

Current logic:
```ruby
keepalive = in_lan && !peer_in_lan
```

New logic should detect roaming clients (hosts with neither 'lan' nor 'internet' keys) and enable keepalive for all their peer connections:

```ruby
# Detect if current host is a roaming client (no lan or internet section)
is_roaming = !hosts[myself].key?('lan') && !hosts[myself].key?('internet')

# Set keepalive: LAN hosts connecting to internet hosts, OR roaming clients connecting to anyone
keepalive = is_roaming || (in_lan && !peer_in_lan)
```

**Alternative simpler approach:**
For roaming clients, set keepalive for all peers since they're always behind NAT:
```ruby
# Check if current host is roaming (no fixed location)
is_roaming = !hosts[myself].key?('lan') && !hosts[myself].key?('internet')
keepalive = is_roaming || (in_lan && !peer_in_lan)
```

### 2. Add Laptop and Phone to YAML Configuration

**File:** `/home/paul/git/wireguardmeshgenerator/wireguardmeshgenerator.yaml`

Add two new host entries (after the existing 8 hosts):

```yaml
  earth:
    os: Linux
    wg0:
      domain: 'wg0.wan.buetow.org'
      ip: '192.168.2.200'
    # Note: No 'lan' or 'internet' section = roaming client
    # Note: No 'ssh' section = manual installation

  pixel7pro:
    os: Android
    wg0:
      domain: 'wg0.wan.buetow.org'
      ip: '192.168.2.201'
    # Note: No 'lan' or 'internet' section = roaming client
    # Note: No 'ssh' section = manual installation
```

**Key design decisions:**
- IP addresses: 192.168.2.200 (earth), 192.168.2.201 (pixel7pro)
- No `lan` or `internet` sections → identified as roaming clients
- No `ssh` section → configs will be manually installed (not via `rake install`)
- `os` field for documentation purposes

### 3. Update All Hosts to Include New Clients

**File:** `/home/paul/git/wireguardmeshgenerator/wireguardmeshgenerator.yaml`

Each existing host (f0-f2, r0-r2, blowfish, fishfinger) will automatically include earth and pixel7pro as peers when configs are regenerated. No changes needed to existing host definitions.

### 4. Generate New Configurations

**Command:**
```bash
cd /home/paul/git/wireguardmeshgenerator
rake generate
```

This generates new `wg0.conf` files in `dist/` for all 10 hosts (8 existing + 2 new).

**Expected output:**
```
dist/
├── blowfish/etc/wireguard/wg0.conf
├── earth/etc/wireguard/wg0.conf     ← NEW
├── f0/etc/wireguard/wg0.conf
├── f1/etc/wireguard/wg0.conf
├── f2/etc/wireguard/wg0.conf
├── fishfinger/etc/wireguard/wg0.conf
├── pixel7pro/etc/wireguard/wg0.conf ← NEW
├── r0/etc/wireguard/wg0.conf
├── r1/etc/wireguard/wg0.conf
└── r2/etc/wireguard/wg0.conf
```

### 5. Deploy Updated Configs to Existing Hosts

**Command:**
```bash
cd /home/paul/git/wireguardmeshgenerator
rake install
```

OR selectively update only existing hosts:
```bash
ruby wireguardmeshgenerator.rb --install --hosts=f0,f1,f2,r0,r1,r2,blowfish,fishfinger
```

This updates all 8 existing hosts to include earth and pixel7pro in their peer lists. The script will SSH to each host, upload the config, and reload WireGuard.

### 6. Update /etc/hosts on All Participating Hosts

Add DNS entries for the new VPN clients to all hosts in the mesh for easier access.

**On each of the 8 existing hosts (f0-f2, r0-r2, blowfish, fishfinger):**

Add these lines to `/etc/hosts`:
```
192.168.2.200   earth.wg0.wan.buetow.org earth
192.168.2.201   pixel7pro.wg0.wan.buetow.org pixel7pro
```

**Manual approach:**
```bash
# On each host (f0, f1, f2, r0, r1, r2, blowfish, fishfinger)
echo "192.168.2.200   earth.wg0.wan.buetow.org earth" | sudo tee -a /etc/hosts
echo "192.168.2.201   pixel7pro.wg0.wan.buetow.org pixel7pro" | sudo tee -a /etc/hosts
```

**On earth (laptop), add entries for all mesh hosts:**
```bash
# Add to /etc/hosts
echo "# WireGuard mesh hosts" | sudo tee -a /etc/hosts
echo "192.168.2.130   f0.wg0.wan.buetow.org f0" | sudo tee -a /etc/hosts
echo "192.168.2.131   f1.wg0.wan.buetow.org f1" | sudo tee -a /etc/hosts
echo "192.168.2.132   f2.wg0.wan.buetow.org f2" | sudo tee -a /etc/hosts
echo "192.168.2.120   r0.wg0.wan.buetow.org r0" | sudo tee -a /etc/hosts
echo "192.168.2.121   r1.wg0.wan.buetow.org r1" | sudo tee -a /etc/hosts
echo "192.168.2.122   r2.wg0.wan.buetow.org r2" | sudo tee -a /etc/hosts
echo "192.168.2.110   blowfish.wg0.wan.buetow.org blowfish" | sudo tee -a /etc/hosts
echo "192.168.2.111   fishfinger.wg0.wan.buetow.org fishfinger" | sudo tee -a /etc/hosts
echo "192.168.2.201   pixel7pro.wg0.wan.buetow.org pixel7pro" | sudo tee -a /etc/hosts
```

**Note:** The WireGuard mesh generator doesn't automatically manage /etc/hosts, so this is a manual step.

### 7. Install WireGuard on Fedora Laptop (earth)

**Commands on earth:**
```bash
# Install WireGuard
sudo dnf install wireguard-tools

# Copy generated config
sudo cp /home/paul/git/wireguardmeshgenerator/dist/earth/etc/wireguard/wg0.conf /etc/wireguard/

# Set proper permissions
sudo chmod 600 /etc/wireguard/wg0.conf

# Enable and start
sudo systemctl enable --now wg-quick@wg0.service

# Verify connection
sudo wg show
```

**Expected result:**
- Interface wg0 up with IP 192.168.2.200
- Handshakes established with all 8 peers
- Can ping other hosts (e.g., `ping 192.168.2.130` for f0)

### 8. Install WireGuard on Android Phone (pixel7pro)

**Client:** Official WireGuard Android client from Google Play Store

**Steps:**
1. Install the official WireGuard app from Google Play Store (https://play.google.com/store/apps/details?id=com.wireguard.android)
2. Transfer config file:
   - Copy `/home/paul/git/wireguardmeshgenerator/dist/pixel7pro/etc/wireguard/wg0.conf` to phone
   - OR generate QR code: `qrencode -t ansiutf8 < dist/pixel7pro/etc/wireguard/wg0.conf`
3. Import config into WireGuard app (either via file import or QR code scan)
4. Activate the tunnel

**Expected result:**
- Tunnel shows as active in WireGuard app
- Status shows connected peers
- Can access VPN network (test with ping or accessing internal services)

## Critical Files

### To Modify
- `/home/paul/git/wireguardmeshgenerator/wireguardmeshgenerator.rb` (lines ~149-163)
- `/home/paul/git/wireguardmeshgenerator/wireguardmeshgenerator.yaml` (add laptop and phone entries)

### Generated (Review)
- `/home/paul/git/wireguardmeshgenerator/dist/earth/etc/wireguard/wg0.conf`
- `/home/paul/git/wireguardmeshgenerator/dist/pixel7pro/etc/wireguard/wg0.conf`

### Keys Generated
- `/home/paul/git/wireguardmeshgenerator/keys/earth/pub.key`
- `/home/paul/git/wireguardmeshgenerator/keys/earth/priv.key`
- `/home/paul/git/wireguardmeshgenerator/keys/pixel7pro/pub.key`
- `/home/paul/git/wireguardmeshgenerator/keys/pixel7pro/priv.key`
- `/home/paul/git/wireguardmeshgenerator/keys/psk/earth_*.key` (8 preshared keys)
- `/home/paul/git/wireguardmeshgenerator/keys/psk/pixel7pro_*.key` (8 preshared keys)

## Verification

### On earth (Laptop)
```bash
# Check interface status
sudo wg show

# Verify connectivity to all hosts
for host in 130 131 132 120 121 122 110 111; do
  ping -c1 192.168.2.$host && echo "✓ 192.168.2.$host reachable"
done

# Test access to services (e.g., Prometheus)
curl http://192.168.2.130:9100/metrics  # f0 node-exporter

# Test hostname resolution
ping -c1 f0
ping -c1 blowfish
```

### On pixel7pro (Phone)
- WireGuard app shows active tunnel
- Status shows recent handshakes with all 8 peers
- Can access internal services (test with browser to 192.168.2.120:30090 for Prometheus)

### On Existing Hosts
```bash
# On any existing host (e.g., SSH to f0)
sudo wg show

# Should see two new peers:
# - earth (192.168.2.200)
# - pixel7pro (192.168.2.201)

# Test hostname resolution
ping -c1 earth
ping -c1 pixel7pro
```

## Configuration Details

### earth (Laptop) Config Structure
```
[Interface]
Address = 192.168.2.200
PrivateKey = <generated>
ListenPort = 56709

[Peer]  # f0
PublicKey = <f0 public key>
PresharedKey = <generated>
AllowedIPs = 192.168.2.130/32
Endpoint = 192.168.1.130:56709
PersistentKeepalive = 25    ← NEW: Enabled for roaming client

[Peer]  # f1
...
(continues for all 8 peers)
```

### pixel7pro (Phone) Config Structure
Identical to earth, but with:
- Interface Address: 192.168.2.201
- Different private key
- Different preshared keys

## Notes

1. **Roaming vs Fixed Clients:**
   - Roaming clients have no `lan` or `internet` section in YAML
   - They get PersistentKeepalive to ALL peers
   - They have no incoming Endpoint (behind NAT)

2. **Security:**
   - Each peer relationship uses a unique preshared key
   - Private keys are never transmitted
   - Configs contain sensitive keys - protect them

3. **Connection Behavior:**
   - earth/pixel7pro will initiate connections to all peers
   - If on same LAN as f0-f2/r0-r2, will use LAN IPs (192.168.1.x)
   - If remote, will connect to blowfish/fishfinger public IPs, and LAN hosts will be unreachable (behind NAT)

4. **Multiple Gateway Strategy:**
   - With full mesh, earth/pixel7pro can reach services through any reachable peer
   - If blowfish is down, can route through fishfinger
   - If both internet gateways are down, no access (expected for roaming clients)

5. **Git Repository:**
   - The f3s repository doesn't contain WireGuard configs (managed separately)
   - Changes are in wireguardmeshgenerator repo only
   - Consider committing updated YAML and script to version control

## Quick Reference Commands

```bash
# Generate configs
cd /home/paul/git/wireguardmeshgenerator && rake generate

# Deploy to all existing hosts
rake install

# Or deploy to specific hosts
ruby wireguardmeshgenerator.rb --install --hosts=f0,f1,f2,r0,r1,r2,blowfish,fishfinger

# Update /etc/hosts on existing hosts (run on each host)
echo "192.168.2.200   earth.wg0.wan.buetow.org earth" | sudo tee -a /etc/hosts
echo "192.168.2.201   pixel7pro.wg0.wan.buetow.org pixel7pro" | sudo tee -a /etc/hosts

# Install on earth (laptop)
sudo cp dist/earth/etc/wireguard/wg0.conf /etc/wireguard/
sudo chmod 600 /etc/wireguard/wg0.conf
sudo systemctl enable --now wg-quick@wg0.service

# Add mesh hosts to earth's /etc/hosts

# Check status
sudo wg show
```

## Failover Limitation and Solutions

### The Problem

WireGuard **does not support automatic failover** by design. When both peers (blowfish and fishfinger) are configured with `AllowedIPs = 0.0.0.0/0`, the following behavior occurs:

1. The client establishes connection to one peer (typically the first to respond)
2. The client remains "sticky" to that peer as long as packets can be sent
3. Even when the active peer goes down, the client does not immediately switch to the backup peer
4. Detection of peer failure can take several minutes due to:
   - PersistentKeepalive interval (25 seconds)
   - Network timeout detection
   - Lack of active health monitoring in WireGuard protocol

**Test results:**
- Stopped WireGuard on fishfinger (doas ifconfig wg0 down)
- Phone continued showing fishfinger's IP (Netherlands)
- Blowfish showed old handshake (17+ minutes)
- No automatic failover occurred

### Why WireGuard Doesn't Have Failover

WireGuard's design philosophy prioritizes simplicity and security over complex features. The protocol intentionally avoids implementing:
- Active peer health monitoring
- Automatic peer selection logic
- Load balancing or failover mechanisms

The official stance: failover should be handled at higher layers (routing protocols, external monitoring, load balancers).

### Possible Solutions

#### Option 1: Manual Failover (Simplest)
**Current state - accept the limitation:**
- Keep both peers configured in the client
- User manually disconnects and reconnects to trigger new peer selection
- Or switch between two saved configs (one with fishfinger primary, one with blowfish primary)

**Pros:**
- Simple, no code changes needed
- Reliable once user intervenes

**Cons:**
- Requires manual intervention
- Downtime until user notices and acts

#### Option 2: Single Primary Peer (Recommended for reliability)
**Configure only one peer as primary:**
- Edit pixel7pro config to include only fishfinger (or only blowfish)
- Keep backup config file for manual switchover if needed
- User loads backup config if primary gateway fails

**Implementation:**
```yaml
# In wireguardmeshgenerator.yaml, add to pixel7pro:
exclude_peers:
  - blowfish  # To use only fishfinger
  # OR
  - fishfinger  # To use only blowfish
```

**Pros:**
- Clear primary/backup designation
- No routing conflicts
- Faster to troubleshoot

**Cons:**
- Still requires manual intervention for failover
- Only one gateway used at a time

#### Option 3: Split AllowedIPs (Partial redundancy)
**Divide IP space between peers:**
```
[Peer]  # blowfish
AllowedIPs = 0.0.0.0/1, 128.0.0.0/2, 192.0.0.0/3, ...::/0

[Peer]  # fishfinger
AllowedIPs = 128.0.0.0/1
```

**Pros:**
- Both peers actively used
- Provides load distribution
- Partial redundancy (if one fails, half of internet still works)

**Cons:**
- Complex routing setup
- Not true failover (loses half of routes if one peer fails)
- DNS may fail if routed through dead peer

#### Option 4: External Monitoring (Complex)
**Use external script/app to monitor and switch:**
- Background app on Android monitors peer health
- Automatically reconfigures WireGuard when failure detected
- Requires custom app development

**Pros:**
- Truly automatic failover

**Cons:**
- Complex implementation
- Requires additional software
- May drain battery
- Not officially supported

### Recommended Approach

For phone (pixel7pro): **Accept manual failover** with the current dual-peer configuration.

**Reasoning:**
- Phone usage is typically interactive - user will notice connectivity issues quickly
- User can manually disconnect/reconnect WireGuard to trigger failover
- Keeps both gateways as options without complex scripts
- Simple and reliable

For automation-critical use cases (servers, IoT): Use **Option 2** with monitoring that sends alerts, allowing quick manual intervention.

### Current Configuration Status

**pixel7pro config** (/home/paul/git/wireguardmeshgenerator/dist/pixel7pro/etc/wireguard/wg0.conf):
- Two peers: blowfish (23.88.35.144) and fishfinger (46.23.94.99)
- Both with AllowedIPs = 0.0.0.0/0, ::/0
- Both with PersistentKeepalive = 25
- Both with DNS = 1.1.1.1, 8.8.8.8

**Observed behavior:**
- Client prefers fishfinger (first peer listed in some WireGuard client implementations)
- Both peers maintain handshakes, but only one actively routes traffic
- No automatic switchover when active peer fails

NEXT:

* ~~Ensure, when fishfinger goes down, wireguard traffic from phoen gets auto-rerouted via blowfish VPN~~ LIMITATION DOCUMENTED: WireGuard does not support automatic failover. Manual reconnection required.
* Ensure, that OpenBSD NAT rules are deployed via IaC (conf/frontends/...)
* Ensure, that WireGuard tunnel also works on earth, but only when started manually. It should work in the same way as the client.
* Commit all changes to the wireguardmeshegenerator git repo and push
* Update the blog post /home/paul/git/foo.zone-content/gemtext/gemfeed/2025-05-11-f3s-kubernetes-with-freebsd-part-5.gmi.tpl to include the two additional clients and how they were configured additionally. Also mention in the header like in part 7 that the post was updated, and put the timestamp accordingly. also add the updated info before the new section/s added to the blog post.
* also update the mesh network graph to include the two clients which connect to the two edge nodes blowfish and fishfinger.

