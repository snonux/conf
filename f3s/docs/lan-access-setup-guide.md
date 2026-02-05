# LAN Access Setup Guide

Complete guide for setting up LAN access to f3s services via `*.f3s.lan.buetow.org` using FreeBSD CARP, relayd, and cert-manager.

## Overview

This setup provides secure HTTPS access to k3s services from your local network, bypassing the OpenBSD/WireGuard external routing.

**Benefits:**
- Direct LAN access with lower latency
- TLS encryption in LAN
- Automatic failover via CARP (f0/f1)
- Same services accessible externally and locally

## Architecture

```
┌────────────────────────────────────────────────────────┐
│                    External Access                     │
│  Internet → OpenBSD relayd → WireGuard → k3s Traefik  │
│           service.f3s.buetow.org                       │
└────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────┐
│                     LAN Access                         │
│  LAN → FreeBSD CARP VIP (192.168.1.138) → k3s Traefik │
│      service.f3s.lan.buetow.org                        │
└────────────────────────────────────────────────────────┘
```

## Prerequisites

- f3s cluster with k3s running on r0, r1, r2
- FreeBSD hosts f0, f1 with CARP configured (VIP 192.168.1.138)
- kubectl access to k3s cluster
- Git repository synced to git-server in k3s

## Setup Steps

### Step 1: Deploy cert-manager

cert-manager manages TLS certificates for LAN services.

#### Commit and Push Changes

```bash
cd /home/paul/git/conf
git add f3s/cert-manager
git add f3s/argocd-apps/infra/cert-manager.yaml
git commit -m "Add cert-manager for LAN TLS certificates"
git push r0 master
git push r1 master
git push r2 master
```

#### Wait for ArgoCD Sync

ArgoCD will automatically deploy cert-manager. Monitor progress:

```bash
# Watch ArgoCD application status
kubectl get application cert-manager -n cicd -w

# Check cert-manager pods
kubectl get pods -n cert-manager
```

Expected output:
```
NAME                                       READY   STATUS
cert-manager-XXXXX                         1/1     Running
cert-manager-cainjector-XXXXX              1/1     Running
cert-manager-webhook-XXXXX                 1/1     Running
```

#### Verify Certificates

```bash
# Check certificates
kubectl get certificate -n cert-manager

# Should show:
# selfsigned-ca        True    CA certificate ready
# f3s-lan-wildcard     True    Certificate is up to date
```

### Step 2: Export TLS Certificates

Export certificates from k3s for use by relayd:

```bash
cd /home/paul/git/conf/f3s/cert-manager
just export-certs
```

This creates:
- `/tmp/f3s-lan-cert.pem`
- `/tmp/f3s-lan-key.pem`

### Step 3: Install relayd on FreeBSD

#### Install Package

On f0 and f1:

```bash
ssh paul@192.168.1.130 'doas pkg install -y relayd'
ssh paul@192.168.1.131 'doas pkg install -y relayd'
```

#### Create Configuration

On f0:

```bash
ssh paul@192.168.1.130 'doas tee /usr/local/etc/relayd.conf' << 'EOF'
# k3s nodes backend table
table <k3s_nodes> { 192.168.1.120 192.168.1.121 192.168.1.122 }

# HTTP protocol (pass-through to Traefik)
http protocol "lan_http" {
    pass request quick
    pass response quick
}

# HTTPS protocol with TLS termination
http protocol "lan_https" {
    tls keypair "f3s.lan.buetow.org"
    pass request quick
    pass response quick
}

# HTTP relay (port 80)
relay "lan_http" {
    listen on 192.168.1.138 port 80
    protocol "lan_http"
    forward to <k3s_nodes> port 80 check tcp
}

# HTTPS relay (port 443) with TLS
relay "lan_https" {
    listen on 192.168.1.138 port 443 tls
    protocol "lan_https"
    forward to <k3s_nodes> port 80 check tcp
}
EOF
```

Repeat for f1 (same config):

```bash
ssh paul@192.168.1.131 'doas tee /usr/local/etc/relayd.conf' << 'EOF'
[... same config as above ...]
EOF
```

#### Copy Certificates to FreeBSD

```bash
# Copy to f0
scp /tmp/f3s-lan-cert.pem paul@192.168.1.130:/tmp/
scp /tmp/f3s-lan-key.pem paul@192.168.1.130:/tmp/

# Copy to f1
scp /tmp/f3s-lan-cert.pem paul@192.168.1.131:/tmp/
scp /tmp/f3s-lan-key.pem paul@192.168.1.131:/tmp/
```

#### Install Certificates

On both f0 and f1:

```bash
for host in 192.168.1.130 192.168.1.131; do
  ssh paul@$host << 'EOF'
    doas mkdir -p /usr/local/etc/ssl/relayd
    doas mv /tmp/f3s-lan-cert.pem /usr/local/etc/ssl/relayd/f3s.lan.buetow.org.crt
    doas mv /tmp/f3s-lan-key.pem /usr/local/etc/ssl/relayd/f3s.lan.buetow.org.key
    doas sh -c 'cat /usr/local/etc/ssl/relayd/f3s.lan.buetow.org.crt /usr/local/etc/ssl/relayd/f3s.lan.buetow.org.key > /usr/local/etc/ssl/relayd/f3s.lan.buetow.org.pem'
    doas chmod 600 /usr/local/etc/ssl/relayd/*
    doas chown root:wheel /usr/local/etc/ssl/relayd/*
EOF
done
```

#### Enable and Start relayd

```bash
ssh paul@192.168.1.130 'doas sysrc relayd_enable=YES && doas service relayd start'
ssh paul@192.168.1.131 'doas sysrc relayd_enable=YES && doas service relayd start'
```

#### Verify relayd

```bash
ssh paul@192.168.1.130 'doas sockstat -4 -l | grep 192.168.1.138'
```

Expected output:
```
stunnel  stunnel     1546 8   tcp4   192.168.1.138:2323    *:*
relayd   relayd      2101 3   tcp4   192.168.1.138:80      *:*
relayd   relayd      2101 4   tcp4   192.168.1.138:443     *:*
```

### Step 4: Deploy Navidrome with LAN Ingress

#### Commit and Push Changes

```bash
cd /home/paul/git/conf
git add f3s/navidrome
git commit -m "Add LAN ingress for Navidrome"
git push r0 master
git push r1 master
git push r2 master
```

#### Wait for ArgoCD Sync

```bash
# Watch for sync
kubectl get application navidrome -n cicd -w

# Check ingress
kubectl get ingress -n services | grep navidrome
```

Expected output:
```
navidrome-ingress        navidrome.f3s.buetow.org
navidrome-ingress-lan    navidrome.f3s.lan.buetow.org
```

### Step 5: Configure DNS

Add DNS records for LAN domains. Choose one method:

#### Method A: Local DNS Server

If you have a local DNS server (e.g., Pi-hole, dnsmasq), add:

```
192.168.1.138  navidrome.f3s.lan.buetow.org
192.168.1.138  *.f3s.lan.buetow.org  # Wildcard for future services
```

#### Method B: /etc/hosts (per device)

On each client device, edit `/etc/hosts`:

```bash
# Linux/macOS
sudo bash -c 'echo "192.168.1.138  navidrome.f3s.lan.buetow.org" >> /etc/hosts'

# Windows (as Administrator)
# Edit C:\Windows\System32\drivers\etc\hosts
```

### Step 6: Trust Self-Signed CA Certificate

To avoid browser warnings, install the CA certificate on client devices.

#### Export CA Certificate

```bash
cd /home/paul/git/conf/f3s/cert-manager
just export-ca
# Creates /tmp/f3s-lan-ca.crt
```

#### Install on Linux (Fedora)

```bash
sudo cp /tmp/f3s-lan-ca.crt /etc/pki/ca-trust/source/anchors/
sudo update-ca-trust
```

#### Install on Linux (Debian/Ubuntu)

```bash
sudo cp /tmp/f3s-lan-ca.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates
```

#### Install on macOS

```bash
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain /tmp/f3s-lan-ca.crt
```

#### Install on Windows

1. Double-click `f3s-lan-ca.crt`
2. Click "Install Certificate"
3. Select "Local Machine"
4. Choose "Place all certificates in the following store"
5. Select "Trusted Root Certification Authorities"
6. Finish

#### Install on Android

1. Copy `f3s-lan-ca.crt` to device
2. Settings → Security → Encryption & credentials
3. Install a certificate → CA certificate
4. Select the file

#### Install on iOS

1. AirDrop or email `f3s-lan-ca.crt` to device
2. Open the file
3. Settings → General → VPN & Device Management
4. Install the profile
5. Settings → General → About → Certificate Trust Settings
6. Enable full trust for the certificate

### Step 7: Test Access

#### Test HTTP Access

```bash
curl -v http://navidrome.f3s.lan.buetow.org
```

#### Test HTTPS Access

```bash
# Without CA trust (expect certificate warning)
curl -k https://navidrome.f3s.lan.buetow.org

# With CA trust installed
curl https://navidrome.f3s.lan.buetow.org
```

#### Test in Browser

Open in browser: `https://navidrome.f3s.lan.buetow.org`

You should see the Navidrome login page with no certificate warnings (if CA is trusted).

### Step 8: Test CARP Failover

Verify failover works:

```bash
# Disable f0 interface
ssh paul@192.168.1.130 'doas ifconfig re0 down'

# Wait 2-3 seconds, then test access
curl https://navidrome.f3s.lan.buetow.org
# Should still work (f1 becomes MASTER)

# Re-enable f0
ssh paul@192.168.1.130 'doas ifconfig re0 up'
```

## Adding More Services

To add LAN access to other services:

1. **Add LAN ingress** to service's helm chart:
   ```yaml
   ---
   apiVersion: networking.k8s.io/v1
   kind: Ingress
   metadata:
     name: service-ingress-lan
     namespace: services
     annotations:
       spec.ingressClassName: traefik
       traefik.ingress.kubernetes.io/router.entrypoints: web
   spec:
     rules:
       - host: service.f3s.lan.buetow.org
         http:
           paths:
             - path: /
               pathType: Prefix
               backend:
                 service:
                   name: service-name
                   port:
                     number: 1234
   ```

2. **Add DNS entry**: `192.168.1.138  service.f3s.lan.buetow.org`

3. **Commit and push** changes

4. No relayd or cert-manager changes needed!

## Troubleshooting

### Certificate Warnings in Browser

- Ensure CA certificate is installed and trusted
- Restart browser after installing CA
- Check certificate validity: `openssl s_client -connect navidrome.f3s.lan.buetow.org:443`

### Connection Refused

- Check DNS resolution: `nslookup navidrome.f3s.lan.buetow.org`
- Verify relayd is running: `ssh paul@192.168.1.130 'doas service relayd status'`
- Check CARP status: `ssh paul@192.168.1.130 'ifconfig re0 | grep carp'`

### 502 Bad Gateway

- Verify k3s nodes are reachable from f0/f1
- Check Traefik is running: `kubectl get pods -n kube-system -l app.kubernetes.io/name=traefik`
- Test backend directly: `curl -H "Host: navidrome.f3s.lan.buetow.org" http://192.168.1.120`

### Service Not Found (404)

- Verify ingress exists: `kubectl get ingress -n services | grep navidrome`
- Check ingress details: `kubectl describe ingress navidrome-ingress-lan -n services`
- Verify service is running: `kubectl get pods -n services | grep navidrome`

## Certificate Renewal

Certificates renew automatically every 75 days. After renewal:

```bash
cd /home/paul/git/conf/f3s/cert-manager
just export-certs

# Copy to FreeBSD
scp /tmp/f3s-lan-*.pem paul@192.168.1.130:/tmp/
scp /tmp/f3s-lan-*.pem paul@192.168.1.131:/tmp/

# Reinstall and reload relayd on both hosts
for host in 192.168.1.130 192.168.1.131; do
  ssh paul@$host << 'EOF'
    doas mv /tmp/f3s-lan-cert.pem /usr/local/etc/ssl/relayd/f3s.lan.buetow.org.crt
    doas mv /tmp/f3s-lan-key.pem /usr/local/etc/ssl/relayd/f3s.lan.buetow.org.key
    doas sh -c 'cat /usr/local/etc/ssl/relayd/f3s.lan.buetow.org.crt /usr/local/etc/ssl/relayd/f3s.lan.buetow.org.key > /usr/local/etc/ssl/relayd/f3s.lan.buetow.org.pem'
    doas chmod 600 /usr/local/etc/ssl/relayd/*
    doas service relayd reload
EOF
done
```

## Summary

You now have:

- ✅ cert-manager providing self-signed TLS certificates
- ✅ FreeBSD relayd forwarding LAN traffic to k3s
- ✅ CARP failover between f0 and f1
- ✅ Navidrome accessible via `https://navidrome.f3s.lan.buetow.org`
- ✅ Pattern for adding more services

External access via `*.f3s.buetow.org` continues to work unchanged.
