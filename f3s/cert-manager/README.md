# cert-manager for f3s LAN TLS

This directory contains cert-manager configuration for providing self-signed TLS certificates for LAN access to f3s services via `*.f3s.lan.buetow.org`.

## Overview

- **Purpose**: Provide TLS certificates for LAN ingresses
- **Certificate Type**: Self-signed (via self-signed ClusterIssuer)
- **Wildcard Cert**: `*.f3s.lan.buetow.org`
- **Used by**: FreeBSD relayd on CARP VIP (192.168.1.138)

## Components

1. **cert-manager.yaml** - Official cert-manager installation (v1.14.4)
2. **self-signed-issuer.yaml** - ClusterIssuer for self-signed certificates
3. **ca-certificate.yaml** - CA certificate for signing
4. **wildcard-certificate.yaml** - Wildcard certificate for `*.f3s.lan.buetow.org`

## Deployment

Deployed via ArgoCD from `argocd-apps/infra/cert-manager.yaml`.

Manual deployment:
```bash
just install
```

## Exporting Certificates for relayd

After cert-manager creates the wildcard certificate, export it for use by FreeBSD relayd:

```bash
# Export from k3s
kubectl get secret f3s-lan-tls -n cert-manager -o jsonpath='{.data.tls\.crt}' | base64 -d > /tmp/f3s-lan-cert.pem
kubectl get secret f3s-lan-tls -n cert-manager -o jsonpath='{.data.tls\.key}' | base64 -d > /tmp/f3s-lan-key.pem

# Copy to FreeBSD hosts
scp /tmp/f3s-lan-cert.pem paul@f0:/tmp/
scp /tmp/f3s-lan-key.pem paul@f0:/tmp/
scp /tmp/f3s-lan-cert.pem paul@f1:/tmp/
scp /tmp/f3s-lan-key.pem paul@f1:/tmp/

# On f0 and f1
doas mkdir -p /usr/local/etc/ssl/relayd
doas mv /tmp/f3s-lan-cert.pem /usr/local/etc/ssl/relayd/f3s.lan.buetow.org.crt
doas mv /tmp/f3s-lan-key.pem /usr/local/etc/ssl/relayd/f3s.lan.buetow.org.key
doas chmod 600 /usr/local/etc/ssl/relayd/*
doas chown root:wheel /usr/local/etc/ssl/relayd/*
doas service relayd reload
```

## Trusting the CA Certificate

To avoid browser warnings, clients must trust the self-signed CA:

### Export CA Certificate

```bash
kubectl get secret selfsigned-ca-secret -n cert-manager -o jsonpath='{.data.ca\.crt}' | base64 -d > f3s-lan-ca.crt
```

### Install on Clients

**Linux (Fedora/Debian/Ubuntu):**
```bash
sudo cp f3s-lan-ca.crt /usr/local/share/ca-certificates/f3s-lan-ca.crt
sudo update-ca-certificates
```

**macOS:**
```bash
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain f3s-lan-ca.crt
```

**Windows:**
- Double-click `f3s-lan-ca.crt`
- Install to "Trusted Root Certification Authorities"

**Android:**
- Settings → Security → Encryption & credentials → Install a certificate → CA certificate

**iOS:**
- AirDrop the certificate or email it
- Settings → General → VPN & Device Management → Install Profile

## Certificate Renewal

Self-signed certificates are valid for 90 days by default. cert-manager automatically renews them before expiration. After renewal, re-export and deploy to relayd.

## See Also

- [cert-manager documentation](https://cert-manager.io/docs/)
- [Self-signed certificates](https://cert-manager.io/docs/configuration/selfsigned/)
