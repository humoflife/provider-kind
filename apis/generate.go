//go:build generate
// +build generate

// Copyright 2024 The provider-kind authors.
//
// SPDX-License-Identifier: Apache-2.0

// Remove existing CRDs
//go:generate rm -rf ../package/crds

// Remove generated zz_ deepcopy files
//go:generate bash -c "find . -iname 'zz_generated.deepcopy.go' -delete"

// Generate deepcopy methodsets and CRD manifests
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile=../hack/boilerplate.go.txt paths=./... crd:allowDangerousTypes=true,crdVersions=v1 output:artifacts:config=../package/crds

// Generate crossplane-runtime methodsets (resource.Managed, etc)
//go:generate go run -tags generate github.com/crossplane/crossplane-tools/cmd/angryjet generate-methodsets --header-file=../hack/boilerplate.go.txt ./...

package apis
