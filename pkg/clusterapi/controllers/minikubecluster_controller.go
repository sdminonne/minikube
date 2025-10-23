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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
)

const (
	clusterFinalizer = "minikubecluster.infrastructure.cluster.x-k8s.io"
)

// MinikubeClusterReconciler reconciles a MinikubeCluster object
type MinikubeClusterReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	HostBridge bridge.HostBridge
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=minikubeclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=minikubeclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=minikubeclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch

// Reconcile handles MinikubeCluster reconciliation
func (r *MinikubeClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the MinikubeCluster instance
	minikubeCluster := &infrav1.MinikubeCluster{}
	if err := r.Get(ctx, req.NamespacedName, minikubeCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster
	cluster, err := util.GetOwnerCluster(ctx, r.Client, minikubeCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Waiting for Cluster Controller to set OwnerRef on MinikubeCluster")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Return early if the object or Cluster is paused
	if annotations.IsPaused(cluster, minikubeCluster) {
		log.Info("MinikubeCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(minikubeCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to patch the object and status after each reconciliation
	defer func() {
		if err := patchHelper.Patch(ctx, minikubeCluster); err != nil {
			log.Error(err, "failed to patch MinikubeCluster")
		}
	}()

	// Handle deletion reconciliation loop
	if !minikubeCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, cluster, minikubeCluster)
	}

	// Handle normal reconciliation loop
	return r.reconcileNormal(ctx, cluster, minikubeCluster)
}

func (r *MinikubeClusterReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster, minikubeCluster *infrav1.MinikubeCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling MinikubeCluster")

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(minikubeCluster, clusterFinalizer) {
		controllerutil.AddFinalizer(minikubeCluster, clusterFinalizer)
		return ctrl.Result{Requeue: true}, nil
	}

	// Determine the profile name
	profileName := minikubeCluster.Spec.ProfileName
	if profileName == "" {
		profileName = cluster.Name
	}

	// Get cluster config from host
	clusterConfig, err := r.HostBridge.GetClusterConfig(ctx, profileName)
	if err != nil {
		log.Error(err, "failed to get cluster config from host")
		minikubeCluster.Status.Ready = false
		minikubeCluster.Status.FailureReason = ptr("ClusterConfigNotFound")
		minikubeCluster.Status.FailureMessage = ptr(fmt.Sprintf("Failed to get cluster config: %v", err))
		return ctrl.Result{}, err
	}

	// Set the control plane endpoint if not already set
	if minikubeCluster.Spec.ControlPlaneEndpoint.Host == "" {
		// Get the primary control plane node IP
		if len(clusterConfig.Nodes) > 0 {
			primaryNode := clusterConfig.Nodes[0]
			if primaryNode.IP != "" {
				minikubeCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
					Host: primaryNode.IP,
					Port: int32(clusterConfig.APIServerPort),
				}
			}
		}
	}

	// Update status
	minikubeCluster.Status.Ready = true
	minikubeCluster.Status.FailureReason = nil
	minikubeCluster.Status.FailureMessage = nil

	log.Info("MinikubeCluster reconciled successfully", "profileName", profileName)
	return ctrl.Result{}, nil
}

func (r *MinikubeClusterReconciler) reconcileDelete(ctx context.Context, cluster *clusterv1.Cluster, minikubeCluster *infrav1.MinikubeCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling deletion of MinikubeCluster")

	// MinikubeCluster deletion doesn't actually delete the minikube cluster
	// That would be too destructive. Instead, we just clean up our resources.
	// Users must manually run `minikube delete` if they want to remove the cluster.

	// Remove finalizer
	controllerutil.RemoveFinalizer(minikubeCluster, clusterFinalizer)

	log.Info("MinikubeCluster deletion reconciled successfully")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *MinikubeClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.MinikubeCluster{}).
		WithEventFilter(predicates.ResourceNotPaused(ctrl.LoggerFrom(ctx))).
		Complete(r)
}

// ptr returns a pointer to the given value
func ptr(s string) *string {
	return &s
}
