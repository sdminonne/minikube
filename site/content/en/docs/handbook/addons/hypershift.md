---
title: "Using the HyperShift Addon"
linkTitle: "HyperShift"
weight: 1
date: 2025-11-09
---

## Prerequisites

- [KubeVirt addon]({{< ref "/docs/handbook/addons/kubevirt" >}}) enabled for hosting local KubeVirt guest clusters
- `hypershift` CLI installed (can be extracted from `quay.io/hypershift/hypershift-operator:latest`)

### Resource Sizing

- **Base footprint (minikube + KubeVirt + CDI + 1 Hosted Control Plane):** ~9–10 GB RAM and ~15 GB disk.
- **Per KubeVirt worker node:** Add **~4.5–5 GB RAM** (4 GiB default guest VM memory + ~300–500 MiB `virt-launcher`/QEMU overhead) and **~20–30 GB disk** (32 GiB default RHCOS root volume).

| Setup | Recommended `--cpus` | Recommended `--memory` | Recommended `--disk-size` |
|---|---|---|---|
| **0 workers** (HCP only) | `4` | `10240` (10 GB) | `20g` |
| **1 worker** (`--memory 4Gi`) | `6` – `8` | `14336` – `16384` (14–16 GB) | `50g` |
| **2 workers** (`--memory 4Gi`) | `8`+ | `18432` – `20480` (18–20 GB) | `80g` |

> **Tip:** On hosts with limited resources, you can reduce per-worker requirements by passing `--memory 2Gi` (or `3Gi`) and `--root-volume-size 16` to `hypershift create cluster kubevirt` or `hypershift create nodepool kubevirt`.

### Enable HyperShift on minikube

Start minikube with sufficient resources and enable the `kubevirt` and `hypershift` addons:

```shell
minikube start --cpus=8 --memory=16384 --disk-size=50g
minikube addons enable kubevirt
minikube addons enable hypershift
```

#### Using the `docker` or `podman` Driver (Faster Worker VM Performance)

With `--driver=kvm2`, KubeVirt worker VMs run inside nested virtualization (`Host KVM` → `minikube VM` → `KubeVirt worker VM`). On Linux, you can eliminate the outer VM layer by using the `docker` or `podman` driver (`--driver=docker` or `--driver=podman`), which runs minikube in a privileged container with direct access to the host's `/dev/kvm`, `/dev/vhost-net`, and `/dev/net/tun`:

```shell
# Using Docker
minikube start --driver=docker --cpus=8 --memory=16384 --disk-size=50g --addons=kubevirt,hypershift

# Using Podman (rootful mode)
minikube start --driver=podman --container-runtime=containerd --cpus=8 --memory=16384 --disk-size=50g --addons=kubevirt,hypershift
```

> **Note for Podman:** Keep Podman in **rootful** mode (minikube's default, which invokes `sudo podman`) and do **not** set `rootless=true`. KubeVirt's `virt-handler` and `virt-launcher` pods require real root privileges on `/dev/kvm`, `/dev/net/tun`, and device cgroups, which fail inside a rootless Podman user namespace.

Once enabled, verify that the HyperShift operator pod is running in the `hypershift` namespace:

```shell
minikube kubectl -- get pods -n hypershift
```

### Install the HyperShift CLI

If you do not already have the `hypershift` CLI installed, you can extract the matching binary from the operator container image:

```shell
podman create --name hs-cli quay.io/hypershift/hypershift-operator:latest
podman cp hs-cli:/usr/bin/hypershift ~/.local/bin/hypershift
podman rm hs-cli
chmod +x ~/.local/bin/hypershift
```

### Create a Hosted Control Plane (HCP)

1. Prepare a pull secret file (or a dummy secret when testing public images):

   ```shell
   echo '{"auths":{"fake":{"auth":"aWQ6cGFzcwo="}}}' > pull-secret.json
   ```

2. Create a KubeVirt `HostedCluster` with `--node-pool-replicas 0` (control plane only), `--base-domain`, `--service-publishing-strategy NodePort`, and the `baseline` PodSecurity override annotation:

   ```shell
   hypershift create cluster kubevirt \
     --name example \
     --node-pool-replicas 0 \
     --base-domain example.com \
     --service-publishing-strategy NodePort \
     --annotations hypershift.openshift.io/pod-security-admission-label-override=baseline \
     --pull-secret ./pull-secret.json \
     --release-image quay.io/openshift-release-dev/ocp-release:4.18.1-x86_64 \
     --generate-ssh
   ```

3. Monitor the `HostedCluster` and its control plane pods in the `clusters-example` namespace until `AVAILABLE` is `True`:

   ```shell
   minikube kubectl -- get hostedclusters -n clusters -w
   minikube kubectl -- get pods -n clusters-example
   ```

4. Generate the `kubeconfig` for the `example` Hosted Control Plane and query its API server:

   ```shell
   hypershift create kubeconfig --name example --namespace clusters > example.kubeconfig
   minikube kubectl -- --kubeconfig=example.kubeconfig --cluster=cluster get namespaces
   ```

### Provision Worker Nodes with a NodePool

To provision KubeVirt worker VMs for the `HostedCluster`:

1. Install the Containerized Data Importer (CDI) on the management cluster so KubeVirt can import the node root disk `DataVolume`:

   ```shell
   export CDI_VERSION=v1.61.0
   minikube kubectl -- create -f https://github.com/kubevirt/containerized-data-importer/releases/download/$CDI_VERSION/cdi-operator.yaml
   minikube kubectl -- create -f https://github.com/kubevirt/containerized-data-importer/releases/download/$CDI_VERSION/cdi-cr.yaml
   ```

2. Scale the default `NodePool` created with the cluster (or create a new `NodePool`):

   ```shell
   # Scale the existing NodePool to 1 worker node
   minikube kubectl -- scale nodepool example -n clusters --replicas=1

   # Or create an additional NodePool using the hypershift CLI
   hypershift create nodepool kubevirt \
     --cluster-name example \
     --name example-workers \
     --node-count 1 \
     --cores 2 \
     --memory 4Gi \
     --root-volume-cache-strategy=PVC
   ```

   > **Tip:** Passing `--root-volume-cache-strategy=PVC` caches the imported RHCOS disk image in a single shared PVC and clones it for additional worker VMs instead of re-importing the container image for each node.

3. Monitor the `NodePool`, KubeVirt `VirtualMachine`, and guest cluster `Node` status:

   ```shell
   minikube kubectl -- get nodepools -n clusters
   minikube kubectl -- get vm -n clusters-example
   minikube kubectl -- --kubeconfig=example.kubeconfig --cluster=cluster get nodes
   ```

### Deprovision Worker Nodes and the Hosted Control Plane

1. **Scale down or delete a `NodePool`** to deprovision its KubeVirt worker VMs:

   ```shell
   # Scale the NodePool down to 0 worker nodes
   minikube kubectl -- scale nodepool example -n clusters --replicas=0

   # Or delete an additional NodePool
   minikube kubectl -- delete nodepool example-workers -n clusters
   ```

2. **Destroy the `HostedCluster`** and its control plane namespace (`clusters-example`):

   ```shell
   hypershift destroy cluster kubevirt --name example --namespace clusters
   ```

   Alternatively, you can delete the `NodePool` and `HostedCluster` resources directly via `kubectl`:

   ```shell
   minikube kubectl -- delete nodepool example -n clusters
   minikube kubectl -- delete hostedcluster example -n clusters
   ```

### Disable HyperShift

To disable the `hypershift` and `kubevirt` addons (or delete the minikube cluster completely), run:

```shell
minikube addons disable hypershift
minikube addons disable kubevirt

# Or remove the entire minikube cluster:
minikube delete
```

### More Information

See the official [HyperShift documentation](https://hypershift-docs.netlify.app/) for more details on creating and managing `HostedCluster` resources.
