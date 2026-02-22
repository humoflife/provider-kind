// Copyright 2024 The provider-kind authors.
//
// SPDX-License-Identifier: Apache-2.0

// Package apis contains Kubernetes API for the provider.
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	clusterv1alpha1 "github.com/humoflife/provider-kind/apis/cluster/v1alpha1"
	namespacedclusterv1alpha1 "github.com/humoflife/provider-kind/apis/namespacedcluster/v1alpha1"
	v1beta1 "github.com/humoflife/provider-kind/apis/v1beta1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		clusterv1alpha1.SchemeBuilder.AddToScheme,
		namespacedclusterv1alpha1.SchemeBuilder.AddToScheme,
		v1beta1.SchemeBuilder.AddToScheme,
	)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
