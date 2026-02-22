/*
Copyright 2024 The provider-kind authors.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpcommon "github.com/crossplane/crossplane-runtime/v2/apis/common"
)

// +kubebuilder:object:root=true

// A NamespacedProviderConfigUsage indicates that a namespaced resource is
// using a ProviderConfig. Used by the Crossplane v2 ModernManaged pattern.
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="CONFIG-NAME",type="string",JSONPath=".providerConfigRef.name"
// +kubebuilder:printcolumn:name="RESOURCE-KIND",type="string",JSONPath=".resourceRef.kind"
// +kubebuilder:printcolumn:name="RESOURCE-NAME",type="string",JSONPath=".resourceRef.name"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,provider,kind}
type NamespacedProviderConfigUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	xpcommon.TypedProviderConfigUsage `json:",inline"`
}

// +kubebuilder:object:root=true

// NamespacedProviderConfigUsageList contains a list of NamespacedProviderConfigUsage.
type NamespacedProviderConfigUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespacedProviderConfigUsage `json:"items"`
}
