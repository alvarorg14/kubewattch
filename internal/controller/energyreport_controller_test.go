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
					Spec: energyv1alpha1.EnergyReportSpec{Schedule: "0 8 * * *", Period: "daily", Pricing: energyv1alpha1.ElectricityPricing{Currency: "EUR", PricePerKWh: "0.15"}, Destinations: []energyv1alpha1.ReportDestination{{Name: "telegram", Type: "telegram", Telegram: energyv1alpha1.TelegramDestination{ChatID: "123", BotTokenSecretRef: energyv1alpha1.SecretKeyReference{Name: "credentials", Key: "token"}}}}},
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
