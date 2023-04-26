#!/usr/bin/env bash

set -xv
set -euo pipefail


INGRESS_NS=${INGRESS_NS:-"ingress"}
METRICS_NS=${METRICS_NS:-"metrics"}

# pre-pull images and 'kind load' into cluster
#   so that every time you recreate the cluster, you don't have to pull
#   these images down from remote registries

images=(
  "k8s.gcr.io/ingress-nginx/controller:v1.1.1"
  "grafana/grafana:8.3.4"
  "quay.io/kiwigrid/k8s-sidecar:1.15.1"
  "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v2.0.0"
  "quay.io/prometheus/alertmanager:v0.21.0"
  "jimmidyson/configmap-reload:v0.5.0"
  "quay.io/prometheus/node-exporter:v1.1.2"
  "prom/pushgateway:v1.3.1"
  "quay.io/prometheus/prometheus:v2.26.0"
)

for image in "${images[@]}"
do
    docker pull "$image"
    kind load docker-image --name kpng-proxy "$image"
done


kubectl apply -f ./metrics-server.yaml


# ingress
kubectl create ns "$INGRESS_NS" || true
helm upgrade --install my-nginx ingress-nginx \
  --namespace "$INGRESS_NS" \
  --repo https://kubernetes.github.io/ingress-nginx \
  --version 4.0.17 \
  -f nginx-values.yaml


# metrics
kubectl create ns "$METRICS_NS" || true

helm upgrade --install my-prom prometheus \
  --repo https://prometheus-community.github.io/helm-charts \
  --version 14.0.0 \
  --namespace "$METRICS_NS"

# create datasource configuration

kubectl create secret generic grafana-datasource \
  --namespace "$METRICS_NS" \
  --from-file=./datasource.yaml \
  --dry-run \
  -o yaml \
  | kubectl apply -f -

kubectl patch secret grafana-datasource \
  --namespace "$METRICS_NS" \
  -p '{"metadata":{"labels":{"grafana_datasource": "1"}}}'

# create dashboard configuration

kubectl create secret generic grafana-dashboards \
  --namespace "$METRICS_NS" \
  --from-file=./grafana-dashboards \
  --dry-run \
  -o yaml \
  | kubectl apply -f -

kubectl patch secret grafana-dashboards \
  --namespace "$METRICS_NS" \
  -p '{"metadata":{"labels":{"grafana_dashboard": "1"}}}'

# set up grafana

helm upgrade --install my-grafana grafana \
  --repo https://grafana.github.io/helm-charts \
  --version 6.21.2 \
  --debug \
  --namespace "$METRICS_NS" \
  -f grafana-values.yaml
