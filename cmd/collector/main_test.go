package main

import (
	"context"
	"testing"

	energy "github.com/alvarorg14/kubewattch/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestWorkerStatusChecksIdentityAndGeneration(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := energy.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	d := &energy.EnergyDevice{ObjectMeta: metav1.ObjectMeta{Name: "device", Namespace: "test", UID: types.UID("device-uid"), Generation: 2}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(d).WithObjects(d).Build()
	w := &worker{client: c, namespace: "test", name: "device", uid: "old-uid", generation: 2}
	w.updateStatus(ctx, true)
	live := &energy.EnergyDevice{}
	key := types.NamespacedName{Namespace: "test", Name: "device"}
	if err := c.Get(ctx, key, live); err != nil {
		t.Fatal(err)
	}
	if live.Status.LastObservationTime != nil {
		t.Fatal("obsolete UID updated status")
	}
	w.uid = "device-uid"
	w.generation = 1
	w.updateStatus(ctx, true)
	if err := c.Get(ctx, key, live); err != nil {
		t.Fatal(err)
	}
	if live.Status.LastObservationTime != nil {
		t.Fatal("obsolete generation updated status")
	}
	w.generation = 2
	w.updateStatus(ctx, true)
	if err := c.Get(ctx, key, live); err != nil {
		t.Fatal(err)
	}
	if live.Status.LastObservationTime == nil || !meta.IsStatusConditionTrue(live.Status.Conditions, "Collecting") {
		t.Fatal("successful observation did not update status")
	}
	w.updateStatus(ctx, false)
	if err := c.Get(ctx, key, live); err != nil {
		t.Fatal(err)
	}
	if !meta.IsStatusConditionFalse(live.Status.Conditions, "Collecting") || live.Status.LastObservationTime == nil {
		t.Fatal("failure should retain last success and set condition false")
	}
}
