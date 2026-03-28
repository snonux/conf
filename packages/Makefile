# Build and upload custom packages to the f3s package repository.
#
# Usage:
#   make pkg NAME=gogios SRC=~/git/gogios          # both FreeBSD + OpenBSD
#   make pkg-freebsd NAME=gogios SRC=~/git/gogios   # FreeBSD only
#   make pkg-openbsd NAME=gogios SRC=~/git/gogios   # OpenBSD only
#
# Required variables:
#   NAME  — package name (e.g. gogios)
#   SRC   — path to the Go project root (must have cmd/$(NAME)/main.go
#           and internal/version.go with a Version constant)
#
# Optional variables:
#   COMMENT     — one-line package description
#   DESC        — longer description (for OpenBSD desc file)
#   MAINTAINER  — maintainer email
#   WWW         — project URL
#   ENTRY       — Go main package path relative to SRC (default: cmd/$(NAME)/main.go)

SHELL := /bin/bash
.ONESHELL:

# SSH targets
FREEBSD_HOST := f0.lan.buetow.org
FREEBSD_SSH  := ssh -p 22
FREEBSD_SCP  := scp -P 22
OPENBSD_HOST := rex@fishfinger.buetow.org
OPENBSD_SSH  := ssh
OPENBSD_SCP  := scp

# NFS-backed PV on f0
PV_BASE         := /data/nfs/k3svolumes/pkgrepo
FREEBSD_REPO    := freebsd/FreeBSD:15:amd64/latest
OPENBSD_VERSION := 7.8
OPENBSD_REPO    := openbsd/$(OPENBSD_VERSION)/packages/amd64

# Defaults for package metadata
COMMENT    ?= $(NAME)
DESC       ?= $(NAME)
MAINTAINER ?= paul@buetow.org
WWW        ?= https://buetow.org
ENTRY      ?= cmd/$(NAME)/main.go

SCRIPTS := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))scripts

# Extract version from internal/version.go, stripping the "v" prefix
VERSION = $(shell grep 'Version' $(SRC)/internal/version.go | sed 's/.*"\(.*\)"/\1/' | tr -d v)

.PHONY: pkg pkg-freebsd pkg-openbsd check-vars clean

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

clean:
	rm -f /tmp/$(NAME)-freebsd /tmp/$(NAME)-openbsd /tmp/$(NAME)-*.tgz
