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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ElectricityPricing specifies a fixed price per kWh.
type ElectricityPricing struct {
	// +kubebuilder:validation:Pattern=`^[A-Z]{3}$`
	// Currency is a three-letter uppercase currency code.
	Currency string `json:"currency"`
	// +kubebuilder:validation:Pattern=`^[0-9]+(\.[0-9]+)?$`
	// PricePerKWh is the nonnegative decimal price per kilowatt-hour.
	PricePerKWh string `json:"pricePerKWh"`
}

// TelegramDestination identifies a chat and its bot token.
type TelegramDestination struct {
	// +kubebuilder:validation:MinLength=1
	// ChatID identifies the Telegram chat as a string.
	ChatID string `json:"chatID"`
	// BotTokenSecretRef points to the bot token in a same-namespace Secret.
	BotTokenSecretRef SecretKeyReference `json:"botTokenSecretRef"`
}

// ReportDestination describes one named reporting destination.
type ReportDestination struct {
	// +kubebuilder:validation:MinLength=1
	// Name uniquely identifies a destination within the report.
	Name string `json:"name"`
	// +kubebuilder:validation:Enum=telegram
	// Type identifies the destination integration.
	Type string `json:"type"`
	// Telegram contains the Telegram delivery configuration.
	Telegram TelegramDestination `json:"telegram"`
}

// EnergyReportSpec defines a scheduled energy report.
type EnergyReportSpec struct {
	// +kubebuilder:validation:Pattern=`^[^ ]+ [^ ]+ [^ ]+ [^ ]+ [^ ]+$`
	// Schedule is a five-field cron expression.
	Schedule string `json:"schedule"`
	// +kubebuilder:default=UTC
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9_+/-]+$`
	// +optional
	// TimeZone is the intended IANA timezone for the schedule.
	TimeZone string `json:"timeZone,omitempty"`
	// DeviceSelector selects devices in the report namespace; an empty selector matches all.
	DeviceSelector metav1.LabelSelector `json:"deviceSelector"`
	// +kubebuilder:validation:Enum=daily;weekly;monthly
	// Period selects daily, weekly, or monthly reporting.
	Period string `json:"period"`
	// Pricing defines the fixed energy tariff.
	Pricing ElectricityPricing `json:"pricing"`
	// +kubebuilder:validation:MinItems=1
	// +listType=map
	// +listMapKey=name
	// Destinations lists the named report recipients.
	Destinations []ReportDestination `json:"destinations"`
}

// EnergyReportStatus describes report execution.
type EnergyReportStatus struct {
	// +optional
	// ObservedGeneration is the latest generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	// LastScheduleTime is the latest scheduled execution time.
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
	// +optional
	// LastSuccessfulTime is the latest successful execution time.
	LastSuccessfulTime *metav1.Time `json:"lastSuccessfulTime,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	// Conditions describe the observed report state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="Period",type=string,JSONPath=`.spec.period`

// EnergyReport is the Schema for the energyreports API
type EnergyReport struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of EnergyReport
	// +required
	Spec EnergyReportSpec `json:"spec"`

	// status defines the observed state of EnergyReport
	// +optional
	Status EnergyReportStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// EnergyReportList contains a list of EnergyReport
type EnergyReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []EnergyReport `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &EnergyReport{}, &EnergyReportList{})
		return nil
	})
}
