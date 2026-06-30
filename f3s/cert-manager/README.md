# cert-manager for f3s LAN TLS

This directory contains cert-manager configuration for providing self-signed TLS certificates for LAN access to f3s services via `*.f3s.lan.buetow.org`.

## Overview

- **Purpose**: Provide TLS certificates for LAN ingresses
- **Certificate Type**: Self-signed (via self-signed ClusterIssuer)
- **Wildcard Cert**: `*.f3s.lan.buetow.org`
- **TLS terminated by**: Traefik inside k3s (via ingress `tls.secretName: f3s-lan-tls`).
  FreeBSD relayd on the CARP VIP (192.168.1.138) is a **pure TCP passthrough**
  (`forward to <k3s_nodes> port 443 check tcp`, no `tls` keyword) — it does **not**
  terminate TLS and needs no certificate of its own.

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

## relayd does NOT need the certificate (historical note)

> **Obsolete:** relayd used to terminate TLS and required the wildcard keypair
> exported to `/usr/local/etc/ssl/relayd/`. The setup has since moved to **TCP
> passthrough** — relayd forwards raw TLS to Traefik, which terminates it using
> the `f3s-lan-tls` secret. There is no longer any export step, and the leftover
> `/usr/local/etc/ssl/relayd/f3s.lan.buetow.org*` files on f0/f1 are unused.

Because cert-manager renews the cert in-cluster and Traefik reloads it
automatically, **no manual action is normally required** on renewal.

Pitfall (root cause of the 2026 LAN-cert outage): if `relayd.conf` is changed
(e.g. termination → passthrough), the running relayd process keeps the **old**
behaviour and its cached keypair until restarted. Always `doas service relayd
restart` (not `reload` — SIGHUP does not re-read TLS keypairs) on **f0 and f1**
after editing relayd's TLS config, then verify the live cert at the VIP:

```bash
echo | openssl s_client -connect 192.168.1.138:443 \
  -servername f3s.lan.buetow.org 2>/dev/null | openssl x509 -noout -dates
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

The wildcard cert is valid for 90 days (`renewBefore: 360h` = 15 days).
cert-manager renews it automatically and Traefik picks up the updated
`f3s-lan-tls` secret on its own — **no manual re-export to relayd** (relayd is
now TCP passthrough; see the section above).

## See Also

- [cert-manager documentation](https://cert-manager.io/docs/)
- [Self-signed certificates](https://cert-manager.io/docs/configuration/selfsigned/)
