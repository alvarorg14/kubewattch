/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	energyv1alpha1 "github.com/alvarorg14/kubewattch/api/v1alpha1"
)

var _ = Describe("EnergyReport Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		energyreport := &energyv1alpha1.EnergyReport{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind EnergyReport")
			err := k8sClient.Get(ctx, typeNamespacedName, energyreport)
			if err != nil && errors.IsNotFound(err) {
				resource := &energyv1alpha1.EnergyReport{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: energyv1alpha1.EnergyReportSpec{Schedule: "0 8 * * *", Period: "daily", Pricing: energyv1alpha1.ElectricityPricing{Currency: "EUR", PricePerKWh: "0.15"}, Destinations: []energyv1alpha1.ReportDestination{{Name: "telegram", Type: "telegram", Telegram: &energyv1alpha1.TelegramDestination{ChatID: "123", BotTokenSecretRef: energyv1alpha1.SecretKeyReference{Name: "credentials", Key: "token"}}}}},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &energyv1alpha1.EnergyReport{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance EnergyReport")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &EnergyReportReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("EnergyReport provisioning", func() {
	It("creates isolated owned resources, preserves snapshots, and suspends invalid schedules", func() {
		ctx := context.Background()
		report := &energyv1alpha1.EnergyReport{ObjectMeta: metav1.ObjectMeta{Name: "provision-test", Namespace: "default"}, Spec: energyv1alpha1.EnergyReportSpec{Schedule: "0 8 * * *", TimeZone: "UTC", Period: "daily", Prometheus: &energyv1alpha1.PrometheusConfig{URL: "http://prometheus.example"}, Pricing: energyv1alpha1.ElectricityPricing{Currency: "EUR", PricePerKWh: "0.15"}, Destinations: []energyv1alpha1.ReportDestination{{Name: "telegram", Type: "telegram", Telegram: &energyv1alpha1.TelegramDestination{ChatID: "123", BotTokenSecretRef: energyv1alpha1.SecretKeyReference{Name: "telegram", Key: "token"}}}}}}
		Expect(k8sClient.Create(ctx, report)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, report)).To(Succeed()) })
		rec := &EnergyReportReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), ReporterImage: "example.invalid/reporter:test"}
		request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: report.Namespace, Name: report.Name}}
		_, err := rec.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		name := reportName(report)
		cj := &batchv1.CronJob{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: report.Namespace, Name: name}, cj)).To(Succeed())
		Expect(metav1.IsControlledBy(cj, report)).To(BeTrue())
		role := &rbacv1.Role{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: report.Namespace, Name: name}, role)).To(Succeed())
		for _, rule := range role.Rules {
			Expect(rule.Resources).NotTo(ContainElement("secrets"))
			Expect(rule.Resources).NotTo(ContainElement("*"))
			Expect(rule.Verbs).NotTo(ContainElement("*"))
		}
		Expect(cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String()).To(Equal("256Mi"))

		Expect(cj.Spec.ConcurrencyPolicy).To(Equal(batchv1.ForbidConcurrent))
		Expect(cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image).To(Equal("example.invalid/reporter:test"))
		job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "provision-test-job", Namespace: report.Namespace, Labels: cj.Spec.JobTemplate.Labels, Annotations: map[string]string{"batch.kubernetes.io/cronjob-scheduled-timestamp": "2026-11-05T08:00:00Z"}, OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "CronJob", Name: cj.Name, UID: cj.UID, Controller: ptr(true)}}}, Spec: cj.Spec.JobTemplate.Spec}
		Expect(k8sClient.Create(ctx, job)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, job) })
		now := metav1.Now()
		job.Status.StartTime = &now
		job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue}, {Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
		Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
		_, err = rec.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		observed := &energyv1alpha1.EnergyReport{}
		Expect(k8sClient.Get(ctx, request.NamespacedName, observed)).To(Succeed())
		Expect(observed.Status.LastRun.State).To(Equal("Failed"))
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: report.Namespace, Name: name}, cj)).To(Succeed())
		cronVersion := cj.ResourceVersion
		_, err = rec.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: report.Namespace, Name: name}, cj)).To(Succeed())
		Expect(cj.ResourceVersion).To(Equal(cronVersion), "unchanged reconciliation should not rewrite the CronJob")
		snapshot := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: report.Namespace, Name: name}, snapshot)).To(Succeed())
		snapshot.Data = map[string]string{"snapshot.json": "saved"}
		Expect(k8sClient.Update(ctx, snapshot)).To(Succeed())
		_, err = rec.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: report.Namespace, Name: name}, snapshot)).To(Succeed())
		Expect(snapshot.Data["snapshot.json"]).To(Equal("saved"))
		current := &energyv1alpha1.EnergyReport{}
		Expect(k8sClient.Get(ctx, request.NamespacedName, current)).To(Succeed())
		current.Spec.Schedule = "99 8 * * *"
		Expect(k8sClient.Update(ctx, current)).To(Succeed())
		_, err = rec.Reconcile(ctx, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: report.Namespace, Name: name}, cj)).To(Succeed())
		Expect(*cj.Spec.Suspend).To(BeTrue())
	})
})
