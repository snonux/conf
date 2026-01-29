# Traefik Configuration

k3s HelmChartConfig to customize the bundled Traefik ingress controller.

## What This Does

Configures Traefik to trust `X-Forwarded-For` headers from trusted proxy networks (relayd on frontends).

This allows backend applications to see the real client IP address instead of internal cluster IPs.

## Apply

```bash
kubectl apply -f helmchartconfig.yaml
```

Traefik will automatically restart and pick up the new configuration.

## Trusted Networks

- `192.168.0.0/16` - WireGuard tunnel IPs (relayd frontends)
- `10.0.0.0/8` - Kubernetes pod/service network
- `172.16.0.0/12` - Docker bridge networks
