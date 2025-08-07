# Apache Helm Chart with Persistent Volume

This chart deploys a simple Apache web server with a persistent volume claim.

## Installing the Chart

To install the chart with the release name `my-release`, run the following command:

```bash
helm install example-apache-volume-claim . --namespace test --create-namespace
```