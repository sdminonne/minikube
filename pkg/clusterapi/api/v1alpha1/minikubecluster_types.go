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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// MinikubeClusterSpec defines the desired state of MinikubeCluster
type MinikubeClusterSpec struct {
	// ProfileName is the minikube profile name for this cluster
	// +optional
	ProfileName string `json:"profileName,omitempty"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`

	// Driver specifies the minikube driver to use (docker, kvm2, hyperkit, etc.)
	// Defaults to the driver used to create the initial cluster
	// +optional
	Driver string `json:"driver,omitempty"`

	// ContainerRuntime specifies the container runtime (docker, containerd, crio)
	// +optional
	ContainerRuntime string `json:"containerRuntime,omitempty"`

	// NetworkPlugin specifies the CNI plugin to use
	// +optional
	NetworkPlugin string `json:"networkPlugin,omitempty"`
}

// MinikubeClusterStatus defines the observed state of MinikubeCluster
type MinikubeClusterStatus struct {
	// Ready indicates the cluster infrastructure is ready
	Ready bool `json:"ready"`

	// Conditions defines current service state of the MinikubeCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the MinikubeCluster and will contain a succinct value suitable
	// for machine interpretation.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the MinikubeCluster and will contain a more verbose string suitable
	// for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=minikubeclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this MinikubeCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.controlPlaneEndpoint.host",description="Control plane endpoint"

// MinikubeCluster is the Schema for the minikubeclusters API
type MinikubeCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinikubeClusterSpec   `json:"spec,omitempty"`
	Status MinikubeClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MinikubeClusterList contains a list of MinikubeCluster
type MinikubeClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinikubeCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MinikubeCluster{}, &MinikubeClusterList{})
}
