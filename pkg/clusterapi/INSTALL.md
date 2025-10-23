# Installation Guide - Minikube Cluster API Provider

This guide walks you through installing and setting up the Minikube Cluster API Provider.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Installation Steps](#installation-steps)
3. [Verification](#verification)
4. [First Node Addition](#first-node-addition)
5. [Troubleshooting](#troubleshooting)

## Prerequisites

### Required

- **minikube** v1.30.0 or later
- **kubectl** v1.28.0 or later
- **Docker** or another container runtime supported by minikube
- A running minikube cluster with at least:
  - 4 CPU cores
  - 8GB RAM
  - Multi-node support (driver: docker, kvm2, hyperkit, etc.)

### Verify Prerequisites

```bash
# Check minikube version
minikube version

# Check your minikube cluster is running
minikube status

# Check driver supports multi-node
minikube profile list

# Verify kubectl access
kubectl cluster-info
```

## Installation Steps

### Step 1: Install Cluster API Core Components

Install `clusterctl` CLI:

```bash
# Download clusterctl
curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.10.0/clusterctl-linux-amd64 -o clusterctl

# Make it executable
chmod +x clusterctl

# Move to PATH
sudo mv clusterctl /usr/local/bin/

# Verify installation
clusterctl version
```

Initialize Cluster API in your minikube cluster:

```bash
# Initialize CAPI management components
clusterctl init

# Wait for components to be ready
kubectl wait --for=condition=Available --timeout=5m \
  deployment/capi-controller-manager \
  -n capi-system
```

### Step 2: Install Minikube Provider CRDs

```bash
# Apply Custom Resource Definitions
kubectl apply -f pkg/clusterapi/config/crd/infrastructure.cluster.x-k8s.io_minikubeclusters.yaml
kubectl apply -f pkg/clusterapi/config/crd/infrastructure.cluster.x-k8s.io_minikubemachines.yaml
kubectl apply -f pkg/clusterapi/config/crd/infrastructure.cluster.x-k8s.io_minikubemachinetemplates.yaml

# Verify CRDs are installed
kubectl get crds | grep minikube
```

Expected output:
```
minikubeclusters.infrastructure.cluster.x-k8s.io
minikubemachines.infrastructure.cluster.x-k8s.io
minikubemachinetemplates.infrastructure.cluster.x-k8s.io
```

### Step 3: Create Provider Namespace and RBAC

```bash
# Create namespace
kubectl create namespace minikube-capi-provider-system

# Apply RBAC resources
kubectl apply -f pkg/clusterapi/config/rbac/service_account.yaml
kubectl apply -f pkg/clusterapi/config/rbac/role.yaml
kubectl apply -f pkg/clusterapi/config/rbac/role_binding.yaml
```

### Step 4: Build and Load Provider Image

**Option A: Build from source**

```bash
# Build the provider binary
cd /path/to/minikube
go build -o bin/minikube-capi-provider cmd/minikube-capi-provider/main.go

# Build Docker image
docker build -f pkg/clusterapi/Dockerfile -t minikube-capi-provider:latest .

# Load image into minikube
minikube image load minikube-capi-provider:latest
```

**Option B: Use pre-built image** (if available)

```bash
# Pull and load pre-built image
docker pull ghcr.io/kubernetes/minikube-capi-provider:v0.1.0
minikube image load ghcr.io/kubernetes/minikube-capi-provider:v0.1.0
```

### Step 5: Deploy the Provider Controller

Before deploying, you may need to adjust the volume mounts in the deployment manifest based on your minikube setup:

```bash
# Check minikube storage location
minikube ssh "ls -la /var/lib/minikube"

# Check minikube home directory
minikube ssh "echo \$HOME"
```

Edit `pkg/clusterapi/config/manager/deployment.yaml` if needed, then deploy:

```bash
# Deploy the controller
kubectl apply -f pkg/clusterapi/config/manager/deployment.yaml

# Wait for the deployment to be ready
kubectl rollout status deployment/minikube-capi-provider-controller-manager \
  -n minikube-capi-provider-system

# Check pod is running
kubectl get pods -n minikube-capi-provider-system
```

## Verification

### Verify Provider is Running

```bash
# Check pod status
kubectl get pods -n minikube-capi-provider-system

# Check logs
kubectl logs -n minikube-capi-provider-system \
  -l control-plane=controller-manager \
  --tail=50
```

Expected log output should show:
```
INFO    setup   starting manager
INFO    controller-runtime.manager      Starting server
INFO    controller-runtime.webhook      Registering webhook
```

### Verify CRDs are Available

```bash
# Test creating a minimal MinikubeCluster
kubectl apply -f - <<EOF
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: MinikubeCluster
metadata:
  name: test-cluster
  namespace: default
spec:
  profileName: minikube
EOF

# Check it was created
kubectl get minikubeclusters

# Clean up test resource
kubectl delete minikubecluster test-cluster
```

## First Node Addition

Let's add your first worker node using the quick-start example:

```bash
# Apply the quick-start manifest
kubectl apply -f pkg/clusterapi/examples/quick-start.yaml

# Watch the machine being created
kubectl get minikubemachines -w

# In another terminal, watch nodes
kubectl get nodes -w
```

You should see:
1. MinikubeMachine resource created
2. Controller provisioning the node (Phase: Provisioning)
3. Node joining the cluster
4. MinikubeMachine status updated (Phase: Provisioned, Ready: true)
5. New node appearing in `kubectl get nodes`

### Verify the Node

```bash
# List all nodes
kubectl get nodes

# Check minikube nodes
minikube node list

# Describe the MinikubeMachine
kubectl describe minikubemachine minikube-worker-1
```

### Scale Up

Try adding more nodes using a MachineDeployment:

```bash
# Apply machinedeployment example
kubectl apply -f pkg/clusterapi/examples/machinedeployment.yaml

# Scale to 5 workers
kubectl scale machinedeployment minikube-workers --replicas=5

# Watch nodes being added
kubectl get nodes -w
```

## Troubleshooting

### Provider Pod Not Starting

**Issue**: Pod stuck in `Pending` or `CrashLoopBackOff`

```bash
# Check pod events
kubectl describe pod -n minikube-capi-provider-system \
  -l control-plane=controller-manager

# Common fixes:
# 1. Volume mount issues - verify paths in deployment.yaml
# 2. RBAC issues - verify service account and roles
# 3. Image not loaded - run: minikube image ls | grep capi-provider
```

### Node Provisioning Fails

**Issue**: MinikubeMachine shows FailureReason

```bash
# Check machine status
kubectl describe minikubemachine <name>

# Check controller logs
kubectl logs -n minikube-capi-provider-system \
  -l control-plane=controller-manager \
  --tail=100

# Common issues:
# 1. Profile name mismatch - check spec.profileName
# 2. Insufficient resources - check host resources
# 3. Driver doesn't support multi-node - verify driver
```

### Permission Errors

**Issue**: Controller logs show permission denied errors

```bash
# Check RBAC
kubectl auth can-i create minikubemachines \
  --as=system:serviceaccount:minikube-capi-provider-system:minikube-capi-provider-controller-manager

# Reapply RBAC if needed
kubectl apply -f pkg/clusterapi/config/rbac/
```

### Volume Mount Issues

**Issue**: Controller can't access minikube storage

```bash
# Verify volume mounts on host
minikube ssh "ls -la /var/lib/minikube"

# Check if paths match deployment
kubectl get deployment -n minikube-capi-provider-system \
  minikube-capi-provider-controller-manager \
  -o yaml | grep -A 10 volumes
```

## Uninstallation

To remove the provider:

```bash
# Delete all MinikubeMachine resources first
kubectl delete minikubemachines --all

# Delete the provider deployment
kubectl delete -f pkg/clusterapi/config/manager/deployment.yaml

# Delete RBAC
kubectl delete -f pkg/clusterapi/config/rbac/

# Delete CRDs (this will delete all MinikubeCluster/Machine resources)
kubectl delete -f pkg/clusterapi/config/crd/

# Optionally remove Cluster API core
clusterctl delete --all
```

**Note**: Deleting MinikubeCluster resources does NOT delete your actual minikube cluster. Use `minikube delete` if you want to remove the cluster.

## Next Steps

- Read the [README](README.md) for usage examples
- Try the [MachineDeployment example](examples/machinedeployment.yaml)
- Check out the [Cluster API documentation](https://cluster-api.sigs.k8s.io/)

## Support

For issues and questions:
- GitHub Issues: https://github.com/kubernetes/minikube/issues
- Cluster API Slack: #cluster-api on Kubernetes Slack
