#!/usr/bin/env bash
# Called by run.sh only, with its isolated Kind kubeconfig.
set -euo pipefail
tmpdir=$1
report_forward_pid=
cleanup_report_test() { if [ -n "$report_forward_pid" ]; then kill "$report_forward_pid" 2>/dev/null || true; fi; }
trap cleanup_report_test EXIT
kubectl create secret generic simulated-telegram --from-literal=token=synthetic-token
kubectl apply -f - <<'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: report-simulator
spec:
  replicas: 1
  selector:
    matchLabels: {app: report-simulator}
  template:
    metadata:
      labels: {app: report-simulator}
    spec:
      containers:
        - name: simulator
          image: kubewattch/report-simulator:e2e
          imagePullPolicy: IfNotPresent
          ports: [{name: http, containerPort: 8080}]
---
apiVersion: v1
kind: Service
metadata:
  name: report-simulator
spec:
  selector: {app: report-simulator}
  ports: [{name: http, port: 8080, targetPort: http}]
---
apiVersion: energy.kubewattch.alvarorg14.github.io/v1alpha1
kind: EnergyReport
metadata:
  name: simulated-report
spec:
  schedule: '*/2 * * * *'
  timeZone: UTC
  prometheus: {url: 'http://report-simulator.default.svc:8080'}
  deviceSelector:
    matchLabels: {report-test: no-devices}
  period: daily
  pricing: {currency: EUR, pricePerKWh: '0.15'}
  destinations:
    - name: telegram
      type: telegram
      telegram:
        chatID: '123'
        botTokenSecretRef: {name: simulated-telegram, key: token}
YAML
kubectl rollout status deployment/report-simulator --timeout=120s
for _ in $(seq 1 120); do
  if [ "$(kubectl get energyreport simulated-report -o jsonpath='{.status.lastRun.state}')" = Succeeded ]; then break; fi
  sleep 2
done
test "$(kubectl get energyreport simulated-report -o jsonpath='{.status.lastRun.state}')" = Succeeded
uid=$(kubectl get energyreport simulated-report -o jsonpath='{.metadata.uid}')
cronjob="kwr-${uid//-/}"
# Replay the same scheduled timestamp as a second Job owned by the CronJob.
scheduled=$(kubectl get energyreport simulated-report -o jsonpath='{.status.lastRun.scheduledTime}')
cron_uid=$(kubectl get cronjob "$cronjob" -o jsonpath='{.metadata.uid}')
kubectl create job report-replay --from="cronjob/$cronjob" --dry-run=client -o json >"$tmpdir/report-replay.json"
python3 - "$tmpdir/report-replay.json" "$scheduled" "$cronjob" "$cron_uid" <<'PYCODE'
import json, sys
path, scheduled, name, uid = sys.argv[1:]
with open(path, encoding='utf-8') as f:
    job = json.load(f)
metadata = job['metadata']
metadata.setdefault('annotations', {})['batch.kubernetes.io/cronjob-scheduled-timestamp'] = scheduled
metadata['ownerReferences'] = [{'apiVersion': 'batch/v1', 'kind': 'CronJob', 'name': name, 'uid': uid, 'controller': True}]
with open(path, 'w', encoding='utf-8') as f:
    json.dump(job, f)
PYCODE
kubectl create -f "$tmpdir/report-replay.json"
kubectl wait --for=condition=complete job/report-replay --timeout=90s
kubectl patch energyreport simulated-report --type=merge -p '{"spec":{"suspend":true}}'
port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
kubectl port-forward service/report-simulator "$port:8080" >"$tmpdir/report-forward.log" 2>&1 &
report_forward_pid=$!
for _ in $(seq 1 30); do if curl -sf "http://127.0.0.1:$port/stats" >"$tmpdir/report-stats.json"; then break; fi; sleep 1; done
python3 - "$tmpdir/report-stats.json" <<'PY'
import json, sys
assert json.load(open(sys.argv[1], encoding='utf-8'))['received'] == 1
PY
test "$(kubectl get energyreport simulated-report -o jsonpath='{.status.lastRun.destinations[0].confirmedChunks}')" = 1
