package controller

import (
	"context"
	"fmt"

	energyv1alpha1 "github.com/alvarorg14/kubewattch/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("CRD schema", func() {
	It("defaults the collection interval and rejects invalid devices", func() {
		ctx := context.Background()
		device := &energyv1alpha1.EnergyDevice{ObjectMeta: metav1.ObjectMeta{Name: "schema-device", Namespace: "default"}, Spec: energyv1alpha1.EnergyDeviceSpec{Provider: "tapo", Tapo: energyv1alpha1.TapoConfig{Address: "192.0.2.1", EmailSecretRef: energyv1alpha1.SecretKeyReference{Name: "credentials", Key: "email"}, PasswordSecretRef: energyv1alpha1.SecretKeyReference{Name: "credentials", Key: "password"}}}}
		Expect(k8sClient.Create(ctx, device)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, device)).To(Succeed()) })
		Expect(device.Spec.CollectionIntervalSeconds).To(Equal(int32(30)))
		device.Status.ObservedGeneration = device.Generation
		Expect(k8sClient.Status().Update(ctx, device)).To(Succeed())
		Expect(device.Status.ObservedGeneration).To(Equal(device.Generation))
		invalid := device.DeepCopy()
		invalid.Name = "invalid-device"
		invalid.ResourceVersion = ""
		invalid.UID = ""
		invalid.Spec.Provider = "other"
		Expect(k8sClient.Create(ctx, invalid)).NotTo(Succeed())
		invalid.Name = "invalid-secret-key"
		invalid.Spec.Provider = "tapo"
		invalid.Spec.Tapo.EmailSecretRef.Key = "invalid/key"
		Expect(k8sClient.Create(ctx, invalid)).NotTo(Succeed())
	})
	It("defaults the timezone and rejects invalid report fields", func() {
		ctx := context.Background()
		report := &energyv1alpha1.EnergyReport{ObjectMeta: metav1.ObjectMeta{Name: "schema-report", Namespace: "default"}, Spec: energyv1alpha1.EnergyReportSpec{Schedule: "0 8 * * *", Period: "daily", Pricing: energyv1alpha1.ElectricityPricing{Currency: "EUR", PricePerKWh: "0.15"}, Destinations: []energyv1alpha1.ReportDestination{{Name: "telegram", Type: "telegram", Telegram: energyv1alpha1.TelegramDestination{ChatID: "123", BotTokenSecretRef: energyv1alpha1.SecretKeyReference{Name: "telegram", Key: "token"}}}}}}
		Expect(k8sClient.Create(ctx, report)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, report)).To(Succeed()) })
		Expect(report.Spec.TimeZone).To(Equal("UTC"))
		cases := []func(*energyv1alpha1.EnergyReport){
			func(r *energyv1alpha1.EnergyReport) { r.Spec.Schedule = "bad" },
			func(r *energyv1alpha1.EnergyReport) { r.Spec.Pricing.PricePerKWh = "-1" },
			func(r *energyv1alpha1.EnergyReport) { r.Spec.Period = "yearly" },
			func(r *energyv1alpha1.EnergyReport) {
				r.Spec.Destinations = append(r.Spec.Destinations, r.Spec.Destinations[0])
			},
		}
		for i, mutate := range cases {
			invalid := report.DeepCopy()
			invalid.Name = fmt.Sprintf("invalid-report-%d", i)
			invalid.ResourceVersion = ""
			invalid.UID = ""
			mutate(invalid)
			Expect(k8sClient.Create(ctx, invalid)).NotTo(Succeed())
		}
	})
})
