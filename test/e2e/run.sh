#!/usr/bin/env bash
# Runs only against a new disposable Kind cluster, never the current kubeconfig.
set -euo pipefail
name="kubewattch-test-$$"
tmpdir=$(mktemp -d)
export KUBECONFIG="$tmpdir/kubeconfig"
created=false
cleanup() {
  if [ -n "${forward_pid:-}" ]; then kill "$forward_pid" 2>/dev/null || true; fi
  if [ "$created" = true ]; then kind delete cluster --name "$name" || true; fi
  rm -rf "$tmpdir"
}
trap cleanup EXIT
if kind get clusters | grep -Fxq "$name"; then
  echo "Cluster already exists: $name" >&2
  exit 1
fi
created=true
kind create cluster --name "$name" --kubeconfig "$KUBECONFIG"
docker build -f build/operator/Dockerfile -t kubewattch/operator:e2e .
kind load docker-image kubewattch/operator:e2e --name "$name"
helm install kw charts/kubewattch --set image.repository=kubewattch/operator --set image.tag=e2e --wait --timeout 180s
kubectl apply -f examples/tapo-device.yaml -f examples/telegram-daily-report.yaml
kubectl get energydevices,energyreports
kubectl create serviceaccount metrics-client
kubectl create clusterrole kubewattch-e2e-metrics --verb=get --non-resource-url=/metrics
kubectl create clusterrolebinding kubewattch-e2e-metrics --clusterrole=kubewattch-e2e-metrics --serviceaccount=default:metrics-client
token=$(kubectl create token metrics-client)
port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
kubectl port-forward service/kw-operator-metrics "$port:8443" >"$tmpdir/port-forward.log" 2>&1 &
forward_pid=$!
for i in $(seq 1 30); do kill -0 "$forward_pid"; curl -sk https://127.0.0.1:$port/metrics >/dev/null && break || sleep 1; done
test "$(curl -sk -o /dev/null -w '%{http_code}' https://127.0.0.1:$port/metrics)" = 401
test "$(curl -sk -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $token" https://127.0.0.1:$port/metrics)" = 200
kill "$forward_pid" || true
forward_pid=
