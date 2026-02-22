/*
Copyright 2024 The provider-kind authors.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

// ClusterParameters defines the desired state of a KIND cluster.
type ClusterParameters struct {
	// Nodes defines the nodes in the cluster. If empty, a single
	// control-plane node is created.
	// +optional
	// +listType=atomic
	Nodes []Node `json:"nodes,omitempty"`

	// Networking defines cluster-wide networking configuration.
	// +optional
	Networking *Networking `json:"networking,omitempty"`

	// FeatureGates is a map of Kubernetes feature gate names to boolean
	// values, passed to the cluster via kubeadm.
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`

	// RuntimeConfig is passed to the API server as --runtime-config flags.
	// Values are mapped to the flag: --runtime-config=key=value.
	// +optional
	RuntimeConfig map[string]string `json:"runtimeConfig,omitempty"`

	// KubeProxyMode sets the kube-proxy mode for the cluster.
	// +optional
	// +kubebuilder:validation:Enum=iptables;ipvs;nftables;none
	KubeProxyMode *string `json:"kubeProxyMode,omitempty"`

	// ContainerdConfigPatches are toml-encoded patches to apply to all
	// node containerd configs. Only the last patch in the list that
	// applies to a given node will be used.
	// +optional
	ContainerdConfigPatches []string `json:"containerdConfigPatches,omitempty"`

	// WaitForReady is the duration to wait for the cluster to become
	// ready after creation (e.g. "5m", "30s"). Defaults to no wait.
	// +optional
	WaitForReady *string `json:"waitForReady,omitempty"`
}

// Node defines a KIND cluster node.
type Node struct {
	// Role is the node role in the cluster.
	// +kubebuilder:validation:Enum=control-plane;worker
	// +kubebuilder:default=control-plane
	Role string `json:"role"`

	// Image is the node container image to use. Defaults to the
	// KIND default node image for the current kind version.
	// +optional
	Image *string `json:"image,omitempty"`

	// ExtraMounts are additional directory or file mounts from the
	// host into the node container.
	// +optional
	ExtraMounts []Mount `json:"extraMounts,omitempty"`

	// ExtraPortMappings are additional port mappings from the node
	// container to the host machine.
	// +optional
	ExtraPortMappings []PortMapping `json:"extraPortMappings,omitempty"`

	// KubeadmConfigPatches are kubeadm config patches applied to nodes
	// during cluster creation. Patches are applied using strategic merge
	// or JSON merge, depending on the type of the patch.
	// +optional
	KubeadmConfigPatches []string `json:"kubeadmConfigPatches,omitempty"`

	// Labels are additional labels to apply to the node.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// Mount defines a bind mount from the host into a KIND node container.
type Mount struct {
	// HostPath is the absolute path on the host to mount.
	HostPath string `json:"hostPath"`

	// ContainerPath is the path inside the node container to mount to.
	ContainerPath string `json:"containerPath"`

	// Readonly makes the mount read-only inside the container.
	// +optional
	Readonly *bool `json:"readonly,omitempty"`

	// SelinuxRelabel enables SELinux relabeling on the mounted directory.
	// +optional
	SelinuxRelabel *bool `json:"selinuxRelabel,omitempty"`

	// Propagation sets the mount propagation mode.
	// +optional
	// +kubebuilder:validation:Enum=None;HostToContainer;Bidirectional
	Propagation *string `json:"propagation,omitempty"`
}

// PortMapping defines a port forwarding from the node container to the host.
type PortMapping struct {
	// ContainerPort is the port inside the node container.
	ContainerPort int32 `json:"containerPort"`

	// HostPort is the port on the host machine. Use 0 to let the OS
	// select a random available port.
	HostPort int32 `json:"hostPort"`

	// ListenAddress is the host IP address to bind the port on.
	// Defaults to 0.0.0.0 (all interfaces).
	// +optional
	ListenAddress *string `json:"listenAddress,omitempty"`

	// Protocol is the network protocol for the port mapping.
	// +optional
	// +kubebuilder:validation:Enum=TCP;UDP;SCTP
	Protocol *string `json:"protocol,omitempty"`
}

// Networking defines cluster-wide networking options for a KIND cluster.
type Networking struct {
	// IPFamily is the IP address family for the cluster.
	// +optional
	// +kubebuilder:validation:Enum=ipv4;ipv6;dual
	IPFamily *string `json:"ipFamily,omitempty"`

	// APIServerAddress is the IP address on the host to listen on for
	// the Kubernetes API server. Defaults to 127.0.0.1.
	// +optional
	APIServerAddress *string `json:"apiServerAddress,omitempty"`

	// APIServerPort is the port on the host to listen on for the
	// Kubernetes API server. Defaults to a random available port.
	// +optional
	APIServerPort *int32 `json:"apiServerPort,omitempty"`

	// PodSubnet is the CIDR block for pod networking.
	// Defaults to 10.244.0.0/16 for IPv4.
	// +optional
	PodSubnet *string `json:"podSubnet,omitempty"`

	// ServiceSubnet is the CIDR block for service networking.
	// Defaults to 10.96.0.0/16 for IPv4.
	// +optional
	ServiceSubnet *string `json:"serviceSubnet,omitempty"`

	// DisableDefaultCNI disables the default kindnetd CNI plugin so
	// that an alternative CNI (e.g. Calico, Cilium) can be installed.
	// +optional
	DisableDefaultCNI *bool `json:"disableDefaultCNI,omitempty"`

	// KubeProxyMode sets the kube-proxy mode for the cluster.
	// +optional
	// +kubebuilder:validation:Enum=iptables;ipvs;nftables;none
	KubeProxyMode *string `json:"kubeProxyMode,omitempty"`
}

// ClusterObservation is the observable state of a KIND cluster.
type ClusterObservation struct {
	// Ready indicates whether the cluster is ready and all nodes are running.
	Ready bool `json:"ready,omitempty"`

	// Nodes are the observed states of the cluster nodes.
	// +optional
	Nodes []NodeObservation `json:"nodes,omitempty"`

	// APIServerEndpoint is the address of the Kubernetes API server.
	// +optional
	APIServerEndpoint *string `json:"apiServerEndpoint,omitempty"`
}

// NodeObservation is the observed state of a KIND cluster node.
type NodeObservation struct {
	// Name is the Docker container name for this node.
	Name string `json:"name"`

	// Role is the node role (control-plane or worker).
	Role string `json:"role"`

	// Status is the Docker container status.
	Status string `json:"status"`

	// Image is the container image used for this node.
	Image string `json:"image,omitempty"`

	// IPAddress is the IPv4 address of the node container.
	IPAddress string `json:"ipAddress,omitempty"`

	// IPv6Address is the IPv6 address of the node container.
	// Only populated for IPv6 or dual-stack clusters.
	IPv6Address string `json:"ipv6Address,omitempty"`
}

// ClusterSpec defines the desired state of a Cluster.
type ClusterSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       ClusterParameters `json:"forProvider"`
}

// ClusterStatus defines the observed state of a Cluster.
type ClusterStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ClusterObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,kind}
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Cluster is the Schema for the KIND clusters API.
// A Cluster represents a KIND (Kubernetes IN Docker) cluster managed by
// the provider-kind Crossplane provider.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster.
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}
