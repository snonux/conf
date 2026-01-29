# IPv6 Test

A simple IPv6/IPv4 connectivity test application deployed on k3s.

## Description

This application displays the client's IP address and determines whether they are connecting via IPv4 or IPv6.

## Deployment

The application is deployed via ArgoCD using the helm chart in `helm-chart/`.

## Access

- URL: https://ipv6test.f3s.buetow.org

## Justfile Commands

```bash
just status       # Show deployment status
just logs         # View application logs
just port-forward # Forward to localhost:8080
just sync         # Trigger ArgoCD sync
just restart      # Restart the deployment
```
