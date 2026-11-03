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

// SecretKeyReference identifies a required key in a Secret in the same namespace.
type SecretKeyReference struct {
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// Name is the name of the Secret in the resource namespace.
	Name string `json:"name"`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9._-]+$`
	// Key is the required entry in the Secret.
	Key string `json:"key"`
}

// TapoConfig configures the connection to a Tapo device.
type TapoConfig struct {
	// +kubebuilder:validation:MinLength=1
	// Address is the device hostname or IP address.
	Address string `json:"address"`
	// EmailSecretRef points to the Tapo account email.
	EmailSecretRef SecretKeyReference `json:"emailSecretRef"`
	// PasswordSecretRef points to the Tapo account password.
	PasswordSecretRef SecretKeyReference `json:"passwordSecretRef"`
}

// DeviceMetricsConfig configures the collector's secure metrics endpoint.
type DeviceMetricsConfig struct {
	// +kubebuilder:validation:MinLength=1
	// ServingCertSecretName names a TLS Secret in the device namespace.
	ServingCertSecretName string `json:"servingCertSecretName"`
	// +optional
	// CASecretRef identifies the trust anchor for the ServiceMonitor.
	CASecretRef *SecretKeyReference `json:"caSecretRef,omitempty"`
}

// EnergyDeviceSpec defines the desired state of an energy device.
// +kubebuilder:validation:XValidation:rule="self.provider != 'tapo' || has(self.tapo)",message="tapo configuration is required for provider tapo"
type EnergyDeviceSpec struct {
	// +kubebuilder:validation:Enum=tapo
	// Provider identifies the device integration.
	Provider string `json:"provider"`
	// +optional
	// Tapo configures the device connection.
	Tapo *TapoConfig `json:"tapo,omitempty"`
	// +optional
	// Metrics configures the collector metrics endpoint.
	Metrics *DeviceMetricsConfig `json:"metrics,omitempty"`
	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=1
	// +optional
	// CollectionIntervalSeconds is the planned polling interval in seconds.
	CollectionIntervalSeconds int32 `json:"collectionIntervalSeconds,omitempty"`
}

// EnergyDeviceStatus describes the observed state.
type EnergyDeviceStatus struct {
	// +optional
	// ObservedGeneration is the latest generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	// LastObservationTime is the most recent successful device observation.
	LastObservationTime *metav1.Time `json:"lastObservationTime,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	// Conditions describe the observed device health.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Provider",type=string,JSONPath=`.spec.provider`

// EnergyDevice is the Schema for the energydevices API
type EnergyDevice struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of EnergyDevice
	// +required
	Spec EnergyDeviceSpec `json:"spec"`

	// status defines the observed state of EnergyDevice
	// +optional
	Status EnergyDeviceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// EnergyDeviceList contains a list of EnergyDevice
type EnergyDeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []EnergyDevice `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &EnergyDevice{}, &EnergyDeviceList{})
		return nil
	})
}
