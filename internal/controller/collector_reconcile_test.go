package controller

import (
	"context"
	"testing"

	energy "github.com/alvarorg14/kubewattch/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCollectorReconciliationAndDeletion(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := energy.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	d := &energy.EnergyDevice{ObjectMeta: metav1.ObjectMeta{Name: "plug", Namespace: "test", UID: types.UID("12345678-1234-1234-1234-123456789012"), Generation: 1}, Spec: energy.EnergyDeviceSpec{Provider: "tapo", Tapo: &energy.TapoConfig{Address: "192.0.2.1", EmailSecretRef: energy.SecretKeyReference{Name: "credentials", Key: "email"}, PasswordSecretRef: energy.SecretKeyReference{Name: "credentials", Key: "password"}}, Metrics: &energy.DeviceMetricsConfig{ServingCertSecretName: "tls"}, CollectionIntervalSeconds: 30}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(d).WithObjects(d).Build()
	r := &EnergyDeviceReconciler{Client: c, Scheme: scheme, CollectorImage: "example.invalid/collector:test"}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(d)}
	for i := 0; i < 2; i++ {
		if _, err := r.Reconcile(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	name := collectorName(d)
	dep := &appsv1.Deployment{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: d.Namespace, Name: name}, dep); err != nil {
		t.Fatal(err)
	}
	if dep.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType || dep.Spec.Template.Spec.ServiceAccountName != name {
		t.Fatal("unexpected collector deployment")
	}
	if dep.Spec.Template.Spec.Containers[0].Image != "example.invalid/collector:test" {
		t.Fatal("incorrect collector image")
	}
	binding := &rbacv1.ClusterRoleBinding{}
	if err := c.Get(ctx, types.NamespacedName{Name: name}, binding); err != nil {
		t.Fatal(err)
	}
	if len(binding.Subjects) != 1 || binding.Subjects[0].Namespace != d.Namespace {
		t.Fatal("incorrect authentication binding")
	}
	live := &energy.EnergyDevice{}
	if err := c.Get(ctx, req.NamespacedName, live); err != nil {
		t.Fatal(err)
	}
	if err := c.Delete(ctx, live); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, types.NamespacedName{Name: name}, binding); err == nil {
		t.Fatal("authentication binding was not cleaned up")
	}
}
