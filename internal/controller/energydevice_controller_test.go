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

var _ = Describe("EnergyDevice Controller", func() {
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
		energydevice := &energyv1alpha1.EnergyDevice{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind EnergyDevice")
			err := k8sClient.Get(ctx, typeNamespacedName, energydevice)
			if err != nil && errors.IsNotFound(err) {
				resource := &energyv1alpha1.EnergyDevice{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: energyv1alpha1.EnergyDeviceSpec{Provider: "tapo", Tapo: &energyv1alpha1.TapoConfig{Address: "192.0.2.1", EmailSecretRef: energyv1alpha1.SecretKeyReference{Name: "credentials", Key: "email"}, PasswordSecretRef: energyv1alpha1.SecretKeyReference{Name: "credentials", Key: "password"}}},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &energyv1alpha1.EnergyDevice{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance EnergyDevice")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &EnergyDeviceReconciler{
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
