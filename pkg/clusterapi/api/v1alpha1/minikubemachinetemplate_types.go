/*
Copyright 2025 The Kubernetes Authors All rights reserved.

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

// MinikubeMachineTemplateSpec defines the desired state of MinikubeMachineTemplate
type MinikubeMachineTemplateSpec struct {
	Template MinikubeMachineTemplateResource `json:"template"`
}

// MinikubeMachineTemplateResource describes the data needed to create a MinikubeMachine from a template
type MinikubeMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec MinikubeMachineSpec `json:"spec"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=minikubemachinetemplates,scope=Namespaced,categories=cluster-api

// MinikubeMachineTemplate is the Schema for the minikubemachinetemplates API
type MinikubeMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MinikubeMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// MinikubeMachineTemplateList contains a list of MinikubeMachineTemplate
type MinikubeMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinikubeMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MinikubeMachineTemplate{}, &MinikubeMachineTemplateList{})
}
