package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	energy "github.com/alvarorg14/kubewattch/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeliveryCheckpointsSurviveRetry(t *testing.T) {
	counts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counts[r.URL.Path]++
		if r.URL.Path == "/bottwo/sendMessage" && counts[r.URL.Path] == 1 {
			http.Error(w, "failure", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	root := t.TempDir()
	for i, token := range []string{"one", "two"} {
		dir := filepath.Join(root, fmt.Sprintf("destination-%d", i))
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "value"), []byte(token), 0600); err != nil {
			t.Fatal(err)
		}
	}
	scheduled := time.Date(2024, 3, 2, 8, 0, 0, 0, time.UTC)
	controller := true
	report := &energy.EnergyReport{ObjectMeta: metav1.ObjectMeta{Name: "report", Namespace: "ns", UID: types.UID("uid"), Generation: 1}, Spec: energy.EnergyReportSpec{Prometheus: &energy.PrometheusConfig{URL: "http://example.invalid"}, Period: "daily", Pricing: energy.ElectricityPricing{Currency: "EUR", PricePerKWh: "0.15"}, Destinations: []energy.ReportDestination{{Name: "first", Type: "telegram", Telegram: &energy.TelegramDestination{ChatID: "1"}}, {Name: "second", Type: "telegram", Telegram: &energy.TelegramDestination{ChatID: "2"}}}}}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "job", Namespace: "ns", Annotations: map[string]string{"batch.kubernetes.io/cronjob-scheduled-timestamp": scheduled.Format(time.RFC3339)}, OwnerReferences: []metav1.OwnerReference{{Kind: "CronJob", Name: "owned", Controller: &controller}}}}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "owned", Namespace: "ns"}}
	scheme := runtime.NewScheme()
	_ = energy.AddToScheme(scheme)
	_ = batchv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(report).WithObjects(report, job, cm).Build()
	worker := worker{client: c, namespace: "ns", name: "report", uid: "uid", generation: 1, resource: "owned", job: "job", credentialRoot: root, telegramBase: server.URL}
	if err := worker.run(context.Background()); err == nil {
		t.Fatal("expected failed destination")
	}
	if err := worker.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if counts["/botone/sendMessage"] != 1 || counts["/bottwo/sendMessage"] != 2 {
		t.Fatalf("unexpected delivery attempts: %v", counts)
	}
	if err := worker.run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if counts["/botone/sendMessage"] != 1 || counts["/bottwo/sendMessage"] != 2 {
		t.Fatal("completed schedule delivered again")
	}
}
