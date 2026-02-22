# provider-kind

A native [Crossplane](https://crossplane.io/) provider for
[KIND (Kubernetes IN Docker)](https://kind.sigs.k8s.io/). It manages KIND
clusters as Crossplane managed resources — no Terraform, no cloud credentials.

---

## Table of Contents

- [Motivation](#motivation)
- [Overview](#overview)
- [Implementation](#implementation)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Examples](#examples)
- [API Reference](#api-reference)
- [How to Contribute](#how-to-contribute)

---

## Motivation

KIND is the de-facto standard for running Kubernetes clusters locally and in CI
pipelines. Teams use KIND for:

- **Local development** — spin up a cluster, try something, tear it down
- **CI environments** — ephemeral clusters for integration and e2e test suites
- **Crossplane composition testing** — validate compositions against a real API
  server without touching a cloud account
- **Training and demos** — reproducible, cost-free cluster environments

Managing KIND clusters by hand (`kind create cluster`, `kind delete cluster`)
works fine for one developer but does not scale. There is no declarative record
of which clusters exist, who created them, or what configuration they carry. When
something goes wrong there is nothing to reconcile against.

`provider-kind` brings KIND clusters into the Crossplane control plane so they
can be managed with the same GitOps workflows, RBAC policies, and composition
patterns used for production infrastructure. A Crossplane `Cluster` resource
becomes the single source of truth: apply it and the cluster appears; delete it
and the cluster is cleaned up — automatically, idempotently, and declaratively.

---

## Overview

`provider-kind` implements two managed resource types:

| Kind | API Group | Scope | Pattern |
|---|---|---|---|
| `Cluster` | `kind.crossplane.io/v1alpha1` | Cluster-scoped | LegacyManaged |
| `Cluster` | `kind.m.crossplane.io/v1alpha1` | Namespaced | ModernManaged |

Both types support the same set of parameters (node topology, networking, port
mappings, feature gates, etc.) and publish the cluster kubeconfig as a
Kubernetes `Secret` upon successful provisioning.

The **cluster-scoped** variant (`kind.crossplane.io`) follows the
[LegacyManaged](https://docs.crossplane.io/latest/concepts/managed-resources/)
pattern and is a good fit for shared, platform-team-owned clusters.

The **namespaced** variant (`kind.m.crossplane.io`) follows the
[ModernManaged](https://docs.crossplane.io/latest/concepts/managed-resources/)
pattern introduced in Crossplane v2 and is a good fit for tenant-owned clusters
where namespace isolation matters.

---

## Implementation

### Architecture

```
┌─────────────────────────────────────────────────────┐
│  Kubernetes control plane (runs Crossplane)          │
│                                                      │
│  ┌──────────────┐       ┌──────────────────────────┐ │
│  │  Cluster CR  │──────▶│  provider-kind pod        │ │
│  │  (desired)   │       │  (reconciler)             │ │
│  └──────────────┘       │                           │ │
│                         │  sigs.k8s.io/kind library │ │
│                         └───────────┬───────────────┘ │
└─────────────────────────────────────┼───────────────┘
                                      │ /var/run/docker.sock
                         ┌────────────▼──────────────┐
                         │  Host Docker daemon        │
                         │                           │
                         │  ┌─────────────────────┐  │
                         │  │  KIND cluster        │  │
                         │  │  (Docker containers) │  │
                         │  └─────────────────────┘  │
                         └───────────────────────────┘
```

### How reconciliation works

1. **Observe** — the reconciler calls `provider.List()` to check whether a KIND
   cluster with the expected name already exists on the Docker daemon.
2. **Create** — if the cluster does not exist, the reconciler calls
   `provider.Create()` with a `v1alpha4.Cluster` config built from the managed
   resource spec. KIND creates the necessary Docker containers, runs kubeadm
   inside them, and waits until the cluster's API server is reachable.
3. **Observe (after create)** — the reconciler reads node status from
   `provider.ListNodes()` and publishes the kubeconfig as a connection secret.
4. **Delete** — when the managed resource is deleted, the reconciler calls
   `provider.Delete()` which removes all Docker containers belonging to the
   cluster.

### Why native Go (not upjet/Terraform)

The [upjet](https://github.com/crossplane/upjet) framework generates providers
from Terraform providers. There is no Terraform provider for KIND because KIND
has no API server of its own — it is purely a local CLI tool backed by the
Docker daemon. A native Go provider using the
[`sigs.k8s.io/kind`](https://pkg.go.dev/sigs.k8s.io/kind) library is the
natural fit: no Terraform state files, no provider binaries to download at
runtime, and a much smaller attack surface.

### Kubeconfig isolation

When KIND creates a cluster it normally writes an entry into `~/.kube/config`
and switches the current context. In a Crossplane provider this would corrupt
the provider pod's own in-cluster kubeconfig or, on Docker Desktop (macOS),
the host kubeconfig via the shared `/Users` volume. To prevent this, the
provider passes `os.DevNull` as the kubeconfig path on every `Create` and
`Delete` call. The kubeconfig for the managed cluster is retrieved separately
via `provider.KubeConfig()`, which reads it directly from the cluster's API
server configuration without touching any file on disk.

### Provider image

The provider container image is based on `docker:27-cli` (a minimal Alpine
image that includes only the Docker CLI binary). The KIND Go library shells out
to the `docker` binary for several operations (container exec, log streaming,
image loading). A distroless or scratch base image would cause those calls to
fail with `exec: "docker": executable file not found in $PATH`.

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Kubernetes cluster | Any distribution; KIND itself works well |
| [Crossplane](https://docs.crossplane.io/latest/software/install/) v2+ | Install with Helm |
| Docker daemon on each node running the provider pod | The provider communicates with Docker directly, not via the Kubernetes CRI |
| Docker socket at `/var/run/docker.sock` | Standard path; see [runtime config](#configuration) if yours differs |

> **Note for KIND-on-KIND (nested KIND):** if your Crossplane control plane
> itself runs in a KIND cluster you must configure that cluster with an
> `extraMounts` entry that bind-mounts the host Docker socket into the node
> container. See `cluster/test/kind-config.yaml` for a working example.

---

## Installation

### 1. Apply the runtime config

The `DeploymentRuntimeConfig` tells Crossplane how to configure the provider
pod. It must exist before the provider is installed so Crossplane can apply it
when the pod is first scheduled:

```bash
kubectl apply -f examples/runtime-config.yaml
```

This mounts `/var/run/docker.sock` from the host into the provider pod and
grants the pod root-level access (required because the Docker socket is owned
by `root:root` with mode `660`).

If your Docker socket is at a different path (common on some Linux
distributions where it lives at `/run/docker.sock`), edit the `hostPath.path`
field in `examples/runtime-config.yaml` before applying.

### 2. Install the provider

```bash
kubectl apply -f examples/install.yaml
```

Or inline:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-kind
spec:
  package: xpkg.upbound.io/humoflife/provider-kind:v0.1.0
  runtimeConfigRef:
    apiVersion: pkg.crossplane.io/v1beta1
    kind: DeploymentRuntimeConfig
    name: provider-kind
```

Wait for the provider to become healthy:

```bash
kubectl wait provider/provider-kind --for=condition=Healthy --timeout=5m
```

### 3. Create a ProviderConfig

The KIND provider requires no credentials — it uses the Docker daemon directly.

```bash
kubectl apply -f examples/providerconfig/providerconfig.yaml
```

---

## Configuration

### ProviderConfig

```yaml
apiVersion: kind.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec: {}
```

No secrets or credentials are needed. The provider connects to whichever Docker
daemon is reachable via the mounted socket.

### DeploymentRuntimeConfig

The runtime config ships in `examples/runtime-config.yaml`. The only setting
that typically needs adjustment is the Docker socket path:

```yaml
spec:
  deploymentTemplate:
    spec:
      template:
        spec:
          volumes:
            - name: docker-sock
              hostPath:
                path: /var/run/docker.sock  # adjust if needed
                type: Socket
```

---

## Usage

### Create a cluster (cluster-scoped)

```yaml
apiVersion: kind.crossplane.io/v1alpha1
kind: Cluster
metadata:
  name: my-cluster
spec:
  providerConfigRef:
    name: default
  forProvider:
    waitForReady: "5m"
    nodes:
      - role: control-plane
  writeConnectionSecretToRef:
    name: my-cluster-kubeconfig
    namespace: crossplane-system
```

```bash
kubectl apply -f examples/cluster/simple-cluster.yaml
kubectl get cluster my-cluster -o wide -w
```

### Create a cluster (namespaced)

```yaml
apiVersion: kind.m.crossplane.io/v1alpha1
kind: Cluster
metadata:
  name: my-cluster
  namespace: my-team
spec:
  providerConfigRef:
    kind: ProviderConfig
    name: default
  forProvider:
    waitForReady: "5m"
    nodes:
      - role: control-plane
  writeConnectionSecretToRef:
    name: my-cluster-kubeconfig
```

### Access the cluster kubeconfig

After the cluster becomes `Ready=True`, the kubeconfig is available as a
connection secret:

```bash
# Cluster-scoped resource
kubectl get secret my-cluster-kubeconfig \
  -n crossplane-system \
  -o jsonpath='{.data.kubeconfig}' | base64 -d > /tmp/my-cluster.kubeconfig

# Namespaced resource (secret lands in the same namespace as the Cluster)
kubectl get secret my-cluster-kubeconfig \
  -n my-team \
  -o jsonpath='{.data.kubeconfig}' | base64 -d > /tmp/my-cluster.kubeconfig

kubectl --kubeconfig /tmp/my-cluster.kubeconfig get nodes
```

### Delete a cluster

```bash
kubectl delete cluster my-cluster
```

Crossplane calls the provider's Delete function which removes all Docker
containers belonging to the KIND cluster.

---

## Examples

| File | Description |
|---|---|
| `examples/runtime-config.yaml` | DeploymentRuntimeConfig (Docker socket + root) |
| `examples/install.yaml` | Provider installation |
| `examples/providerconfig/providerconfig.yaml` | Default ProviderConfig (no credentials) |
| `examples/cluster/simple-cluster.yaml` | Single control-plane node (cluster-scoped) |
| `examples/cluster/ha-cluster.yaml` | 3 control-plane + 2 worker nodes |
| `examples/cluster/port-mapped-cluster.yaml` | Control-plane with ingress port mappings |
| `examples/namespacedcluster/simple-cluster.yaml` | Namespaced Cluster with 1 control-plane + 2 workers |

### HA cluster

```yaml
apiVersion: kind.crossplane.io/v1alpha1
kind: Cluster
metadata:
  name: ha-cluster
spec:
  providerConfigRef:
    name: default
  forProvider:
    waitForReady: "10m"
    nodes:
      - role: control-plane
      - role: control-plane
      - role: control-plane
      - role: worker
      - role: worker
    networking:
      apiServerAddress: "127.0.0.1"
      podSubnet: "10.244.0.0/16"
      serviceSubnet: "10.96.0.0/16"
  writeConnectionSecretToRef:
    name: ha-cluster-kubeconfig
    namespace: crossplane-system
```

### Cluster with port mappings

Useful when you want to reach services inside the KIND cluster from the host:

```yaml
forProvider:
  nodes:
    - role: control-plane
      extraPortMappings:
        - containerPort: 30080
          hostPort: 8080
          protocol: TCP
```

---

## API Reference

### ClusterParameters

| Field | Type | Required | Description |
|---|---|---|---|
| `nodes` | `[]Node` | No | Node topology. Defaults to a single control-plane node |
| `waitForReady` | `string` | No | Duration to wait for nodes to become ready (e.g. `"5m"`) |
| `networking` | `Networking` | No | Cluster networking configuration |
| `featureGates` | `map[string]bool` | No | Kubernetes feature gates |
| `runtimeConfig` | `map[string]string` | No | Runtime config key/value pairs |
| `kubeProxyMode` | `string` | No | kube-proxy mode (`iptables`, `ipvs`, `nftables`) |
| `containerdConfigPatches` | `[]string` | No | TOML patches for the containerd config |

### Node

| Field | Type | Required | Description |
|---|---|---|---|
| `role` | `string` | Yes | `control-plane` or `worker` |
| `image` | `string` | No | Node image override (e.g. `kindest/node:v1.31.0`) |
| `extraMounts` | `[]Mount` | No | Additional volume mounts into the node container |
| `extraPortMappings` | `[]PortMapping` | No | Host-to-container port mappings |
| `kubeadmConfigPatches` | `[]string` | No | YAML patches applied to the kubeadm config |
| `labels` | `map[string]string` | No | Labels applied to the node |

### Networking

| Field | Type | Description |
|---|---|---|
| `ipFamily` | `string` | `ipv4`, `ipv6`, or `dual` |
| `apiServerAddress` | `string` | Address the API server binds to |
| `apiServerPort` | `int32` | Port the API server listens on |
| `podSubnet` | `string` | CIDR for pod IPs |
| `serviceSubnet` | `string` | CIDR for service IPs |
| `disableDefaultCNI` | `bool` | Disable the default Kindnet CNI |
| `kubeProxyMode` | `string` | kube-proxy mode for this cluster |

### ClusterObservation (status.atProvider)

| Field | Type | Description |
|---|---|---|
| `apiServerEndpoint` | `string` | HTTPS endpoint of the managed cluster API server |
| `nodes` | `[]NodeObservation` | Observed state of each cluster node |
| `ready` | `bool` | True when all nodes report Running status |

---

## How to Contribute

Contributions are welcome. This section explains how to get a working
development environment, make changes, and submit them.

### Repository layout

```
provider-kind/
├── apis/                    # CRD Go type definitions and generated code
│   ├── cluster/v1alpha1/    # Cluster-scoped Cluster resource
│   ├── namespacedcluster/   # Namespaced Cluster resource
│   └── v1beta1/             # ProviderConfig types
├── cmd/provider/            # Provider binary entry point
├── internal/controller/     # Reconciler implementations
│   ├── cluster/             # Cluster-scoped controller
│   ├── namespacedcluster/   # Namespaced controller
│   └── providerconfig/      # ProviderConfig controller
├── package/                 # Crossplane package metadata + CRDs
│   ├── crossplane.yaml      # Provider metadata
│   └── crds/                # Generated CRD YAML files
├── examples/                # Example manifests for users
├── cluster/
│   ├── images/provider-kind/ # Dockerfile
│   └── test/                # E2E test setup (kind-config.yaml, setup.sh)
└── build/                   # Build system (git submodule)
```

### Set up the development environment

**Prerequisites:**

- Go 1.24+
- Docker Desktop (macOS) or Docker Engine (Linux)
- KIND v0.25+
- `kubectl`
- `helm`

**Clone and initialise submodules:**

```bash
git clone https://github.com/humoflife/provider-kind.git
cd provider-kind
git submodule update --init --recursive
```

### Run end-to-end tests

The `make e2e` target builds the provider, creates a local KIND control-plane
cluster, installs Crossplane, deploys the provider, and runs the full uptest
suite:

```bash
make e2e
```

To clean up the test cluster afterwards:

```bash
make controlplane.down
```

### Develop locally (out-of-cluster)

Run the provider binary directly against an existing cluster. This is the
fastest iteration loop for controller logic changes:

```bash
# Start a cluster and install Crossplane
kind create cluster --name provider-kind-dev
helm install crossplane crossplane-stable/crossplane \
  --namespace crossplane-system --create-namespace --wait

# Install CRDs
kubectl apply -f package/crds/

# Build and run (the provider uses your host Docker daemon)
go build -o _output/provider ./cmd/provider
./_output/provider --debug

# In another terminal, apply examples
kubectl apply -f examples/providerconfig/providerconfig.yaml
kubectl apply -f examples/cluster/simple-cluster.yaml
kubectl get cluster -o wide -w
```

### Modify API types

After changing any file under `apis/`, regenerate the deepcopy methods and CRD
manifests:

```bash
go generate ./apis/...
```

Always run `gofmt` after editing Go files:

```bash
gofmt -w <file>.go
```

### Run linters

```bash
make lint
```

### Submitting a pull request

1. Fork the repository and create a feature branch.
2. Make your changes. Keep commits focused and atomic.
3. Ensure `make e2e` passes on your local machine before opening a PR.
4. Open a pull request against `main`. Fill in the PR template.
5. A maintainer will review your changes and run CI.

### Reporting bugs

Open an [issue](https://github.com/humoflife/provider-kind/issues) and include:

- The provider version (`kubectl get provider provider-kind`)
- The Crossplane version (`kubectl get deployment crossplane -n crossplane-system`)
- The relevant managed resource YAML
- `kubectl describe` output for the failing resource
- Provider pod logs: `kubectl logs -n crossplane-system deployment/provider-kind-*`
