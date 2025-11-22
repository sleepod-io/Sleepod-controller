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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SleepOrderSpec defines the desired state of SleepOrder
type SleepOrderSpec struct {
    // +kubebuilder:validation:Required
    TargetRef TargetRef `json:"targetRef"`
    // +kubebuilder:validation:Pattern=`^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$`
    WakeAt string `json:"wakeAt"`
    // +kubebuilder:validation:Pattern=`^([0-1]?[0-9]|2[0-3]):[0-5][0-9]$`
    SleepAt string `json:"sleepAt"`
    // +kubebuilder:default="UTC"
    Timezone string `json:"timezone,omitempty"`
}

type TargetRef struct {
	// +kubebuilder:default=apps
    APIVersion string `json:"apiVersion"`
	// +kubebuilder:validation:Enum=Deployment;StatefulSet
    Kind string `json:"kind"`
    Name string `json:"name"`
}

// SleepOrderStatus defines the observed state of SleepOrder.
type SleepOrderStatus struct {
    // +kubebuilder:validation:Enum=Sleeping;Awake;Error
    CurrentState string `json:"currentState,omitempty"`
    OriginalReplicas *int32 `json:"originalReplicas,omitempty"`
    LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
	NextOperationTime *metav1.Time `json:"nextOperationTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SleepOrder is the Schema for the sleeporders API
type SleepOrder struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of SleepOrder
	// +required
	Spec SleepOrderSpec `json:"spec"`

	// status defines the observed state of SleepOrder
	// +optional
	Status SleepOrderStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// SleepOrderList contains a list of SleepOrder
type SleepOrderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SleepOrder `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SleepOrder{}, &SleepOrderList{})
}
