/*
Copyright 2024 The provider-kind authors.
*/

// Package cluster implements the Crossplane managed reconciler for KIND clusters.
package cluster

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	kindcluster "sigs.k8s.io/kind/pkg/cluster"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	clusterv1alpha1 "github.com/humoflife/provider-kind/apis/cluster/v1alpha1"
	"github.com/humoflife/provider-kind/apis/v1beta1"
)

const (
	errNotCluster    = "managed resource is not a Cluster custom resource"
	errTrackUsage    = "cannot track ProviderConfig usage"
	errListClusters  = "cannot list KIND clusters"
	errCreateCluster = "cannot create KIND cluster"
	errDeleteCluster = "cannot delete KIND cluster"
	errGetKubeConfig = "cannot get kubeconfig for KIND cluster"
	errGetNodes      = "cannot list KIND cluster nodes"
	errParseWait     = "cannot parse waitForReady duration"
)

// Setup adds a controller that reconciles Cluster managed resources.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	name := managed.ControllerName(clusterv1alpha1.ClusterGroupVersionKind.String())

	reconcilerOpts := []managed.ReconcilerOption{
		managed.WithExternalConnecter(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithPollInterval(o.PollInterval),
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(clusterv1alpha1.ClusterGroupVersionKind),
		reconcilerOpts...,
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&clusterv1alpha1.Cluster{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// connector creates a KIND cluster provider for each reconcile.
type connector struct {
	kube client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*clusterv1alpha1.Cluster)
	if !ok {
		return nil, errors.New(errNotCluster)
	}

	// Track ProviderConfig usage so ProviderConfig deletion is blocked
	// while resources reference it. Use the legacy tracker for the
	// cluster-scoped Cluster resource (LegacyManaged).
	tracker := resource.NewLegacyProviderConfigUsageTracker(c.kube, &v1beta1.ProviderConfigUsage{})
	if err := tracker.Track(ctx, cr); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}

	// Create KIND provider using the local Docker daemon.
	// No credentials are required for local KIND clusters.
	provider := kindcluster.NewProvider()

	return &external{provider: provider}, nil
}

// external implements managed.ExternalClient for KIND clusters.
type external struct {
	provider *kindcluster.Provider
}

// Observe checks whether the KIND cluster already exists and observes its state.
func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*clusterv1alpha1.Cluster)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotCluster)
	}

	clusterName := getClusterName(cr)

	// List all local KIND clusters to check if ours exists.
	clusters, err := e.provider.List()
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errListClusters)
	}

	exists := false
	for _, c := range clusters {
		if c == clusterName {
			exists = true
			break
		}
	}

	if !exists {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// Cluster exists - get the kubeconfig.
	kubeconfig, err := e.provider.KubeConfig(clusterName, false)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetKubeConfig)
	}

	// Parse the API server endpoint from the kubeconfig.
	if kubeconf, parseErr := clientcmd.Load([]byte(kubeconfig)); parseErr == nil {
		for _, clusterInfo := range kubeconf.Clusters {
			endpoint := clusterInfo.Server
			cr.Status.AtProvider.APIServerEndpoint = &endpoint
			break
		}
	}

	// Observe node states.
	nodes, err := e.provider.ListNodes(clusterName)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetNodes)
	}

	nodeObs := make([]clusterv1alpha1.NodeObservation, 0, len(nodes))
	allReady := len(nodes) > 0

	for _, n := range nodes {
		role, roleErr := n.Role()
		ipv4, ipv6, ipErr := n.IP()

		obs := clusterv1alpha1.NodeObservation{
			Name:        n.String(),
			Role:        role,
			IPAddress:   ipv4,
			IPv6Address: ipv6,
			Image:       nodeImage(ctx, n.String()),
		}

		// If we can get the node's role and IP, it's running.
		if roleErr == nil && ipErr == nil {
			obs.Status = "Running"
		} else {
			obs.Status = "Unknown"
			allReady = false
		}

		nodeObs = append(nodeObs, obs)
	}

	cr.Status.AtProvider.Nodes = nodeObs
	cr.Status.AtProvider.Ready = allReady

	if allReady {
		cr.SetConditions(xpv1.Available())
	} else {
		cr.SetConditions(xpv1.Unavailable())
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
		ConnectionDetails: managed.ConnectionDetails{
			"kubeconfig": []byte(kubeconfig),
		},
	}, nil
}

// Create provisions a new KIND cluster.
func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*clusterv1alpha1.Cluster)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotCluster)
	}

	cr.SetConditions(xpv1.Creating())

	clusterName := getClusterName(cr)
	// Set the external name so Observe can find the cluster later.
	meta.SetExternalName(cr, clusterName)

	// Build the KIND cluster configuration from the spec.
	kindConfig := buildKindConfig(cr.Spec.ForProvider)

	opts := []kindcluster.CreateOption{
		kindcluster.CreateWithV1Alpha4Config(kindConfig),
		// Write the kubeconfig to /dev/null to prevent KIND from modifying the
		// default ~/.kube/config and changing the kubectl current-context on the
		// host. The kubeconfig is retrieved separately via provider.KubeConfig().
		kindcluster.CreateWithKubeconfigPath(os.DevNull),
	}

	// If waitForReady is specified, wait for nodes to become ready.
	if cr.Spec.ForProvider.WaitForReady != nil {
		wait, err := time.ParseDuration(*cr.Spec.ForProvider.WaitForReady)
		if err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, errParseWait)
		}
		opts = append(opts, kindcluster.CreateWithWaitForReady(wait))
	}

	if err := e.provider.Create(clusterName, opts...); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateCluster)
	}

	// Retrieve the kubeconfig immediately after creation.
	kubeconfig, err := e.provider.KubeConfig(clusterName, false)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errGetKubeConfig)
	}

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{
			"kubeconfig": []byte(kubeconfig),
		},
	}, nil
}

// Disconnect is a no-op because the KIND provider uses the local Docker daemon
// and holds no persistent connection that needs to be cleaned up.
func (e *external) Disconnect(_ context.Context) error {
	return nil
}

// Update is a no-op because KIND clusters are largely immutable after creation.
// Node count and networking changes require deleting and recreating the cluster.
func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, nil
}

// Delete removes the KIND cluster.
func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*clusterv1alpha1.Cluster)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotCluster)
	}

	cr.SetConditions(xpv1.Deleting())

	clusterName := getClusterName(cr)

	// Pass os.DevNull so KIND does not attempt to remove the cluster entry from
	// the default ~/.kube/config (which would also not exist there anyway since
	// Create wrote to /dev/null).
	if err := e.provider.Delete(clusterName, os.DevNull); err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteCluster)
	}

	return managed.ExternalDelete{}, nil
}

// nodeImage returns the container image for a KIND node by inspecting the
// Docker container. Returns an empty string if the image cannot be determined.
func nodeImage(ctx context.Context, containerName string) string {
	out, err := exec.CommandContext(ctx, "docker", "inspect",
		"--format={{.Config.Image}}", containerName).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getClusterName returns the external name of the cluster, falling back to
// the managed resource name if no external name has been set.
func getClusterName(cr *clusterv1alpha1.Cluster) string {
	if name := meta.GetExternalName(cr); name != "" {
		return name
	}
	return cr.GetName()
}

// buildKindConfig converts the ClusterParameters from the CRD spec into a KIND
// v1alpha4 cluster configuration.
func buildKindConfig(params clusterv1alpha1.ClusterParameters) *v1alpha4.Cluster {
	cfg := &v1alpha4.Cluster{
		TypeMeta: v1alpha4.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "kind.x-k8s.io/v1alpha4",
		},
	}

	// Convert node definitions.
	for _, node := range params.Nodes {
		n := v1alpha4.Node{
			Role: v1alpha4.NodeRole(node.Role),
		}

		if node.Image != nil {
			n.Image = *node.Image
		}

		for _, m := range node.ExtraMounts {
			mount := v1alpha4.Mount{
				HostPath:      m.HostPath,
				ContainerPath: m.ContainerPath,
			}
			if m.Readonly != nil {
				mount.Readonly = *m.Readonly
			}
			if m.SelinuxRelabel != nil {
				mount.SelinuxRelabel = *m.SelinuxRelabel
			}
			if m.Propagation != nil {
				mount.Propagation = v1alpha4.MountPropagation(*m.Propagation)
			}
			n.ExtraMounts = append(n.ExtraMounts, mount)
		}

		for _, pm := range node.ExtraPortMappings {
			portMap := v1alpha4.PortMapping{
				ContainerPort: pm.ContainerPort,
				HostPort:      pm.HostPort,
			}
			if pm.ListenAddress != nil {
				portMap.ListenAddress = *pm.ListenAddress
			}
			if pm.Protocol != nil {
				portMap.Protocol = v1alpha4.PortMappingProtocol(*pm.Protocol)
			}
			n.ExtraPortMappings = append(n.ExtraPortMappings, portMap)
		}

		n.KubeadmConfigPatches = node.KubeadmConfigPatches
		n.Labels = node.Labels
		cfg.Nodes = append(cfg.Nodes, n)
	}

	// Convert networking configuration.
	if params.Networking != nil {
		net := params.Networking
		if net.IPFamily != nil {
			cfg.Networking.IPFamily = v1alpha4.ClusterIPFamily(*net.IPFamily)
		}
		if net.APIServerAddress != nil {
			cfg.Networking.APIServerAddress = *net.APIServerAddress
		}
		if net.APIServerPort != nil {
			cfg.Networking.APIServerPort = *net.APIServerPort
		}
		if net.PodSubnet != nil {
			cfg.Networking.PodSubnet = *net.PodSubnet
		}
		if net.ServiceSubnet != nil {
			cfg.Networking.ServiceSubnet = *net.ServiceSubnet
		}
		if net.DisableDefaultCNI != nil {
			cfg.Networking.DisableDefaultCNI = *net.DisableDefaultCNI
		}
		if net.KubeProxyMode != nil {
			cfg.Networking.KubeProxyMode = v1alpha4.ProxyMode(*net.KubeProxyMode)
		}
	}

	// Feature gates and runtime config.
	cfg.FeatureGates = params.FeatureGates
	cfg.RuntimeConfig = params.RuntimeConfig

	// Top-level kube-proxy mode: apply to networking if not already set there.
	if params.KubeProxyMode != nil && (params.Networking == nil || params.Networking.KubeProxyMode == nil) {
		cfg.Networking.KubeProxyMode = v1alpha4.ProxyMode(*params.KubeProxyMode)
	}

	// Containerd config patches.
	cfg.ContainerdConfigPatches = params.ContainerdConfigPatches

	return cfg
}
