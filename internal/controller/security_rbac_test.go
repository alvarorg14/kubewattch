package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

// TestServiceWriteRBACScope guards the file-scoped Trivy exception. Only the
// manager may write Services; no wildcard or other traffic-routing resources
// may gain write access in either installation manifest.
func TestServiceWriteRBACScope(t *testing.T) {
	generated, err := os.ReadFile(filepath.Join("..", "..", "config", "rbac", "role.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	verifyClusterRoles(t, generated, "manager-role")
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Log("Helm unavailable; chart RBAC is checked in the chart CI job")
		return
	}
	chart := filepath.Join("..", "..", "charts", "kubewattch")
	cmd := exec.Command(helm, "template", "security-check", chart, "--set", "collector.serviceMonitor.enabled=true", "--set", "collector.serviceMonitor.prometheusServiceAccount.name=prometheus", "--set", "collector.serviceMonitor.prometheusServiceAccount.namespace=monitoring")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("render chart: %v: %s", err, output)
	}
	verifyClusterRoles(t, output, "security-check-kubewattch-manager")
}

func verifyClusterRoles(t *testing.T, data []byte, manager string) {
	t.Helper()
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	found := false
	foundServiceRule := false
	for {
		var raw map[string]interface{}
		if err := decoder.Decode(&raw); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if raw["kind"] != "ClusterRole" {
			continue
		}
		// Re-encode the unstructured document before decoding into the RBAC type.
		encoded, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		obj := &rbacv1.ClusterRole{}
		if err := json.Unmarshal(encoded, obj); err != nil {
			t.Fatal(err)
		}
		if obj.Name == manager {
			found = true
		}
		for _, rule := range obj.Rules {
			for _, resource := range rule.Resources {
				risky := resource == "*" || resource == "services" || resource == "endpoints" || resource == "endpointslices" || resource == "networkpolicies" || resource == "ingresses"
				if !risky {
					continue
				}
				var writes []string
				for _, verb := range rule.Verbs {
					switch verb {
					case "create", "update", "patch", "delete", "deletecollection", "impersonate", "*":
						writes = append(writes, verb)
					}
				}
				if len(writes) == 0 {
					continue
				}
				sort.Strings(writes)
				verbs := append([]string(nil), rule.Verbs...)
				sort.Strings(verbs)
				allowed := obj.Name == manager && resource == "services" && reflect.DeepEqual(rule.APIGroups, []string{""}) && reflect.DeepEqual(verbs, []string{"create", "get", "list", "update", "watch"})
				if !allowed {
					t.Errorf("unexpected traffic-control permissions in %s: group=%v resource=%s verbs=%v", obj.Name, rule.APIGroups, resource, verbs)
				} else {
					foundServiceRule = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("manager ClusterRole %q not found", manager)
	}
	if !foundServiceRule {
		t.Fatal("required manager Service permissions not found")
	}
}
