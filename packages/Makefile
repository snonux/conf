# Build and upload custom packages to the f3s package repository.
#
# Single-binary Go packages (pure Go, cross-compiled):
#   make pkg NAME=gogios SRC=/home/paul/git/gogios          # both OSes
#   make pkg-freebsd NAME=gogios SRC=/home/paul/git/gogios  # FreeBSD only
#   make pkg-openbsd NAME=gogios SRC=/home/paul/git/gogios  # OpenBSD only
#
# Multi-binary / CGo packages (built natively on OpenBSD build VM):
#   make dtail-openbsd                                       # DTail for OpenBSD
#   make dtail-rocky                                         # DTail RPM repo for Rocky Linux 9 (x86_64 + aarch64)
#
# Build VM management:
#   make buildvm-start   # boot the OpenBSD build VM
#   make buildvm-stop    # shut it down
#
# Required variables (single-binary):
#   NAME  — package name (e.g. gogios)
#   SRC   — path to the Go project root (must have cmd/$(NAME)/main.go
#           and internal/version.go with a Version constant)
#
# Optional variables (single-binary):
#   COMMENT     — one-line package description
#   DESC        — longer description (for OpenBSD desc file)
#   MAINTAINER  — maintainer email
#   WWW         — project URL
#   ENTRY       — Go main package path relative to SRC (default: cmd/$(NAME)/main.go)

SHELL := /bin/bash
.ONESHELL:

# SSH targets for production hosts
FREEBSD_HOST := f0.lan.buetow.org
FREEBSD_SSH  := ssh -p 22
FREEBSD_SCP  := scp -P 22
OPENBSD_HOST := rex@fishfinger.buetow.org
OPENBSD_SSH  := ssh
OPENBSD_SCP  := scp
ROCKY_ARM_BUILD_HOST := paul@pi0.lan.buetow.org
ROCKY_ARM_SSH := ssh -p 22
ROCKY_ARM_SCP := scp -P 22

# Local OpenBSD build VM (QEMU/KVM) for native compilation
BUILDVM_SSH  := ssh -o StrictHostKeyChecking=no -p 2222
BUILDVM_SCP  := scp -o StrictHostKeyChecking=no -P 2222
BUILDVM_HOST := pbuild@localhost
BUILDVM_DIR  := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))buildvm

# NFS-backed PV on f0
PV_BASE         := /data/nfs/k3svolumes/pkgrepo
FREEBSD_REPO    := freebsd/FreeBSD:15:amd64/latest
OPENBSD_VERSION := 7.8
OPENBSD_REPO    := openbsd/$(OPENBSD_VERSION)/packages/amd64
ROCKY_VERSION   := 9
ROCKY_REPO      := rockylinux/$(ROCKY_VERSION)
CONTAINER_ENGINE ?= podman

# Defaults for package metadata
COMMENT    ?= $(NAME)
DESC       ?= $(NAME)
MAINTAINER ?= paul@buetow.org
WWW        ?= https://buetow.org
ENTRY      ?= cmd/$(NAME)/main.go

SCRIPTS := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))scripts

# Extract version from internal/version.go, stripping the "v" prefix
VERSION = $(shell grep 'Version' $(SRC)/internal/version.go | sed 's/.*"\(.*\)"/\1/' | tr -d v)

# DTail settings
DTAIL_SRC      := /home/paul/git/dtail
DTAIL_VERSION   = $(shell grep 'Version string' $(DTAIL_SRC)/internal/version/version.go | sed 's/.*"\(.*\)"/\1/')
DTAIL_BINARIES := dserver dcat dgrep dmap dtail dtailhealth
CONF_FRONTENDS := $(abspath $(dir $(abspath $(lastword $(MAKEFILE_LIST))))../frontends)
DTAIL_RPM_FILES := $(abspath $(dir $(abspath $(lastword $(MAKEFILE_LIST))))files/dtail-rocky)

.PHONY: pkg pkg-freebsd pkg-openbsd dtail-openbsd dtail-rocky check-vars clean
.PHONY: buildvm-start buildvm-stop buildvm-ensure

# --- Build VM management ---

buildvm-start:
	@$(BUILDVM_DIR)/start.sh

buildvm-stop:
	@$(BUILDVM_DIR)/stop.sh

# Ensure the build VM is running before native builds
buildvm-ensure:
	@if ! $(BUILDVM_SSH) $(BUILDVM_HOST) true 2>/dev/null; then \
		echo "Build VM not running, starting..."; \
		$(BUILDVM_DIR)/start.sh; \
	fi

# --- Single-binary Go packages (cross-compiled on Linux) ---

check-vars:
ifndef NAME
	$(error NAME is required — e.g. make pkg NAME=gogios SRC=~/git/gogios)
endif
ifndef SRC
	$(error SRC is required — e.g. make pkg NAME=gogios SRC=~/git/gogios)
endif
	@test -f "$(SRC)/$(ENTRY)" || { echo "Error: $(SRC)/$(ENTRY) not found"; exit 1; }
	@test -n "$(VERSION)" || { echo "Error: could not extract version from $(SRC)/internal/version.go"; exit 1; }
	@echo "Package: $(NAME) $(VERSION)"

# Cross-compile for FreeBSD amd64
/tmp/$(NAME)-freebsd: check-vars
	@echo "Cross-compiling $(NAME) for FreeBSD/amd64..."
	cd $(SRC) && GOOS=freebsd GOARCH=amd64 go build -o $@ $(ENTRY)

# Cross-compile for OpenBSD amd64
/tmp/$(NAME)-openbsd: check-vars
	@echo "Cross-compiling $(NAME) for OpenBSD/amd64..."
	cd $(SRC) && GOOS=openbsd GOARCH=amd64 go build -o $@ $(ENTRY)

# Build, package, and upload the FreeBSD package
pkg-freebsd: /tmp/$(NAME)-freebsd
	@echo "Packaging $(NAME) $(VERSION) for FreeBSD..."
	$(FREEBSD_SCP) /tmp/$(NAME)-freebsd $(FREEBSD_HOST):/tmp/$(NAME)
	$(FREEBSD_SCP) $(SCRIPTS)/pkg-freebsd.sh $(FREEBSD_HOST):/tmp/pkg-freebsd.sh
	$(FREEBSD_SSH) $(FREEBSD_HOST) "/bin/sh /tmp/pkg-freebsd.sh \
		'$(NAME)' '$(VERSION)' '$(COMMENT)' '$(DESC)' '$(MAINTAINER)' '$(WWW)' \
		'$(PV_BASE)/$(FREEBSD_REPO)'"
	rm -f /tmp/$(NAME)-freebsd

# Build, package, sign, and upload the OpenBSD package
pkg-openbsd: /tmp/$(NAME)-openbsd
	@echo "Packaging $(NAME) $(VERSION) for OpenBSD..."
	$(OPENBSD_SCP) /tmp/$(NAME)-openbsd $(OPENBSD_HOST):/tmp/$(NAME)
	$(OPENBSD_SCP) $(SCRIPTS)/pkg-openbsd.sh $(OPENBSD_HOST):/tmp/pkg-openbsd.sh
	$(OPENBSD_SSH) $(OPENBSD_HOST) "/bin/sh /tmp/pkg-openbsd.sh \
		'$(NAME)' '$(VERSION)' '$(COMMENT)' '$(DESC)'"
	@echo "Copying signed package to PV via f0..."
	$(OPENBSD_SCP) $(OPENBSD_HOST):/tmp/$(NAME)-pkg/out/$(NAME)-$(VERSION).tgz /tmp/$(NAME)-$(VERSION).tgz
	$(FREEBSD_SCP) /tmp/$(NAME)-$(VERSION).tgz $(FREEBSD_HOST):/tmp/$(NAME)-$(VERSION).tgz
	$(FREEBSD_SSH) $(FREEBSD_HOST) "doas cp /tmp/$(NAME)-$(VERSION).tgz $(PV_BASE)/$(OPENBSD_REPO)/ && rm /tmp/$(NAME)-$(VERSION).tgz"
	@# Clean up remote and local temp files
	$(OPENBSD_SSH) $(OPENBSD_HOST) "doas rm -rf /tmp/$(NAME)-pkg /tmp/$(NAME) /tmp/pkg-openbsd.sh"
	rm -f /tmp/$(NAME)-openbsd /tmp/$(NAME)-$(VERSION).tgz
	@echo "OpenBSD package $(NAME)-$(VERSION) uploaded to repo"

# Build and upload for both OSes
pkg: pkg-freebsd pkg-openbsd

# --- DTail (multi-binary, native build on OpenBSD VM) ---

# Sync dtail source to build VM and compile natively.
# Native build handles CGo dependencies (e.g. DataDog/zstd).
/tmp/dtail-binaries/.built: buildvm-ensure
	@echo "Building DTail $(DTAIL_VERSION) natively on OpenBSD build VM..."
	@mkdir -p /tmp/dtail-binaries
	@# Sync source to VM via git archive (excludes build artifacts, saves space)
	$(BUILDVM_SSH) $(BUILDVM_HOST) "rm -rf /tmp/dtail-src"
	cd $(DTAIL_SRC) && git archive HEAD | $(BUILDVM_SSH) $(BUILDVM_HOST) "mkdir -p /tmp/dtail-src && tar -C /tmp/dtail-src -xf -"
	@# Build all binaries natively on OpenBSD
	$(BUILDVM_SSH) $(BUILDVM_HOST) "cd /tmp/dtail-src && for bin in $(DTAIL_BINARIES); do \
		echo \"  Building \$$bin...\"; \
		go build -o /tmp/\$$bin ./cmd/\$$bin/main.go || exit 1; \
	done"
	@# Retrieve built binaries
	@for bin in $(DTAIL_BINARIES); do \
		$(BUILDVM_SCP) $(BUILDVM_HOST):/tmp/$$bin /tmp/dtail-binaries/$$bin; \
	done
	$(BUILDVM_SSH) $(BUILDVM_HOST) "rm -rf /tmp/dtail-src /tmp/dserver /tmp/dcat /tmp/dgrep /tmp/dmap /tmp/dtail /tmp/dtailhealth"
	@# Bundle config files alongside binaries
	cp $(CONF_FRONTENDS)/etc/dserver/dtail.json.tpl /tmp/dtail-binaries/dtail.json
	cp $(CONF_FRONTENDS)/scripts/dserver-update-key-cache.sh.tpl /tmp/dtail-binaries/dserver-update-key-cache.sh
	cp $(CONF_FRONTENDS)/etc/rc.d/dserver.tpl /tmp/dtail-binaries/dserver.rc
	@touch $@

# Package, sign, and upload the OpenBSD dtail package.
# Binaries are built on the local build VM; packaging and signing happen on fishfinger.
dtail-openbsd: /tmp/dtail-binaries/.built
	@echo "Packaging dtail $(DTAIL_VERSION) for OpenBSD..."
	$(OPENBSD_SCP) -r /tmp/dtail-binaries $(OPENBSD_HOST):/tmp/dtail-binaries
	$(OPENBSD_SCP) $(SCRIPTS)/pkg-dtail-openbsd.sh $(OPENBSD_HOST):/tmp/pkg-dtail-openbsd.sh
	$(OPENBSD_SSH) $(OPENBSD_HOST) "/bin/sh /tmp/pkg-dtail-openbsd.sh '$(DTAIL_VERSION)'"
	@echo "Copying signed package to PV via f0..."
	$(OPENBSD_SCP) $(OPENBSD_HOST):/tmp/dtail-pkg/out/dtail-$(DTAIL_VERSION).tgz /tmp/dtail-$(DTAIL_VERSION).tgz
	$(FREEBSD_SCP) /tmp/dtail-$(DTAIL_VERSION).tgz $(FREEBSD_HOST):/tmp/dtail-$(DTAIL_VERSION).tgz
	$(FREEBSD_SSH) $(FREEBSD_HOST) "doas cp /tmp/dtail-$(DTAIL_VERSION).tgz $(PV_BASE)/$(OPENBSD_REPO)/ && rm /tmp/dtail-$(DTAIL_VERSION).tgz"
	@# Clean up remote and local temp files
	$(OPENBSD_SSH) $(OPENBSD_HOST) "doas rm -rf /tmp/dtail-pkg /tmp/dtail-binaries /tmp/pkg-dtail-openbsd.sh"
	rm -rf /tmp/dtail-binaries /tmp/dtail-$(DTAIL_VERSION).tgz
	@echo "OpenBSD package dtail-$(DTAIL_VERSION) uploaded to repo"

# Build RPMs for Rocky Linux 9 (x86_64 + aarch64), generate repodata, and upload to the PV.
dtail-rocky:
	@echo "Packaging dtail $(DTAIL_VERSION) for Rocky Linux $(ROCKY_VERSION)..."
	rm -rf /tmp/dtail-rocky-repo /tmp/dtail-rocky-upload /tmp/dtail-rocky-aarch64-root
	mkdir -p /tmp/dtail-rocky-repo/x86_64 /tmp/dtail-rocky-repo/aarch64
	$(SCRIPTS)/pkg-dtail-rpm.sh x86_64 '$(DTAIL_VERSION)' '$(DTAIL_SRC)' '$(DTAIL_RPM_FILES)' /tmp/dtail-rocky-repo/x86_64
	mkdir -p /tmp/dtail-rocky-aarch64-root/usr/local/bin /tmp/dtail-rocky-aarch64-root/etc/dserver /tmp/dtail-rocky-aarch64-root/usr/lib/systemd/system /tmp/dtail-rocky-aarch64-root/usr/share/licenses/dtail
	cd $(DTAIL_SRC) && for bin in $(DTAIL_BINARIES); do \
		CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags nozstd -o /tmp/dtail-rocky-aarch64-root/usr/local/bin/$$bin ./cmd/$$bin/main.go || exit 1; \
	done
	install -m 0644 $(DTAIL_RPM_FILES)/dtail.json /tmp/dtail-rocky-aarch64-root/etc/dserver/dtail.json
	install -m 0755 $(DTAIL_RPM_FILES)/dserver-update-key-cache.sh /tmp/dtail-rocky-aarch64-root/usr/local/bin/dserver-update-key-cache.sh
	install -m 0644 $(DTAIL_RPM_FILES)/dserver.service /tmp/dtail-rocky-aarch64-root/usr/lib/systemd/system/dserver.service
	install -m 0644 $(DTAIL_RPM_FILES)/dserver-update-keycache.service /tmp/dtail-rocky-aarch64-root/usr/lib/systemd/system/dserver-update-keycache.service
	install -m 0644 $(DTAIL_RPM_FILES)/dserver-update-keycache.timer /tmp/dtail-rocky-aarch64-root/usr/lib/systemd/system/dserver-update-keycache.timer
	install -m 0644 $(DTAIL_SRC)/LICENSE /tmp/dtail-rocky-aarch64-root/usr/share/licenses/dtail/LICENSE
	$(ROCKY_ARM_SSH) $(ROCKY_ARM_BUILD_HOST) "rm -rf /tmp/dtail-rocky-aarch64-root /tmp/dtail-rocky-aarch64-out /tmp/pkg-dtail-rpm.sh"
	$(ROCKY_ARM_SCP) -r /tmp/dtail-rocky-aarch64-root $(ROCKY_ARM_BUILD_HOST):/tmp/dtail-rocky-aarch64-root
	$(ROCKY_ARM_SCP) $(SCRIPTS)/pkg-dtail-rpm.sh $(ROCKY_ARM_BUILD_HOST):/tmp/pkg-dtail-rpm.sh
	$(ROCKY_ARM_SSH) $(ROCKY_ARM_BUILD_HOST) "chmod +x /tmp/pkg-dtail-rpm.sh && mkdir -p /tmp/dtail-rocky-aarch64-out && DTAIL_PREBUILT_ROOT=/tmp/dtail-rocky-aarch64-root /tmp/pkg-dtail-rpm.sh aarch64 '$(DTAIL_VERSION)' '$(DTAIL_SRC)' '$(DTAIL_RPM_FILES)' /tmp/dtail-rocky-aarch64-out"
	$(ROCKY_ARM_SCP) $(ROCKY_ARM_BUILD_HOST):/tmp/dtail-rocky-aarch64-out/*.rpm /tmp/dtail-rocky-repo/aarch64/
	$(ROCKY_ARM_SSH) $(ROCKY_ARM_BUILD_HOST) "rm -rf /tmp/dtail-rocky-aarch64-root /tmp/dtail-rocky-aarch64-out /tmp/pkg-dtail-rpm.sh"
	rm -rf /tmp/dtail-rocky-aarch64-root
	$(CONTAINER_ENGINE) run --rm -v /tmp/dtail-rocky-repo:/repo:Z docker.io/library/rockylinux:9 \
		bash -lc 'dnf -qy install createrepo_c >/dev/null && createrepo_c /repo/x86_64 >/dev/null && createrepo_c /repo/aarch64 >/dev/null'
	mkdir -p /tmp/dtail-rocky-upload/$(ROCKY_VERSION)
	cp -R /tmp/dtail-rocky-repo/x86_64 /tmp/dtail-rocky-upload/$(ROCKY_VERSION)/
	cp -R /tmp/dtail-rocky-repo/aarch64 /tmp/dtail-rocky-upload/$(ROCKY_VERSION)/
	$(FREEBSD_SCP) -r /tmp/dtail-rocky-upload $(FREEBSD_HOST):/tmp/dtail-rocky-upload
	$(FREEBSD_SSH) $(FREEBSD_HOST) "doas mkdir -p $(PV_BASE)/rockylinux && doas rm -rf $(PV_BASE)/$(ROCKY_REPO) && doas cp -R /tmp/dtail-rocky-upload/$(ROCKY_VERSION) $(PV_BASE)/rockylinux/ && rm -rf /tmp/dtail-rocky-upload"
	rm -rf /tmp/dtail-rocky-repo /tmp/dtail-rocky-upload
	@echo "Rocky Linux repo updated at $(PV_BASE)/$(ROCKY_REPO)"

clean:
	rm -f /tmp/$(NAME)-freebsd /tmp/$(NAME)-openbsd /tmp/$(NAME)-*.tgz
	rm -rf /tmp/dtail-binaries /tmp/dtail-*.tgz
	rm -rf /tmp/dtail-rocky-repo /tmp/dtail-rocky-upload /tmp/dtail-rocky-aarch64-root
