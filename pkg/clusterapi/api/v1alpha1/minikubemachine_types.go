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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// MinikubeMachineSpec defines the desired state of MinikubeMachine
type MinikubeMachineSpec struct {
	// ProviderID is the unique identifier as specified by the cloud provider.
	// For minikube, this will be in the format: minikube://<profile-name>/<node-name>
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// NodeName is the name of the minikube node (e.g., "m02", "m03")
	// If not specified, it will be automatically generated
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// ControlPlane indicates this is a control-plane node
	// +optional
	ControlPlane bool `json:"controlPlane,omitempty"`

	// Worker indicates this is a worker node (default: true)
	// +optional
	Worker *bool `json:"worker,omitempty"`

	// CPUs specifies the number of CPUs for this node
	// +optional
	CPUs int `json:"cpus,omitempty"`

	// Memory specifies the amount of memory in MB for this node
	// +optional
	Memory int `json:"memory,omitempty"`

	// DiskSize specifies the disk size in MB for this node
	// +optional
	DiskSize int `json:"diskSize,omitempty"`

	// ExtraOptions allows passing additional options to minikube
	// +optional
	ExtraOptions map[string]string `json:"extraOptions,omitempty"`
}

// MinikubeMachineStatus defines the observed state of MinikubeMachine
type MinikubeMachineStatus struct {
	// Ready indicates the machine infrastructure is ready
	Ready bool `json:"ready"`

	// Addresses contains the addresses associated with the minikube machine.
	// +optional
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`

	// Conditions defines current service state of the MinikubeMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Phase represents the current phase of machine actuation.
	// E.g. Pending, Running, Terminating, Failed etc.
	// +optional
	Phase string `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=minikubemachines,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this MinikubeMachine belongs"
// +kubebuilder:printcolumn:name="Node",type="string",JSONPath=".spec.nodeName",description="Minikube node name"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Machine is ready"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Machine phase"

// MinikubeMachine is the Schema for the minikubemachines API
type MinikubeMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinikubeMachineSpec   `json:"spec,omitempty"`
	Status MinikubeMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MinikubeMachineList contains a list of MinikubeMachine
type MinikubeMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinikubeMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MinikubeMachine{}, &MinikubeMachineList{})
}
