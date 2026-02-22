# docker:cli provides the Docker CLI binary (no daemon) on a minimal Alpine base.
# provider-kind uses the KIND Go library which calls `docker` CLI commands
# for all cluster lifecycle operations (list, create, delete, exec, etc.),
# so the docker binary must be present in the provider's PATH.
FROM docker:27-cli

ARG TARGETOS
ARG TARGETARCH

ADD bin/${TARGETOS}_${TARGETARCH}/provider /usr/local/bin/provider

ENTRYPOINT ["provider"]
