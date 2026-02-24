# ====================================================================================
# Setup Project

PROJECT_NAME ?= provider-kind
PROJECT_REPO ?= github.com/humoflife/$(PROJECT_NAME)


PLATFORMS ?= linux_amd64 linux_arm64

# -include will silently skip missing files, which allows us
# to load those files with a target in the Makefile. If only
# "include" was used, the make command would fail and refuse
# to run a target until the include commands succeeded.
-include build/makelib/common.mk

# ====================================================================================
# Setup Output

-include build/makelib/output.mk

# ====================================================================================
# Setup Go

# Set a sane default so that the nprocs calculation below is less noisy on the initial
# loading of this file
NPROCS ?= 1

# each of our test suites starts a kube-apiserver and running many test suites in
# parallel can lead to high CPU utilization. by default we reduce the parallelism
# to half the number of CPU cores.
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))

GO_REQUIRED_VERSION ?= 1.24.13
GOLANGCILINT_VERSION ?= 1.64.8
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += cmd internal apis
-include build/makelib/golang.mk

# ====================================================================================
# Setup Kubernetes tools

KIND_VERSION = v0.25.0
UP_VERSION = v0.44.3
UP_CHANNEL = stable
UPTEST_VERSION = v2.2.0
-include build/makelib/k8s_tools.mk

# uptest moved from upbound/uptest to crossplane/uptest after v0.12.0.
# The build submodule still points to the old org. We use a phony ensure.uptest
# target rather than redefining the $(UPTEST) file target to avoid the
# "overriding commands" warning from GNU Make.
.PHONY: ensure.uptest
ensure.uptest:
	@if [ ! -f "$(UPTEST)" ]; then \
		$(INFO) installing uptest $(UPTEST_VERSION); \
		mkdir -p $(TOOLS_HOST_DIR); \
		curl -fsSLo $(UPTEST) \
			https://github.com/crossplane/uptest/releases/download/$(UPTEST_VERSION)/uptest_$(SAFEHOSTPLATFORM) || $(FAIL); \
		chmod +x $(UPTEST); \
		$(OK) installing uptest $(UPTEST_VERSION); \
	fi

# ====================================================================================
# Setup Images

REGISTRY_ORGS ?= xpkg.upbound.io/humoflife
IMAGES = $(PROJECT_NAME)
-include build/makelib/imagelight.mk

# ====================================================================================
# Setup XPKG

XPKG_REG_ORGS ?= xpkg.upbound.io/humoflife
# NOTE(hasheddan): skip promoting on xpkg.upbound.io as channel tags are
# inferred.
XPKG_REG_ORGS_NO_PROMOTE ?= xpkg.upbound.io/humoflife
XPKGS = $(PROJECT_NAME)
-include build/makelib/xpkg.mk

# ====================================================================================
# Fallthrough

# run `make help` to see the targets and options

# We want submodules to be set up the first time `make` is run.
# We manage the build/ folder and its Makefiles as a submodule.
# The first time `make` is run, the includes of build/*.mk files will
# all fail, and this target will be run. The next time, the default as defined
# by the includes will be run instead.
fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# NOTE(hasheddan): we force image building to happen prior to xpkg build so that
# we ensure image is present in daemon.
xpkg.build.provider-kind: do.build.images

# NOTE(hasheddan): we ensure up is installed prior to running platform-specific
# build steps in parallel to avoid encountering an installation race condition.
build.init: $(UP)
# ====================================================================================
# Targets

# NOTE: the build submodule currently overrides XDG_CACHE_HOME in order to
# force the Helm 3 to use the .work/helm directory. This causes Go on Linux
# machines to use that directory as the build cache as well. We should adjust
# this behavior in the build submodule because it is also causing Linux users
# to duplicate their build cache, but for now we just make it easier to identify
# its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_ | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# This is for running out-of-cluster locally, and is for convenience. Running
# this make target will print out the command which was used. For more control,
# try running the binary directly with different arguments.
run: go.build
	@$(INFO) Running Crossplane locally out-of-cluster . . .
	@# To see other arguments that can be provided, run the command with --help instead
	UPBOUND_CONTEXT="local" $(GO_OUT_DIR)/provider --debug

# ====================================================================================
# End to End Testing
CROSSPLANE_VERSION = 2.2.0
# Use the default Crossplane namespace (crossplane-system) so that connection
# secrets and other core resources land in the standard location.
CROSSPLANE_NAMESPACE = crossplane-system
-include build/makelib/local.xpkg.mk
-include build/makelib/controlplane.mk

# provider-kind calls the Docker CLI (via the KIND Go library) for all cluster
# lifecycle operations, so the KIND node must have access to the host Docker socket.
# We add kind.cluster.ensure as an extra prerequisite to controlplane.up without
# redefining its recipe, which avoids the "overriding commands" warning from GNU Make.
# controlplane.mk's recipe uses "|| create" so it skips creation when the cluster exists.
KIND_CLUSTER_CONFIG ?= cluster/test/kind-config.yaml
controlplane.up: kind.cluster.ensure

.PHONY: kind.cluster.ensure
kind.cluster.ensure: $(KIND)
	@$(KIND) get kubeconfig --name $(KIND_CLUSTER_NAME) >/dev/null 2>&1 || \
		$(KIND) create cluster --name=$(KIND_CLUSTER_NAME) --config=$(KIND_CLUSTER_CONFIG)

# Comma-separated list of example manifests to run through the e2e suite.
# Covers ProviderConfig, cluster-scoped Cluster (LegacyManaged), and
# namespaced Cluster (ModernManaged). Override on the CLI to test a subset.
# See https://github.com/crossplane/uptest for full documentation.
# ProviderConfig is created by setup.sh; the e2e suite tests the managed resources only.
UPTEST_EXAMPLE_LIST ?= examples/cluster/simple-cluster.yaml,examples/namespacedcluster/simple-cluster.yaml

uptest: ensure.uptest $(KUBECTL) $(CHAINSAW)
	@$(INFO) running automated tests
	@KUBECTL=$(KUBECTL) CHAINSAW=$(CHAINSAW) CROSSPLANE_NAMESPACE=$(CROSSPLANE_NAMESPACE) \
		$(UPTEST) e2e "${UPTEST_EXAMPLE_LIST}" --setup-script=cluster/test/setup.sh \
		--default-timeout=600s || $(FAIL)
	@$(OK) running automated tests

local-deploy: build controlplane.up local.xpkg.deploy.provider.$(PROJECT_NAME)
	@$(INFO) running locally built provider
	@$(INFO) patching provider runtime config to mount Docker socket and run as root
	@$(KUBECTL) patch deploymentruntimeconfig runtimeconfig-provider-kind \
		--type=json \
		-p='[{"op":"add","path":"/spec/deploymentTemplate/spec/template/spec/volumes","value":[{"name":"docker-sock","hostPath":{"path":"/var/run/docker.sock","type":"Socket"}}]},{"op":"add","path":"/spec/deploymentTemplate/spec/template/spec/containers/0/volumeMounts","value":[{"name":"docker-sock","mountPath":"/var/run/docker.sock"}]},{"op":"add","path":"/spec/deploymentTemplate/spec/template/spec/containers/0/securityContext","value":{"runAsUser":0,"runAsNonRoot":false}}]'
	@$(KUBECTL) wait provider.pkg $(PROJECT_NAME) --for condition=Healthy --timeout 5m
	@$(OK) running locally built provider

e2e: local-deploy uptest

crddiff: ensure.uptest
	@$(INFO) Checking breaking CRD schema changes
	@for crd in $${MODIFIED_CRD_LIST}; do \
		if ! git cat-file -e "$${GITHUB_BASE_REF}:$${crd}" 2>/dev/null; then \
			echo "CRD $${crd} does not exist in the $${GITHUB_BASE_REF} branch. Skipping..." ; \
			continue ; \
		fi ; \
		echo "Checking $${crd} for breaking API changes..." ; \
		changes_detected=$$($(UPTEST) crddiff revision <(git cat-file -p "$${GITHUB_BASE_REF}:$${crd}") "$${crd}" 2>&1) ; \
		if [[ $$? != 0 ]] ; then \
			printf "\033[31m"; echo "Breaking change detected!"; printf "\033[0m" ; \
			echo "$${changes_detected}" ; \
			echo ; \
		fi ; \
	done
	@$(OK) Checking breaking CRD schema changes

.PHONY: cobertura submodules fallthrough run crds.clean

# ====================================================================================
# Special Targets

define CROSSPLANE_MAKE_HELP
Crossplane Targets:
    cobertura             Generate a coverage report for cobertura applying exclusions on generated files.
    submodules            Update the submodules, such as the common build scripts.
    run                   Run crossplane locally, out-of-cluster. Useful for development.

endef
# The reason CROSSPLANE_MAKE_HELP is used instead of CROSSPLANE_HELP is because the crossplane
# binary will try to use CROSSPLANE_HELP if it is set, and this is for something different.
export CROSSPLANE_MAKE_HELP

crossplane.help:
	@echo "$$CROSSPLANE_MAKE_HELP"

help-special: crossplane.help

.PHONY: crossplane.help help-special

# TODO(negz): Update CI to use these targets.
vendor: modules.download
vendor.check: modules.check
