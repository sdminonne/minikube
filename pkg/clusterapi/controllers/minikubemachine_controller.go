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

package controllers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "k8s.io/minikube/pkg/clusterapi/api/v1alpha1"
	"k8s.io/minikube/pkg/clusterapi/bridge"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/node"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
)

const (
	machineFinalizer = "minikubemachine.infrastructure.cluster.x-k8s.io"
	phaseProvisioning = "Provisioning"
	phaseProvisioned  = "Provisioned"
	phaseDeleting     = "Deleting"
	phaseFailed       = "Failed"
)

// MinikubeMachineReconciler reconciles a MinikubeMachine object
type MinikubeMachineReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	HostBridge bridge.HostBridge
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=minikubemachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=minikubemachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=minikubemachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile handles MinikubeMachine reconciliation
func (r *MinikubeMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the MinikubeMachine instance
	minikubeMachine := &infrav1.MinikubeMachine{}
	if err := r.Get(ctx, req.NamespacedName, minikubeMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine
	machine, err := util.GetOwnerMachine(ctx, r.Client, minikubeMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Waiting for Machine Controller to set OwnerRef on MinikubeMachine")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("machine", machine.Name)

	// Fetch the Cluster
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused
	if annotations.IsPaused(cluster, minikubeMachine) {
		log.Info("MinikubeMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	// Fetch the MinikubeCluster
	minikubeCluster := &infrav1.MinikubeCluster{}
	minikubeClusterName := client.ObjectKey{
		Namespace: minikubeMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Get(ctx, minikubeClusterName, minikubeCluster); err != nil {
		log.Info("MinikubeCluster is not available yet")
		return ctrl.Result{}, nil
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(minikubeMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to patch the object and status after each reconciliation
	defer func() {
		if err := patchHelper.Patch(ctx, minikubeMachine); err != nil {
			log.Error(err, "failed to patch MinikubeMachine")
		}
	}()

	// Handle deletion reconciliation loop
	if !minikubeMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, cluster, minikubeCluster, machine, minikubeMachine)
	}

	// Handle normal reconciliation loop
	return r.reconcileNormal(ctx, cluster, minikubeCluster, machine, minikubeMachine)
}

func (r *MinikubeMachineReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster, minikubeCluster *infrav1.MinikubeCluster, machine *clusterv1.Machine, minikubeMachine *infrav1.MinikubeMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling MinikubeMachine")

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(minikubeMachine, machineFinalizer) {
		controllerutil.AddFinalizer(minikubeMachine, machineFinalizer)
		return ctrl.Result{Requeue: true}, nil
	}

	// Determine the profile name
	profileName := minikubeCluster.Spec.ProfileName
	if profileName == "" {
		profileName = cluster.Name
	}

	// Check if node already exists
	if minikubeMachine.Spec.ProviderID != nil && *minikubeMachine.Spec.ProviderID != "" {
		// Node already exists, just update status
		return r.reconcileExistingNode(ctx, profileName, minikubeMachine)
	}

	// Provision new node
	return r.provisionNode(ctx, profileName, minikubeCluster, machine, minikubeMachine)
}

func (r *MinikubeMachineReconciler) provisionNode(ctx context.Context, profileName string, minikubeCluster *infrav1.MinikubeCluster, machine *clusterv1.Machine, minikubeMachine *infrav1.MinikubeMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Provisioning new node")

	// Set phase to provisioning
	minikubeMachine.Status.Phase = phaseProvisioning

	// Get cluster config to determine next node name
	clusterConfig, err := r.HostBridge.GetClusterConfig(ctx, profileName)
	if err != nil {
		log.Error(err, "failed to get cluster config")
		minikubeMachine.Status.Phase = phaseFailed
		minikubeMachine.Status.FailureReason = ptr("ClusterConfigNotFound")
		minikubeMachine.Status.FailureMessage = ptr(fmt.Sprintf("Failed to get cluster config: %v", err))
		return ctrl.Result{}, err
	}

	// Determine node name if not specified
	nodeName := minikubeMachine.Spec.NodeName
	if nodeName == "" {
		lastID := len(clusterConfig.Nodes)
		if len(clusterConfig.Nodes) > 0 {
			lastNode := clusterConfig.Nodes[len(clusterConfig.Nodes)-1]
			id, err := node.ID(lastNode.Name)
			if err == nil {
				lastID = id
			}
		}
		nodeName = node.Name(lastID + 1)
		minikubeMachine.Spec.NodeName = nodeName
	}

	// Determine worker flag
	worker := true
	if minikubeMachine.Spec.Worker != nil {
		worker = *minikubeMachine.Spec.Worker
	}

	// Create node config
	newNode := config.Node{
		Name:              nodeName,
		Worker:            worker,
		ControlPlane:      minikubeMachine.Spec.ControlPlane,
		KubernetesVersion: clusterConfig.KubernetesConfig.KubernetesVersion,
	}

	// Add node via host bridge
	if err := r.HostBridge.AddNode(ctx, profileName, newNode, false); err != nil {
		log.Error(err, "failed to add node")
		minikubeMachine.Status.Phase = phaseFailed
		minikubeMachine.Status.FailureReason = ptr("NodeProvisionFailed")
		minikubeMachine.Status.FailureMessage = ptr(fmt.Sprintf("Failed to provision node: %v", err))
		return ctrl.Result{}, err
	}

	// Get node info to update status
	nodeInfo, err := r.HostBridge.GetNodeInfo(ctx, profileName, nodeName)
	if err != nil {
		log.Error(err, "failed to get node info after provisioning")
		return ctrl.Result{Requeue: true}, nil
	}

	// Update machine status
	providerID := nodeInfo.ProviderID
	minikubeMachine.Spec.ProviderID = &providerID
	minikubeMachine.Status.Phase = phaseProvisioned
	minikubeMachine.Status.Ready = true
	minikubeMachine.Status.Addresses = []corev1.NodeAddress{
		{
			Type:    corev1.NodeInternalIP,
			Address: nodeInfo.IP,
		},
	}
	minikubeMachine.Status.FailureReason = nil
	minikubeMachine.Status.FailureMessage = nil

	log.Info("Node provisioned successfully", "nodeName", nodeName, "providerID", providerID)
	return ctrl.Result{}, nil
}

func (r *MinikubeMachineReconciler) reconcileExistingNode(ctx context.Context, profileName string, minikubeMachine *infrav1.MinikubeMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling existing node", "nodeName", minikubeMachine.Spec.NodeName)

	// Get node info
	nodeInfo, err := r.HostBridge.GetNodeInfo(ctx, profileName, minikubeMachine.Spec.NodeName)
	if err != nil {
		log.Error(err, "failed to get node info")
		minikubeMachine.Status.Ready = false
		return ctrl.Result{}, err
	}

	// Update status
	minikubeMachine.Status.Phase = phaseProvisioned
	minikubeMachine.Status.Ready = nodeInfo.Running
	minikubeMachine.Status.Addresses = []corev1.NodeAddress{
		{
			Type:    corev1.NodeInternalIP,
			Address: nodeInfo.IP,
		},
	}

	return ctrl.Result{}, nil
}

func (r *MinikubeMachineReconciler) reconcileDelete(ctx context.Context, cluster *clusterv1.Cluster, minikubeCluster *infrav1.MinikubeCluster, machine *clusterv1.Machine, minikubeMachine *infrav1.MinikubeMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling deletion of MinikubeMachine")

	minikubeMachine.Status.Phase = phaseDeleting

	// Determine the profile name
	profileName := minikubeCluster.Spec.ProfileName
	if profileName == "" {
		profileName = cluster.Name
	}

	// Delete the node if it exists
	if minikubeMachine.Spec.NodeName != "" {
		if err := r.HostBridge.DeleteNode(ctx, profileName, minikubeMachine.Spec.NodeName); err != nil {
			log.Error(err, "failed to delete node")
			// Continue anyway to allow cleanup
		} else {
			log.Info("Node deleted successfully", "nodeName", minikubeMachine.Spec.NodeName)
		}
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(minikubeMachine, machineFinalizer)

	log.Info("MinikubeMachine deletion reconciled successfully")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *MinikubeMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.MinikubeMachine{}).
		WithEventFilter(predicates.ResourceNotPaused(ctrl.LoggerFrom(ctx))).
		Complete(r)
}
