#!/bin/bash
# Patches the Grafana datasource ConfigMap to add Loki and Tempo with correct field order

cat > /tmp/datasource-complete.yaml << 'YAML'
apiVersion: 1
datasources:
- name: "Prometheus"
  type: prometheus
  uid: prometheus
  url: http://prometheus-kube-prometheus-prometheus.monitoring:9090/
  access: proxy
  isDefault: true
  jsonData:
    httpMethod: POST
    timeInterval: 30s
- name: "Alertmanager"
  type: alertmanager
  uid: alertmanager
  url: http://prometheus-kube-prometheus-alertmanager.monitoring:9093/
  access: proxy
  jsonData:
    handleGrafanaManagedAlerts: false
    implementation: prometheus
- name: Loki
  type: loki
  uid: loki
  url: http://loki.monitoring.svc.cluster.local:3100
  access: proxy
  isDefault: false
  editable: false
- name: Tempo
  type: tempo
  uid: tempo
  url: http://tempo.monitoring.svc.cluster.local:3200
  access: proxy
  isDefault: false
  editable: false
  jsonData:
    httpMethod: GET
    tracesToLogsV2:
      datasourceUid: loki
      spanStartTimeShift: -1h
      spanEndTimeShift: 1h
    tracesToMetrics:
      datasourceUid: prometheus
    serviceMap:
      datasourceUid: prometheus
    nodeGraph:
      enabled: true
    search:
      hide: false
    lokiSearch:
      datasourceUid: loki
YAML

kubectl create configmap prometheus-kube-prometheus-grafana-datasource \
  --from-file=datasource.yaml=/tmp/datasource-complete.yaml \
  -n monitoring \
  --dry-run=client -o yaml | kubectl apply -f -

rm /tmp/datasource-complete.yaml
