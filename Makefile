CMDS=cloudstack-csi-driver cloudstack-csi-sc-syncer

PKG=github.com/leaseweb/cloudstack-csi-driver
# Revision that gets built into each binary via the main.version
# string. Uses the `git describe` output based on the most recent
# version tag with a short revision suffix or, if nothing has been
# tagged yet, just the revision.
#
# Beware that tags may also be missing in shallow clones as done by
# some CI systems (like TravisCI, which pulls only 50 commits).
REV=$(shell git describe --long --tags --match='v*' --dirty 2>/dev/null || git rev-list -n1 HEAD)
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u -Iseconds)

DOCKER?=docker

IMPORTPATH_LDFLAGS = -X ${PKG}/pkg/driver.driverVersion=$(REV) -X ${PKG}/pkg/driver.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver.buildDate=${BUILD_DATE}
LDFLAGS = -s -w
FULL_LDFLAGS = $(LDFLAGS) $(IMPORTPATH_LDFLAGS)

export REPO_ROOT := $(shell git rev-parse --show-toplevel)

# Directories
TOOLS_DIR := $(REPO_ROOT)/hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
BIN_DIR ?= bin

GO_INSTALL := ./hack/go_install.sh

MOCKGEN_BIN := mockgen
MOCKGEN_VER := v0.5.0
MOCKGEN := $(abspath $(TOOLS_BIN_DIR)/$(MOCKGEN_BIN)-$(MOCKGEN_VER))
MOCKGEN_PKG := go.uber.org/mock/mockgen

.PHONY: all
all: build

.PHONY: build
build: $(CMDS:%=build-%)

.PHONY: container
container: $(CMDS:%=container-%)

.PHONY: clean
clean:
	rm -rf bin test/e2e/e2e.test test/e2e/ginkgo

.PHONY: build-%
$(CMDS:%=build-%): build-%:
	mkdir -p bin 
	CGO_ENABLED=0 go build -ldflags '$(FULL_LDFLAGS)' -o "./bin/$*" ./cmd/$*

.PHONY: container-%
$(CMDS:%=container-%): container-%: build-%
	$(DOCKER) build -f ./cmd/$*/Dockerfile -t $*:latest \
		--label org.opencontainers.image.revision=$(REV) .

.PHONY: generate-mocks
generate-mocks: $(MOCKGEN) pkg/cloud/mock_cloud.go ## Generate mocks needed for testing. Primarily mocks of the cloud package.
pkg/cloud/mock%.go: $(shell find ./pkg/cloud -type f -name "*test*" -prune -o -print)
	go generate ./...

.PHONY: test
test:
	go test ./...

.PHONY: test-sanity
test-sanity:
	go test --tags=sanity ./test/sanity

.PHONY: setup-external-e2e
setup-external-e2e: test/e2e/e2e.test test/e2e/ginkgo

test/e2e/e2e.test test/e2e/ginkgo:
	curl --location https://dl.k8s.io/v1.30.5/kubernetes-test-linux-amd64.tar.gz | \
		tar --strip-components=3 -C test/e2e -zxf - kubernetes/test/bin/e2e.test kubernetes/test/bin/ginkgo 

.PHONY: test-e2e
test-e2e: setup-external-e2e
	bash ./test/e2e/run.sh

##@ hack/tools:

.PHONY: $(MOCKGEN_BIN)
$(MOCKGEN_BIN): $(MOCKGEN) ## Build a local copy of mockgen.

$(MOCKGEN): # Build mockgen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(MOCKGEN_PKG) $(MOCKGEN_BIN) $(MOCKGEN_VER)
