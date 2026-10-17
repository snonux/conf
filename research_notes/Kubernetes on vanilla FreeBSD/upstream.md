# Upstream Kubernetes distributions on native FreeBSD

Research checked 2026-10-15. Scope is upstream supported distributions, not emerging FreeBSD-specific forks; the native-project researcher covers those.

## Can stock K3s, k0s, MicroK8s, or upstream Kubernetes run natively?

### Takeaway
None of these distributions documents native FreeBSD as a supported platform. This is a support/deployment verdict, not a claim that every Kubernetes component is intrinsically Linux-only or that new native FreeBSD experiments do not work.

### Cited Findings
- K3s requirements explicitly target most modern Linux systems, including container/native-service installation; cgroups are a stated requirement. Baseline server requirements are 2 cores/2 GB RAM; agents 1 core/512 MB, excluding workload resources. — [K3s requirements](https://docs.k3s.io/installation/requirements)
- Upstream Kubernetes states its control plane is designed for Linux, and officially documents Linux or Windows workers. — [Getting started](https://kubernetes.io/docs/setup/); [Windows in Kubernetes](https://kubernetes.io/docs/concepts/windows/)
- k0s system requirements list Linux and Windows; the stable docs' installation navigation labels Windows experimental, and the tested OS list is Linux distributions. FreeBSD is absent. — [k0s system requirements](https://docs.k0sproject.io/stable/system-requirements/)
- MicroK8s requires Ubuntu or another snapd-supporting OS and recommends Multipass if the user has no Linux machine. Its alternative install page covers Windows/macOS and virtualization rather than FreeBSD. — [MicroK8s getting started](https://canonical.com/microk8s/docs/getting-started); [Alternative installation](https://canonical.com/microk8s/docs/install-alternatives)

### Inferences
- Switching K3s to k0s or MicroK8s does not remove the native FreeBSD integration problem.
- A Linux VM on a FreeBSD hypervisor can satisfy a distro's Linux requirements, but workloads then execute against the guest Linux kernel, so this does not meet a strict no-Linux/native-FreeBSD requirement.

### Gaps
- No execution tests were performed. Support docs establish documented platforms, not exhaustive impossibility proofs.
- Emerging FreeBSD native efforts must be considered separately; do not turn these findings into a categorical 'Kubernetes on FreeBSD is impossible' claim.

## Why does Linuxulator not supply stock Linux Kubernetes worker semantics?

### Takeaway
The FreeBSD handbook still explicitly lists cgroups and namespaces as Linuxulator features that are not supported. A compatible Linux userland is therefore insufficient for the stock Linux worker stack; a native FreeBSD port must integrate the runtime, kubelet, networking, and isolation behavior with FreeBSD facilities.

### Cited Findings
- Linuxulator runs unmodified Linux binaries, but the handbook specifically names cgroups and namespaces among unsupported Linux system-management functionality. — [FreeBSD Linux binary compatibility](https://docs.freebsd.org/en/books/handbook/linuxemu/)
- Kubernetes uses Linux cgroups as pod resource-control boundaries; its Windows implementation uses different native mechanisms, demonstrating that cgroups are not a universal requirement of the abstract Kubernetes API. — [Kubernetes resource management on Windows](https://kubernetes.io/docs/concepts/configuration/windows-resource-management/)
- Each Kubernetes workload node needs a working container runtime; CRI is the kubelet-to-runtime plugin protocol. — [Container Runtime Interface](https://kubernetes.io/docs/concepts/containers/cri/)

### Inferences
- A Linux jail is not equivalent to a full Linux VM and should not be presented as an established route to running unmodified K3s workers.
- Native jail/OCI container tools alone do not establish Kubernetes compatibility: the kubelet and runtime must actually interoperate via CRI, and pod/network/storage semantics must be implemented and verified.
- The existence of Windows workers demonstrates native FreeBSD workers are architecturally possible; engineering and support maturity, rather than an absolute Linux-only design, is the question.

### Gaps
- This research did not audit the completeness of individual FreeBSD runtime implementations; see the separate native-project findings.

## Are native control-plane components different from native workload nodes?

### Takeaway
Yes. Official FreeBSD ports contain control-plane binaries, but the checked ports are stale Kubernetes 1.27.2. They are evidence that native control-plane code can be packaged, not a current full FreeBSD Kubernetes distribution.

### Cited Findings
- Current main-branch official FreeBSD ports Makefiles for kube-apiserver, kube-controller-manager, and kube-scheduler all specify PORTVERSION 1.27.2 and PORTREVISION 33. The scheduler Makefile also supplies an rc service. — [API server Makefile](https://raw.githubusercontent.com/freebsd/freebsd-ports/main/net/kube-apiserver/Makefile); [Controller manager Makefile](https://raw.githubusercontent.com/freebsd/freebsd-ports/main/net/kube-controller-manager/Makefile); [Scheduler Makefile](https://raw.githubusercontent.com/freebsd/freebsd-ports/main/net/kube-scheduler/Makefile)
- Kubernetes 1.27 reached upstream end of life on 2024-07-16, with final patch version 1.27.16. — [Kubernetes patch release history](https://kubernetes.io/releases/patch-releases/)
- K3s offers experimental agentless servers via --disable-agent. These omit kubelet, container runtime, and CNI, do not register a Node, and cannot run pods. The docs do not claim this adds FreeBSD support. — [K3s agentless servers](https://docs.k3s.io/advanced#running-agentless-servers-experimental)

### Inferences
- A native control plane managing Linux workers is a different achievement from scheduling containers directly on FreeBSD. Native control-plane feasibility must not be mistaken for native pod capability.
- Do not recommend installing the old 1.27.2 ports for a new cluster; port revision bumps are not an upgrade to current Kubernetes releases.
- --disable-agent is not evidence of a supported native K3s FreeBSD installation.

### Gaps
- The exact build/runtime health of the old ports was not tested, nor were current package repository binaries checked.
- Search surfaced an unrelated howtomgr/k3s repository claiming `pkg install k3s` works. It is not the upstream K3s project and conflicts with official platform docs; it was excluded as evidence.
