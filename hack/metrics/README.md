# Metrics

## What is deployed

Prometheus master

Grafana

Nginx ingress controller

## How to use

Create an /etc/hosts entry on your dev machine:

```
127.0.0.1 chart-example.local
```

Create a local cluster with metrics enabled:

```
cd kpng/hack

DEPLOY_PROMETHEUS=true ./kpng-local-up.sh
```

Open Grafana in your web browser:

 - http://chart-example.local/login 
   - creds: admin / admin
 - take a look at a dashboard: http://chart-example.local/d/abcdefgh/nginx-ingress-controller?orgId=1
