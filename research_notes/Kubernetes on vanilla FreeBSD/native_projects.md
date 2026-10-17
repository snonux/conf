# Native Kubernetes and OCI projects on vanilla FreeBSD

## Is there a real native Kubernetes experiment now?

### Takeaway
Yes: Doug Rabson's FreeBSD Kubernetes Tech Demo targets stock FreeBSD 14.3-RELEASE and Kubernetes v1.36.1, including a native kubelet and jail-backed runtime. This is materially newer than the old 1.24/1.27/1.29 experiments, but its author explicitly limits it to experimentation/evaluation and disclaims Kubernetes-project review/support.

### Cited Findings
- The [demo README](https://github.com/dfr/kubernetes-demo) specifies a FreeBSD 14.3 host or VM, builds missing components, creates a single-node cluster, and permits small manually networked multi-node clusters. It warns that included Kubernetes patches are not reviewed, endorsed or supported upstream. Networking uses CNI bridges; multi-node requires linking bridges, for example via VLAN, and separate manually configured allocation ranges. Outbound NAT is disabled in the example; `ipMasq=true` enables it for single-node use. The README explicitly now targets v1.36.1, replacing v1.31.0. **A FreeBSD VM here still has native FreeBSD workers, unlike Linux VMs hosting ordinary K3s.** [README](https://github.com/dfr/kubernetes-demo)
- Commit history shows initial demo July 13, 2025; fixes in September/October 2025; explicit branches June 3, 2026; RACCT enforcement June 21, 2026; latest v1.36.1 upgrade commit e81c2d20dd2f (author date June 20, pushed June 21). [GitHub API history](https://api.github.com/repos/dfr/kubernetes-demo/commits?per_page=8)
- The v1.36.1 kubelet branch has July 24, 2026 commits, including ownership support, FreeBSD state paths and WIP privileged-container security context. Do not mistake the fork's inherited upstream README title “Production-Grade” for a claim about FreeBSD maturity. [Branch commits API](https://api.github.com/repos/dfr/kubernetes/commits?sha=freebsd-kubelet-v1.36.1&per_page=3)
- Enterprise WG meeting August 19, 2026 targets an experimental Kubernetes port with build instructions in the ports tree by end-of-year. It reports an existing FreeBSD Kubernetes cluster running AI workloads, but does not supply enough architecture detail to establish workload/kernel compatibility or reproducibility. Treat the ports statement as a target, not delivery. [Meeting notes](https://lists.freebsd.org/archives/freebsd-enterprisewg/2026-August/000156.html)

### Inferences
- Best native proof-of-concept starting point is the Rabson demo, not historical guides or a K3s installer. Stock FreeBSD kernel suffices according to the documented setup; the userspace Kubernetes/CRI-O components require patches and building.
- Recommend one disposable **FreeBSD** VM or spare machine with ZFS, console access and snapshot; inspect/adapt script first, then test native OCI workloads before touching existing cluster/storage. This is a research recommendation, not a tested deployment.

### Gaps
- No independent reproduction was performed; this Linux workspace is not evidence of runtime success.
- No demonstrated conformance certification, tested HA/upgrade sequence, storage-driver matrix, supported Linux-image scheduling or production SLA found.
- No full delivery verification for the promised end-of-2026 ports-tree package. Upstream researcher checked old core component ports still at 1.27.2.

## What does the native demo actually require/change?

### Takeaway
The underlying OS can remain vanilla; the experiment needs root changes, ZFS and modified Kubernetes/runtime software. Its build script is a starting reference rather than a safe installer for an existing homelab server.

### Cited Findings
- [Build script](https://github.com/dfr/kubernetes-demo/blob/main/build-cluster.sh) checks `kern.racct.enable=1` (boot tunable/reboot), installs podman-suite, expects ocijail >=0.6.0, and loads nullfs/fdescfs/if_bridge/pf. It changes persistent module settings, forwarding/filtering sysctls and service startup; builds crictl, CRI-O branch `freebsd-wip-1.37.0`, kubelet/kubeadm `freebsd-*-v1.36.1`; uses `ghcr.io/dfr` control-plane/etcd images. Storage assumes pool `zroot` when creating `/var/db/containers`. Script uses sudo, overwrites `kld_list`, creates PF configuration only if missing, and lacks `set -e`. It initializes cluster, approves CSRs and removes control-plane taint. Existing PF needs manual integration. These are direct observations from code, not an execution result. [Build source](https://github.com/dfr/kubernetes-demo/blob/main/build-cluster.sh)
- The supplied CNI chain is bridge + host-local IPAM + portmap + firewall + tuning, with default 10.244.0.0/16, allocation .2 through .254 and gateway .1. [CNI configuration](https://github.com/dfr/kubernetes-demo/blob/main/files/10-kubetest.conflist)
- The runtime ocijail describes itself as an experimental OCI runtime using FreeBSD jails, intended for Buildah/Podman. [ocijail](https://github.com/dfr/ocijail)

### Inferences
- Source assumes amd64 (`crictl` install path); do not advertise Raspberry Pi/ARM support without adapting and checking images.
- ZFS image storage does not itself provide Kubernetes persistent-volume provisioning, CSI, shared storage or distributed replication.
- Validate Pod creation/deletion, Deployment rollout/restart, DNS, same-node networking, service routing, egress, mounts/PVC behavior, resource accounting and reboot before evaluating multi-node/HA. Begin with native FreeBSD image.

### Gaps
- No documented complete Kubernetes storage story in this demo, nor claim that Cilium/Calico/Flannel, standard CSI plugins, privileged Linux DaemonSets, or arbitrary Helm charts work.

## How do OCI, Linux images and older projects affect the answer?

### Takeaway
OCI support is real and advancing, and Linux userspace images can sometimes run through Linuxulator; this does not make stock K3s or all Linux Kubernetes workloads supported on FreeBSD.

### Cited Findings
- FreeBSD was added to OCI runtime specification v1.3 on November 4, 2025; FreeBSD OCI containers use jails. Official FreeBSD OCI images began in 14.2 and are published to Docker Hub/GHCR. Specification-level support must not be confused with end-to-end Kubernetes support. [Foundation announcement](https://freebsdfoundation.org/blog/freebsd-officially-supported-in-oci-runtime-specification-v1-3/)
- Foundation's October 31, 2025 tutorial explicitly tested vanilla 14.3 and 15.0-BETA3 with `pkg install podman-suite`, PF configuration and `service linux start`. It demonstrates `podman run --os=linux` for Alpine, Caddy and a CentOS/Pandas image, plus a native FreeBSD runtime image. Podman needs root in this tutorial. Thus **“Linux OCI images never work on FreeBSD” is false**; this tutorial does not establish Kubernetes scheduling those images. [Foundation OCI tutorial](https://freebsdfoundation.org/blog/oci-containers-on-freebsd/)
- runj is a proof-of-concept jail OCI runtime with an experimental containerd shim; README says security has not been evaluated and explicitly warns against production. Its containerd integration requires 1.5+. This is foundational work, not a ready Kubernetes distribution. [runj](https://github.com/samuelkarp/runj)
- Tobias Norlin's December 13, 2023 v1.29 guide demonstrates a native FreeBSD control plane in jails and references an earlier 100% FreeBSD virtual-kubelet concept; author calls it experimental and not mature for production. It does not furnish current maintained native-worker distribution evidence. [Author guide](https://kubernaut.eu/posts/welcome-mandala-kubernetes-v129-on-freebsd-controlplane/)
- FreeBSDCloudTech is the actual GitHub organization for coordinating this work. Its fork timestamps mostly date to early 2025; active Rabson work is in `dfr/*`, so org fork timestamps alone falsely suggest stagnation. [Organization](https://github.com/FreeBSDCloudTech); [API repository listing](https://api.github.com/orgs/FreeBSDCloudTech/repos?per_page=100)

### Inferences
- Native FreeBSD applications should use FreeBSD OCI images. Linuxulator eligibility is workload-specific; successful userspace demos do not establish availability of Linux kernel APIs (cgroups, namespaces, eBPF), broad Linux node compatibility or a working K3s agent.
- Use author demo as a practical experiment, Foundation/working-group updates for direction, and upstream Kubernetes docs for support claims. Avoid categorical “Kubernetes cannot run on FreeBSD” wording.

### Gaps
- Could not establish a currently maintained virtual-kubelet jail provider with turnkey installation. No reason to prioritize it over the current full kubelet demo.
- `knast/freebsd` appears as an older image name in nerdctl documentation, not verified as a current competing Kubernetes distribution. Do not recommend it without further evidence.
