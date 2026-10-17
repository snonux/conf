# FreeBSD Kubernetes follow-up

Checked **17 October 2026**, following the 15 October investigation in
`reports/Kubernetes on vanilla FreeBSD.md`. This is a documentation and source
review; no native cluster was built, no workload was migrated, and repository
configuration is not evidence of current running state.

**Keep the homelab's Linux K3s guests. Native FreeBSD Kubernetes remains an
experiment in the primary sources checked.** Doug Rabson's demo explicitly
limits itself to evaluation and disclaims Kubernetes-project review and
support. It documents FreeBSD 14.3-RELEASE with patched Kubernetes 1.36.1, rather
than a supported native distribution. The follow-up adds an important OS
lifecycle correction, concrete resource/storage limitations, current ports and
working-group checks, and workload-specific migration gates.
([Demo README](https://github.com/dfr/kubernetes-demo/blob/e81c2d20dd2f881071cb7f93763bb30ff1d96546/README.md))

## Versions and delivery status

The demo HEAD is still commit `e81c2d20dd2f881071cb7f93763bb30ff1d96546`, dated
21 June 2026. Its GitHub releases endpoint returned an empty list. Branch names
are version labels, not immutable dependency pins: the script fetches and pulls
source. For reproducibility, the source revisions examined here are recorded
below. ([Demo history](https://api.github.com/repos/dfr/kubernetes-demo/commits?per_page=3),
[Releases](https://api.github.com/repos/dfr/kubernetes-demo/releases))

| Component | Observed target or revision | What that establishes |
|---|---|---|
| FreeBSD host | Demo documents 14.3-RELEASE | A historical experiment baseline; no published demo compatibility matrix for newer releases. |
| Kubernetes | 1.36.1; kubelet branch `freebsd-kubelet-v1.36.1` at `edf7be819240b65fd16808e216d17f89a3b849eb` (24 July); kubeadm branch at `0a9557e60f80d27f36b55c0e3e9c6702f5c773a5` | Patched source components, without a FreeBSD maintenance/support promise. |
| CRI-O | `freebsd-wip-1.37.0` at `ae934645818a4fbdc23769d60a9cd5de0fe79c84` (6 August) | FreeBSD work branch; its number does not change the demo's Kubernetes version. |
| Container/runtime ports | Podman 5.8.9 revision 1; ocijail 0.6.1; CNI plugins FreeBSD fork 0.6 revision 19 | Useful packaged building blocks; package existence does not certify Kubernetes integration. |
| Kubernetes control-plane ports | `net/kube-apiserver`, `net/kube-controller-manager`, `net/kube-scheduler`: 1.27.2 revision 33 | Old standalone binaries, insufficient for a current native worker/distribution. |

The Kubernetes and CRI-O branch identities were checked through the
[kubelet history](https://api.github.com/repos/dfr/kubernetes/commits?sha=freebsd-kubelet-v1.36.1&per_page=2),
[kubeadm branch](https://api.github.com/repos/dfr/kubernetes/branches/freebsd-kubeadm-v1.36.1)
and [CRI-O history](https://api.github.com/repos/dfr/cri-o/commits?sha=freebsd-wip-1.37.0&per_page=2).
Port versions were read at FreeBSD ports snapshot
`d5157320444a27739b9ca1558e7cec695ce7fffd`:
[Podman](https://github.com/freebsd/freebsd-ports/blob/d5157320444a27739b9ca1558e7cec695ce7fffd/sysutils/podman/Makefile),
[ocijail](https://github.com/freebsd/freebsd-ports/blob/d5157320444a27739b9ca1558e7cec695ce7fffd/sysutils/ocijail/Makefile),
[CNI](https://github.com/freebsd/freebsd-ports/blob/d5157320444a27739b9ca1558e7cec695ce7fffd/net/containernetworking-plugins/Makefile),
[API server](https://github.com/freebsd/freebsd-ports/blob/d5157320444a27739b9ca1558e7cec695ce7fffd/net/kube-apiserver/Makefile),
[controller manager](https://github.com/freebsd/freebsd-ports/blob/d5157320444a27739b9ca1558e7cec695ce7fffd/net/kube-controller-manager/Makefile)
and [scheduler](https://github.com/freebsd/freebsd-ports/blob/d5157320444a27739b9ca1558e7cec695ce7fffd/net/kube-scheduler/Makefile).
The `net` and `sysutils` directory listings contained no kubelet, kubeadm or
CRI-O port; this is a scoped tree check, not proof about every outside project.
([net listing](https://api.github.com/repos/freebsd/freebsd-ports/contents/net?ref=d5157320444a27739b9ca1558e7cec695ce7fffd),
[sysutils listing](https://api.github.com/repos/freebsd/freebsd-ports/contents/sysutils?ref=d5157320444a27739b9ca1558e7cec695ce7fffd))

**14.3 is already out of security support:** its EOL was 30 June 2026. The
currently supported release table lists 14.4, 14.5 and 15.1. For a new experiment,
prefer a disposable 14.5 VM and treat bringing up the demo there as an unverified
porting exercise. A 14.3 VM would reproduce the old instructions only in an
isolated, disposable environment. Do not describe that baseline as a currently
supported installation. ([Unsupported releases](https://www.freebsd.org/security/unsupported/),
[Supported releases](https://www.freebsd.org/security/))

Kubernetes 1.36 is still an active upstream series, but the demo's 1.36.1 target
trails the patch history; upstream lifecycle status does not cover this fork.
The packaged 1.27 series reached upstream EOL on 16 July 2024.
([Kubernetes patch history](https://kubernetes.io/releases/patch-releases/))

Changing to another familiar distribution does not provide documented native
support: K3s requires Linux and cgroups; k0s lists Linux and Windows; MicroK8s
uses snapd or virtualization. Upstream's setup guide describes Linux control
planes. These checked support documents and the demo's disclaimer do not
establish a supported native FreeBSD distribution.
([K3s](https://docs.k3s.io/installation/requirements),
[k0s](https://docs.k0sproject.io/stable/system-requirements/),
[MicroK8s](https://canonical.com/microk8s/docs/getting-started),
[Kubernetes setup](https://kubernetes.io/docs/setup/))

## Working-group and OCI progress

The August Enterprise WG minutes target an **experimental** Kubernetes port with
build instructions by the end of 2026. They report AI workloads running on a
FreeBSD cluster, but supply neither a compatibility matrix nor conformance or
support evidence. The public archive index currently ends in September. Its
21 September update requests testers for draft container documentation, without
announcing a supported Kubernetes distribution. Treat the year-end milestone as
a target whose delivery still needs checking.
([August minutes](https://lists.freebsd.org/archives/freebsd-enterprisewg/2026-August/000156.html),
[Archive index](https://lists.freebsd.org/archives/freebsd-enterprisewg/),
[September update](https://lists.freebsd.org/archives/freebsd-enterprisewg/2026-September/000159.html))

The live Handbook now has an OCI Containers chapter, covering official images,
Podman, networking and Linux images. This improves the container setup story;
it does not document Kubernetes integration. The chapter says official OCI
images exist from 14.3 onward on amd64 and arm64. FreeBSD's inclusion in OCI
runtime specification 1.3 is also real progress, distinct from Kubernetes
conformance. ([OCI Handbook](https://docs.freebsd.org/en/books/handbook/containers/),
[OCI announcement](https://freebsdfoundation.org/blog/freebsd-officially-supported-in-oci-runtime-specification-v1-3/))

## Runtime, networking and storage limits

The native execution chain is kubelet → patched CRI-O → ocijail → FreeBSD jails.
The runtime's own README calls it experimental.
([ocijail](https://github.com/dfr/ocijail))

There is a concrete resource-management gap. The inspected kubelet's
`generateContainerResources` returns `nil`; its container manager uses stub/fake
pod, CPU and memory managers. CRI-O's `NullCgroupManager` reads RACCT statistics
but its minimum-memory verification is a no-op. **Inference:** enabling RACCT
and seeing metrics cannot establish that the homelab's Pod CPU/memory limits
are enforced. This source path does not implement the usual resource handoff;
require implementation evidence and a measured stress test before relying on
those limits. This is an upstream experimental limitation, not a change made
to this repository.
([Kubelet resource generation](https://github.com/dfr/kubernetes/blob/edf7be819240b65fd16808e216d17f89a3b849eb/pkg/kubelet/kuberuntime/kuberuntime_container_freebsd.go),
[Container manager](https://github.com/dfr/kubernetes/blob/edf7be819240b65fd16808e216d17f89a3b849eb/pkg/kubelet/cm/container_manager_freebsd.go),
[CRI-O resource manager](https://github.com/dfr/cri-o/blob/ae934645818a4fbdc23769d60a9cd5de0fe79c84/internal/config/cgmgr/cgmgr_freebsd.go))

The demo's CNI config uses a bridge, host-local IPAM, portmap, firewall and tuning
plugins, with outgoing masquerading disabled. Multi-node operation requires
manually connecting bridges and allocating distinct address ranges. A
`firewall` CNI plugin does not by itself establish Kubernetes NetworkPolicy
enforcement. DNS, ClusterIP, NodePort, egress, policy and cross-node recovery
remain acceptance tests, not verified capabilities in this review.
([CNI config](https://github.com/dfr/kubernetes-demo/blob/e81c2d20dd2f881071cb7f93763bb30ff1d96546/files/10-kubetest.conflist),
[Networking instructions](https://github.com/dfr/kubernetes-demo/blob/e81c2d20dd2f881071cb7f93763bb30ff1d96546/README.md#networking))

ZFS image-layer storage is useful, but does not provide Kubernetes dynamic PVs
or a CSI driver. The fork implements nullfs mounts, while `GetMountRefs` and
some safe-format/mount helpers still return unsupported errors. CRI-O's FreeBSD
bind-mount code explicitly leaves Linux-style mount propagation incomplete.
**Inference:** simple hostPath tests may work, but compatibility with arbitrary
CSI node plugins, propagated mounts, ownership and volume lifecycle cannot be
assumed. Test the exact hostPath/PVC/subPath design used here before attempting
data-bearing applications.
([Mount implementation](https://github.com/dfr/kubernetes/blob/edf7be819240b65fd16808e216d17f89a3b849eb/staging/src/k8s.io/mount-utils/mount_freebsd.go),
[CRI-O bind mounts](https://github.com/dfr/cri-o/blob/ae934645818a4fbdc23769d60a9cd5de0fe79c84/server/container_create_freebsd.go))

The build script assumes `zroot`, requires RACCT, installs packages, replaces
`kld_list`, configures forwarding/PF and uses an amd64 crictl output path. Its
fetches and `ghcr.io/dfr` images also need an explicit revision/digest inventory
for a repeatable experiment. Adapt it in the disposable VM; do not run it on
the f-hosts that already provide storage and ingress.
([Build script](https://github.com/dfr/kubernetes-demo/blob/e81c2d20dd2f881071cb7f93763bb30ff1d96546/build-cluster.sh))

## Linux images and the actual homelab

The Foundation demonstrated Alpine, Caddy and an older Python/Pandas image
through Podman/Linuxulator. The current Handbook also documents Linux-image
execution and warns about systemd, PID-1 assumptions and Linux kernel APIs.
The Linux compatibility chapter specifically identifies cgroups and namespaces
as missing facilities. Thus Linux userspace can work, but those Podman examples
do not establish Linux OCI-image selection and execution through this CRI-O /
kubelet stack. Start with a native FreeBSD image; test every later Linux image,
including init containers, through Kubernetes itself.
([Foundation tutorial](https://freebsdfoundation.org/blog/oci-containers-on-freebsd/),
[Handbook Linux containers](https://docs.freebsd.org/en/books/handbook/containers/#containers-linux),
[Linux compatibility](https://docs.freebsd.org/en/books/handbook/linuxemu/))

The repository defines f0–f2 hosting r0–r2 as four-CPU, 12 GB Rocky guests,
three K3s control-plane members, and CARP/relayd forwarding to Traefik. Storage
comes through a mutually authenticated stunnel NFSv4 mount at
`/data/nfs/k3svolumes`. Those dependencies survive an OS change as requirements;
the Linux systemd, mount and recovery recipes need FreeBSD equivalents.
([VM inventory](../gonf/cluster/freebsd_bhyve.go),
[K3s](../gonf/rnodes/k3s.go), [Ingress](../gonf/freebsd/relayd.go),
[NFS client](../gonf/rnodes/nfsclient.go))

| Repository workload or dependency | Assessment and migration gate |
|---|---|
| pkgrepo / nginx | Best first application candidate using a native nginx image and copied, read-only sample data. Its chart also uses a BusyBox NFS sentinel and a ConfigMap file via subPath: replace/test the init image and exercise that mount. [Chart](../f3s/pkgrepo/helm-chart/templates/deployment.yaml) |
| Miniflux + PostgreSQL | Potential later native rebuild candidates. Test image availability/builds, initialization, database locking, backup/restore and NFS behavior; changing only the node OS is insufficient. [Deployment](../f3s/miniflux/helm-chart/templates/deployment.yaml) |
| Forgejo, Radicale, Syncthing, registry | Plausible port/rebuild candidates, without verified Kubernetes compatibility. Existing images, entrypoints, user/group setup and BusyBox helpers need review. Registry access also needs a CRI-O equivalent of the current containerd mirror. [Forgejo](../f3s/forgejo/helm-chart/templates/deployment.yaml), [Radicale](../f3s/radicale/helm-chart/templates/deployment.yaml), [Syncthing](../f3s/syncthing/helm-chart/templates/deployment.yaml), [Registry settings](../gonf/rnodes/k3s.go) |
| Anki, Proton Bridge, Jellyfin, Immich | Hold on Linux pending exact-image testing or a maintained native rebuild. The Anki image explicitly builds on Alpine; Proton Bridge uses a third-party image; media/ML stacks add runtime and dependency checks. Hardware acceleration, if desired, requires a separate device-access test. [Anki Dockerfile](../f3s/anki-sync-server/docker-image/Dockerfile), [Bridge](../f3s/protonbridge/helm-chart/templates/deployment.yaml), [Jellyfin](../f3s/jellyfin/helm-chart/values.yaml), [Immich](../f3s/immich/values.yaml) |
| Traefik, cert-manager, Argo CD and monitoring | Treat as platform prerequisites with their own image, API, network, certificate-renewal and upgrade tests. Existing Ingress/CRD definitions do not install their controllers in the demo. Keep GitOps and monitoring outside the experiment until their dependencies pass. [Ingress usage](../f3s/pkgrepo/helm-chart/templates/ingress.yaml), [Certificates](../f3s/cert-manager/wildcard-certificate.yaml), [Argo CD](../f3s/argocd/values.yaml) |
| Shared PVs and init containers | Recreate the required mount path and sentinel behavior using test data; verify UID/GID, locking, subPath, permission changes, server failover and remount/reboot ordering. The current hard NFSv4-over-stunnel mount is more than a PVC declaration. [PV example](../f3s/pkgrepo/helm-chart/templates/persistent-volume.yaml), [NFS client](../gonf/rnodes/nfsclient.go) |

These are source-based migration assessments, not a complete live workload
inventory or a claim that any listed image has been tested on FreeBSD.

## Decision and next recheck

Proceed only with an isolated proof of concept. Use a separate FreeBSD VM,
console access, a snapshot, separate Pod/service ranges and copied data. First
prove native Pod creation/deletion, probes, exec/logs, ConfigMaps/Secrets,
Deployment replacement and reboot recovery. Then prove DNS/Services/egress,
resource-limit enforcement and the exact NFS/PVC/subPath path. Expand to a
second node only after that, and test rescheduling, address allocation, node
loss, upgrades and rollback before evaluating a service migration.

The migration decision remains **no** for the full homelab: the checked sources
do not establish a supported native distribution, complete resource handling,
the required network/storage behavior or compatibility with its image set.
The practical experiment is a native read-only web workload after the basic
platform checks pass. Keeping K3s in Linux bhyve guests remains the supported
OS path documented by K3s.

Recheck after the working group's end-of-2026 target, or earlier if it publishes
the experimental port. Look for a maintained release with a supported FreeBSD
version matrix, patch/update policy, worker/runtime packages, automated
multi-node networking, documented PV/CSI support, conformance results and
measured resource/security behavior. A new control-plane port or Podman
release alone would not resolve those migration gates.
