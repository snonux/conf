# Apache Helm Chart with Persistent Volume

This chart deploys a simple Apache web server with a persistent volume claim.

## Installing the Chart

Manual install (normally ArgoCD deploys it):

```bash
helm install example-apache-volume-claim . --namespace test --create-namespace
```