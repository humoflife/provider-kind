/*
Copyright 2024 The provider-kind authors.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	clusterv1alpha1 "github.com/humoflife/provider-kind/apis/cluster/v1alpha1"
)

// NamespacedClusterSpec defines the desired state of a namespaced KIND cluster.
// It uses the Crossplane v2 ModernManaged pattern with local connection secrets
// and typed provider config references.
type NamespacedClusterSpec struct {
	// ManagementPolicies specify the array of actions Crossplane is allowed to
	// take on the managed and external resources.
	// +optional
	// +kubebuilder:default={"*"}
	ManagementPolicies xpv1.ManagementPolicies `json:"managementPolicies,omitempty"`

	// ProviderConfigReference specifies how the provider that will be used to
	// create, observe, update, and delete this managed resource should be
	// configured. The Kind field must be set to "ProviderConfig".
	// +kubebuilder:default={"kind": "ProviderConfig", "name": "default"}
	ProviderConfigReference *xpv1.ProviderConfigReference `json:"providerConfigRef,omitempty"`

	// WriteConnectionSecretToReference specifies the name of a Secret, in the
	// same namespace as this managed resource, to which any connection details
	// for this managed resource should be written.
	// +optional
	WriteConnectionSecretToReference *xpv1.LocalSecretReference `json:"writeConnectionSecretToRef,omitempty"`

	ForProvider clusterv1alpha1.ClusterParameters `json:"forProvider"`
}

// NamespacedClusterStatus defines the observed state of a namespaced Cluster.
type NamespacedClusterStatus struct {
	xpv1.ConditionedStatus `json:",inline"`
	AtProvider             clusterv1alpha1.ClusterObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,kind}
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Cluster is the Schema for the namespaced KIND clusters API (Crossplane v2).
// A Cluster represents a KIND (Kubernetes IN Docker) cluster managed by
// the provider-kind Crossplane provider. This is a namespaced resource
// following the Crossplane v2 ModernManaged pattern.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NamespacedClusterSpec   `json:"spec"`
	Status NamespacedClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList contains a list of namespaced Cluster.
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}
