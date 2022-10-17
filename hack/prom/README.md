Install the prometheus monitoring


```LATEST=$(curl -s https://api.github.com/repos/prometheus-operator/prometheus-operator/releases/latest | jq -cr .tag_name) curl -sL https://github.com/prometheus-operator/prometheus-operator/releases/download/${LATEST}/bundle.yaml | kubectl create -f -```

Then install all the manifests in this folder:

- kpng-service-monitor: installs the Prometheus CRDs that tell it to scrape KPNG
- prometheus-rbac-svc: installs the infra needed for prometheus to access things and do api calls

