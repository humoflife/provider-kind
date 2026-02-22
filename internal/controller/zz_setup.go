// Copyright 2024 The provider-kind authors.
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/humoflife/provider-kind/internal/controller/cluster"
	"github.com/humoflife/provider-kind/internal/controller/namespacedcluster"
	"github.com/humoflife/provider-kind/internal/controller/providerconfig"
)

// Setup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	for _, setup := range []func(ctrl.Manager, xpcontroller.Options) error{
		cluster.Setup,
		namespacedcluster.Setup,
		providerconfig.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
