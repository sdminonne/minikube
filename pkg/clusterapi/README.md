# Minikube Cluster API Provider

A Cluster API infrastructure provider for minikube that enables scaling minikube clusters from within the cluster using Kubernetes-native declarative APIs.

## Overview

This provider implements the [Cluster API](https://cluster-api.sigs.k8s.io/) infrastructure provider contract for minikube, allowing you to:

- Scale your minikube cluster by adding/removing nodes declaratively
- Manage minikube nodes using standard Cluster API resources (Machine, MachineDeployment, etc.)
- Integrate minikube with the broader Cluster API ecosystem

The provider runs **inside** the minikube cluster and communicates with the host's minikube installation to provision and manage nodes.

## Architecture

The provider consists of:

1. **Custom Resources (CRDs)**:
   - `MinikubeCluster`: Represents the minikube cluster infrastructure
   - `MinikubeMachine`: Represents individual minikube nodes

2. **Controllers**:
   - MinikubeCluster controller: Manages cluster-level infrastructure
   - MinikubeMachine controller: Provisions and manages individual nodes

3. **Host Bridge**: Interface for communication between the in-cluster controller and host minikube

## Prerequisites

- A running minikube cluster (v1.30.0 or later recommended)
- Cluster API core components installed
- kubectl configured to access your minikube cluster

## Installation

### 1. Install Cluster API Core Components

```bash
# Install clusterctl
curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.10.0/clusterctl-linux-amd64 -o clusterctl
chmod +x clusterctl
sudo mv clusterctl /usr/local/bin/

# Initialize Cluster API management cluster
clusterctl init
```

### 2. Install the Minikube Provider

```bash
# Apply CRDs
kubectl apply -f pkg/clusterapi/config/crd/

# Apply RBAC
kubectl apply -f pkg/clusterapi/config/rbac/

# Apply deployment
kubectl apply -f pkg/clusterapi/config/manager/
```

### 3. Verify Installation

```bash
# Check that the provider is running
kubectl get pods -n minikube-capi-provider-system

# Check that CRDs are installed
kubectl get crds | grep minikube
```

## Usage

### Basic Example: Add a Worker Node

Create a `MinikubeCluster` and `MinikubeMachine` to add a worker node:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-cluster
  namespace: default
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
      - 10.244.0.0/16
    services:
      cidrBlocks:
      - 10.96.0.0/12
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: MinikubeCluster
    name: my-cluster
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: MinikubeCluster
metadata:
  name: my-cluster
  namespace: default
spec:
  profileName: minikube
  driver: docker
  containerRuntime: containerd
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: my-cluster-worker-1
  namespace: default
  labels:
    cluster.x-k8s.io/cluster-name: my-cluster
spec:
  clusterName: my-cluster
  version: v1.28.0
  bootstrap:
    dataSecretName: worker-bootstrap-data
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: MinikubeMachine
    name: my-cluster-worker-1
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: MinikubeMachine
metadata:
  name: my-cluster-worker-1
  namespace: default
  labels:
    cluster.x-k8s.io/cluster-name: my-cluster
spec:
  worker: true
  controlPlane: false
  cpus: 2
  memory: 2048
```

Apply the resources:

```bash
kubectl apply -f cluster.yaml
```

Check the status:

```bash
# Watch the machine being provisioned
kubectl get minikubemachines -w

# Check the node is added to the cluster
kubectl get nodes
```

### Using MachineDeployment

For more advanced use cases, you can use a MachineDeployment to manage a set of worker nodes:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: my-cluster-md-0
  namespace: default
spec:
  clusterName: my-cluster
  replicas: 3
  selector:
    matchLabels:
      cluster.x-k8s.io/cluster-name: my-cluster
      cluster.x-k8s.io/deployment-name: my-cluster-md-0
  template:
    metadata:
      labels:
        cluster.x-k8s.io/cluster-name: my-cluster
        cluster.x-k8s.io/deployment-name: my-cluster-md-0
    spec:
      clusterName: my-cluster
      version: v1.28.0
      bootstrap:
        dataSecretName: worker-bootstrap-data
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
        kind: MinikubeMachine
        name: my-cluster-md-0
---
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: MinikubeMachine
metadata:
  name: my-cluster-md-0
  namespace: default
spec:
  worker: true
  cpus: 2
  memory: 2048
```

Scale the deployment:

```bash
# Scale to 5 nodes
kubectl scale machinedeployment my-cluster-md-0 --replicas=5

# Scale down to 2 nodes
kubectl scale machinedeployment my-cluster-md-0 --replicas=2
```

### Advanced Configuration

#### Custom Node Resources

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: MinikubeMachine
metadata:
  name: my-cluster-worker-custom
  namespace: default
spec:
  nodeName: m05  # Custom node name
  worker: true
  cpus: 4
  memory: 4096
  diskSize: 20000
  extraOptions:
    feature-gates: "SomeFeature=true"
```

#### Control Plane Node

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
kind: MinikubeMachine
metadata:
  name: my-cluster-cp-2
  namespace: default
spec:
  controlPlane: true
  worker: false
  cpus: 2
  memory: 2048
```

## How It Works

1. **User creates a MinikubeMachine resource** via kubectl
2. **MinikubeMachine controller reconciles** the resource:
   - Reads the minikube cluster configuration from the host
   - Determines the next available node name
   - Calls `node.Add()` via the HostBridge to provision the node
   - Updates the MinikubeMachine status with node information
3. **Node joins the cluster** using minikube's standard node provisioning
4. **Cluster API tracks** the node lifecycle through standard Machine resources

## Limitations

- The provider must run inside the minikube cluster it manages
- Requires access to the host's minikube storage directory (`/var/lib/minikube`)
- Only supports drivers that allow multi-node clusters (docker, kvm2, etc.)
- Does not support the `none` (bare metal) driver
- Deleting a MinikubeCluster does NOT delete the underlying minikube cluster (must run `minikube delete` manually)

## Troubleshooting

### Provider pod not starting

Check the deployment logs:
```bash
kubectl logs -n minikube-capi-provider-system deployment/minikube-capi-provider-controller-manager
```

Common issues:
- Volume mount paths incorrect (check deployment.yaml)
- Insufficient RBAC permissions
- Cluster API CRDs not installed

### Node provisioning fails

Check the MinikubeMachine status:
```bash
kubectl describe minikubemachine <machine-name>
```

Common issues:
- Profile name mismatch
- Insufficient resources on host
- Network configuration issues

### Check controller logs

```bash
kubectl logs -n minikube-capi-provider-system -l control-plane=controller-manager --tail=100 -f
```

## Development

### Building from source

```bash
# Build the provider binary
go build -o bin/minikube-capi-provider cmd/minikube-capi-provider/main.go

# Build Docker image
docker build -t minikube-capi-provider:dev -f pkg/clusterapi/Dockerfile .

# Load into minikube
minikube image load minikube-capi-provider:dev
```

### Running locally

For development, you can run the controller outside the cluster:

```bash
# Export kubeconfig
export KUBECONFIG=~/.kube/config

# Run the controller
go run cmd/minikube-capi-provider/main.go \
  --storage-path=/var/lib/minikube \
  --profile=minikube
```

## Contributing

Contributions are welcome! Please see the main [minikube contributing guide](../../CONTRIBUTING.md).

## License

Apache 2.0 - See [LICENSE](../../LICENSE) for details.
