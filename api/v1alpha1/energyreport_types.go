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
// +kubebuilder:validation:XValidation:rule="self.type != 'telegram' || has(self.telegram)",message="telegram configuration is required for type telegram"
type ReportDestination struct {
	// +kubebuilder:validation:MinLength=1
	// Name uniquely identifies a destination within the report.
	Name string `json:"name"`
	// +kubebuilder:validation:Enum=telegram
	// Type identifies the destination integration.
	Type string `json:"type"`
	// +optional
	// Telegram contains the Telegram delivery configuration.
	Telegram *TelegramDestination `json:"telegram,omitempty"`
}

// PrometheusConfig identifies the metrics service and optional credentials.
type PrometheusConfig struct {
	// +kubebuilder:validation:MinLength=1
	// URL is the base URL of the Prometheus HTTP API.
	URL string `json:"url"`
	// +optional
	// BearerTokenSecretRef identifies a same-namespace bearer token.
	BearerTokenSecretRef *SecretKeyReference `json:"bearerTokenSecretRef,omitempty"`
	// +optional
	// CASecretRef identifies a same-namespace trusted CA bundle.
	CASecretRef *SecretKeyReference `json:"caSecretRef,omitempty"`
}

// EnergyReportSpec defines a scheduled energy report.
type EnergyReportSpec struct {
	// +optional
	// Prometheus identifies the metrics service and is required to schedule a report.
	Prometheus *PrometheusConfig `json:"prometheus,omitempty"`
	// +optional
	// Suspend prevents scheduled executions.
	Suspend bool `json:"suspend,omitempty"`
	// +kubebuilder:validation:Pattern=`^[^ ]+ [^ ]+ [^ ]+ [^ ]+ [^ ]+$`
	// Schedule is a five-field cron expression.
	Schedule string `json:"schedule"`
	// +kubebuilder:default=UTC
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9_+/-]+$`
	// +optional
	// TimeZone is the IANA timezone for the schedule and report windows.
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

// DestinationProgress records confirmed messages for one destination.
type DestinationProgress struct {
	// Name identifies the destination.
	Name string `json:"name"`
	// ConfirmedChunks is the number of sequentially confirmed messages.
	ConfirmedChunks int32 `json:"confirmedChunks"`
}

// ReportRunStatus records the latest scheduled execution.
type ReportRunStatus struct {
	// ScheduledTime identifies the scheduled execution.
	ScheduledTime metav1.Time `json:"scheduledTime"`
	// Generation is the report generation used by this execution.
	Generation int64 `json:"generation"`
	// State is the latest execution state.
	State string `json:"state"`
	// +optional
	// +listType=map
	// +listMapKey=name
	// Destinations records delivery progress.
	Destinations []DestinationProgress `json:"destinations,omitempty"`
}

// EnergyReportStatus describes report execution.
type EnergyReportStatus struct {
	// +optional
	// LastRun records the latest execution and its delivery progress.
	LastRun *ReportRunStatus `json:"lastRun,omitempty"`
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
