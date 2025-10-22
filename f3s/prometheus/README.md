# Prometheus installation

## Prometheus stack installation

First, install the Prometheus Helm chart using the following commands:

```sh
kubectl greate ns monitoring
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
```

Followed by the actual installation into the monitoring namespace:
  
```sh
helm install prometheus prometheus-community/kube-prometheus-stack --namespace monitoring

NAME: prometheus
LAST DEPLOYED: Wed Oct 22 09:22:00 2025
NAMESPACE: monitoring
STATUS: deployed
REVISION: 1
NOTES:
kube-prometheus-stack has been installed. Check its status by running:
  kubectl --namespace monitoring get pods -l "release=prometheus"

Get Grafana 'admin' user password by running:

  kubectl --namespace monitoring get secrets prometheus-grafana -o jsonpath="{.data.admin-password}" | base64 -d ; echo

Access Grafana local instance:

  export POD_NAME=$(kubectl --namespace monitoring get pod -l "app.kubernetes.io/name=grafana,app.kubernetes.io/instance=prometheus" -oname)
  kubectl --namespace monitoring port-forward $POD_NAME 3000

Visit https://github.com/prometheus-operator/kube-prometheus for instructions on how to create & configure Alertmanager and Prometheus instances using the Operator.

```

## Grafana Ingress

After this, deploy Grafana Ingress:

```
helm install grafana-ingress ./grafana-ingress
```
