/*
Copyright 2025.

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
)
// ResourceSleepParams defines the sleep configuration for a specific resource or default.
type ResourceSleepParams struct {
	Name     string
	Namespace string
	Kind     string
	SleepAt  string
	WakeAt   string
	Timezone string
}
// PolicyConfig defines the sleep configuration for a specific resource or default.
type PolicyConfig struct {
	// +kubebuilder:validation:Required
	Enable bool `json:"enable"`
	// +kubebuilder:validation:Pattern=`^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$`
	// +optional
	WakeAt string `json:"wakeAt,omitempty"`
	// +kubebuilder:validation:Pattern=`^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$`
	// +optional
	SleepAt string `json:"sleepAt,omitempty"`
	// +optional
	Timezone string `json:"timezone,omitempty"`
}

// SleepPolicySpec defines the desired state of SleepPolicy.
type SleepPolicySpec struct {
	// Global Timezone for the policy.
	// +kubebuilder:default="UTC"
	// +optional
	Timezone string `json:"timezone,omitempty"`

	// Map of Deployments to manage. Key "default" is required for global settings.
	Deployments map[string]PolicyConfig `json:"deployments,omitempty"`

	// Map of StatefulSets to manage. Key "default" is required for global settings.
	StatefulSets map[string]PolicyConfig `json:"statefulSets,omitempty"`
}

// SleepPolicyStatus defines the observed state of SleepPolicy.
type SleepPolicyStatus struct {
	// List of resources currently managed by this policy.
	// +optional
	ManagedResources []string `json:"managedResources,omitempty"`
	// +optional
	State map[string]ResourceSleepParams `json:"state,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SleepPolicy is the Schema for the sleeppolicies API
type SleepPolicy struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of SleepPolicy
	// +required
	Spec SleepPolicySpec `json:"spec"`

	// status defines the observed state of SleepPolicy
	// +optional
	Status SleepPolicyStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// SleepPolicyList contains a list of SleepPolicy
type SleepPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SleepPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SleepPolicy{}, &SleepPolicyList{})
}
