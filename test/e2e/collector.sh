#!/usr/bin/env bash
# Called by run.sh only, with its isolated Kind kubeconfig.
set -euo pipefail
tmpdir=$1
prom_forward_pid=
collector_forward_pid=
cleanup_collector_test() {
  if [ -n "$prom_forward_pid" ]; then kill "$prom_forward_pid" 2>/dev/null || true; fi
  if [ -n "$collector_forward_pid" ]; then kill "$collector_forward_pid" 2>/dev/null || true; fi
}
trap cleanup_collector_test EXIT
kubectl create secret generic simulated-tapo --from-literal=email=test@example.invalid --from-literal=password=test-password
kubectl apply -f - <<'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tapo-simulator
spec:
  replicas: 1
  selector:
    matchLabels: {app: tapo-simulator}
  template:
    metadata:
      labels: {app: tapo-simulator}
    spec:
      containers:
        - name: simulator
          image: kubewattch/tapo-simulator:e2e
          imagePullPolicy: IfNotPresent
          ports: [{name: http, containerPort: 8080}]
---
apiVersion: v1
kind: Service
metadata:
  name: tapo-simulator
spec:
  selector: {app: tapo-simulator}
  ports: [{name: http, port: 8080, targetPort: http}]
---
apiVersion: energy.kubewattch.alvarorg14.github.io/v1alpha1
kind: EnergyDevice
metadata:
  name: simulated-device
spec:
  provider: tapo
  collectionIntervalSeconds: 2
  tapo:
    address: tapo-simulator.default.svc:8080
    emailSecretRef: {name: simulated-tapo, key: email}
    passwordSecretRef: {name: simulated-tapo, key: password}
  metrics:
    servingCertSecretName: simulated-collector-tls
YAML
uid=$(kubectl get energydevice simulated-device -o jsonpath='{.metadata.uid}')
service="kw-${uid//-/}"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -keyout "$tmpdir/collector.key" -out "$tmpdir/collector.crt" -subj '/CN=collector' -addext "subjectAltName=DNS:${service}.default.svc" >/dev/null 2>&1
kubectl create secret tls simulated-collector-tls --cert="$tmpdir/collector.crt" --key="$tmpdir/collector.key"
kubectl rollout status deployment/tapo-simulator --timeout=120s
for _ in $(seq 1 60); do
  if kubectl get deployment "$service" >/dev/null 2>&1; then break; fi
  sleep 2
done
kubectl rollout status "deployment/$service" --timeout=180s
collector_port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
kubectl port-forward "service/$service" "$collector_port:8443" >"$tmpdir/collector-forward.log" 2>&1 &
collector_forward_pid=$!
collector_host="${service}.default.svc"
for _ in $(seq 1 30); do
  if curl -sS --noproxy '*' --cacert "$tmpdir/collector.crt" --resolve "$collector_host:$collector_port:127.0.0.1" "https://$collector_host:$collector_port/metrics" >/dev/null 2>&1; then break; fi
  sleep 1
done
test "$(curl -sS --noproxy '*' --cacert "$tmpdir/collector.crt" --resolve "$collector_host:$collector_port:127.0.0.1" -o /dev/null -w '%{http_code}' "https://$collector_host:$collector_port/metrics")" = 401
token=$(kubectl create token metrics-client)
test "$(curl -sS --noproxy '*' --cacert "$tmpdir/collector.crt" --resolve "$collector_host:$collector_port:127.0.0.1" -H "Authorization: Bearer $token" -o /dev/null -w '%{http_code}' "https://$collector_host:$collector_port/metrics")" = 200
kill "$collector_forward_pid" || true
collector_forward_pid=
cat > "$tmpdir/prometheus.yml" <<YAML
global:
  scrape_interval: 2s
scrape_configs:
  - job_name: collector
    scheme: https
    bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
    tls_config:
      ca_file: /ca/tls.crt
      server_name: ${service}.default.svc
    static_configs:
      - targets: ['${service}.default.svc:8443']
YAML
kubectl create configmap simulated-prometheus --from-file=prometheus.yml="$tmpdir/prometheus.yml"
kubectl apply -f - <<'YAML'
apiVersion: v1
kind: Pod
metadata:
  name: simulated-prometheus
spec:
  serviceAccountName: metrics-client
  containers:
    - name: prometheus
      image: prom/prometheus:v3.6.0
      args: ["--config.file=/config/prometheus.yml", "--storage.tsdb.path=/tmp/prometheus", "--web.listen-address=:9090"]
      ports: [{name: http, containerPort: 9090}]
      volumeMounts:
        - {name: config, mountPath: /config, readOnly: true}
        - {name: ca, mountPath: /ca, readOnly: true}
  volumes:
    - name: config
      configMap: {name: simulated-prometheus}
    - name: ca
      secret:
        secretName: simulated-collector-tls
        items: [{key: tls.crt, path: tls.crt}]
YAML
kubectl wait --for=condition=Ready pod/simulated-prometheus --timeout=180s
port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
kubectl port-forward pod/simulated-prometheus "$port:9090" >"$tmpdir/prometheus-forward.log" 2>&1 &
prom_forward_pid=$!
found=false
for _ in $(seq 1 60); do
  if curl -fsSG --data-urlencode 'query=kubewattch_device_power_watts{device="simulated-device"}' "http://127.0.0.1:$port/api/v1/query" 2>/dev/null | python3 -c 'import json,sys; data=json.load(sys.stdin); assert any(float(x["value"][1]) == 12.5 for x in data["data"]["result"])' 2>/dev/null; then found=true; break; fi
  sleep 2
done
if [ "$found" != true ]; then echo 'Prometheus did not store the expected collector measurement' >&2; exit 1; fi
