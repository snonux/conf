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
