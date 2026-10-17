# FreeBSD bhyve hosting and fit to this homelab

## Can vanilla FreeBSD host a practical K3s deployment?

### Takeaway
Yes, through Linux virtual machines under the base-system bhyve hypervisor, with vm-bhyve as a package for administration. This preserves an ordinary FreeBSD host but runs Kubernetes components and workloads on a Linux guest kernel; it does not meet a strict requirement to remove Linux entirely.

### Cited Findings
- FreeBSD's handbook documents bhyve in the base system since FreeBSD 10, Linux guest support, and UEFI Linux boot. It lists Intel EPT, AMD RVI/NPT, or aarch64 hardware, with Intel UG needed for the relevant Linux guest case. Architecture support in current documentation must not be read as proof every older release supports it. — [FreeBSD handbook](https://docs.freebsd.org/en/books/handbook/virtualization/#virtualization-host-bhyve)
- vm-bhyve supports Linux guests, ZFS-backed VM directories, bridge switches attached to physical NICs, UEFI firmware, autostart, cloud images, and optional zvol disks. Its README documents installing `vm-bhyve` and `bhyve-firmware`. — [vm-bhyve upstream README](https://github.com/freebsd/vm-bhyve/blob/master/README.md)
- K3s targets modern Linux systems. Its documented baseline is 2 CPU cores/2 GB RAM for a server, 1 core/512 MB for an agent, excluding workloads, with SSD storage recommended. Requirements include unique hostnames, server TCP 6443 connectivity, TCP 2379–2380 between embedded-etcd servers, TCP 10250 for metrics, and CNI-specific node-to-node traffic (VXLAN UDP 8472 or WireGuard UDP 51820/51821). — [K3s requirements](https://docs.k3s.io/installation/requirements)

### Inferences
- For a small isolated proof of concept, propose a Linux VM with 2 vCPUs, 4 GB RAM, and roughly 30–40 GB SSD-backed disk, UEFI and a bridged virtio NIC. This is an engineering starting point, not an official capacity guarantee; application images and volumes dominate storage needs. K3s itself does not require nested hardware virtualization.
- Reuse Rocky Linux for consistency with this repository; switching Kubernetes distribution or Linux guest OS brings little benefit solely for FreeBSD hosting. The kernel boundary remains in place with any Linux guest distribution.
- Keep the Linux guest filesystem conventional, e.g. ext4/XFS, on a host ZFS-backed virtual disk. Host ZFS benefits remain available without requiring ZFS inside the guest. A guest's Kubernetes hostPath points inside the guest, not at the FreeBSD host filesystem.

### Gaps
- No live host inspection or benchmarks were performed. CPU capability, spare memory, current FreeBSD release, disk headroom, live K3s version and network reachability were not verified.
- Other researchers cover native Kubernetes and experimental FreeBSD runtimes; this note does not establish that native execution is impossible.

## How does this match the existing conf repository?

### Takeaway
The proposed VM architecture is already encoded in the user's repository. The likely useful investigation is therefore whether the Linux layer can be removed, rather than explaining bhyve as a new solution.

### Cited Findings
- Inventory identifies f0–f3 as FreeBSD/amd64 hosts and r0–r2 as Linux/amd64 K3s nodes. f0–f2 share `k3sBhyve`; f3 has a separate ordinary Rocky VM. — [cluster.go](/home/paul/git/conf/gonf/cluster/cluster.go:165)
- The existing r0–r2 guest configuration uses 4 CPUs and 12 GB RAM each; f3's Rocky VM uses 4 CPUs/14 GB. Comments date observed values to 2026-09-25 and explicitly say older 14 GB documentation for r0–r2 is outdated. — [freebsd_bhyve.go](/home/paul/git/conf/gonf/cluster/freebsd_bhyve.go:5)
- Existing FreeBSD recipes install vm-bhyve/UEFI firmware and configure Linux guests with UEFI, virtio-net on the `public` switch, and an emulated NVMe `disk0.img`. The VM directory is `zfs:zroot/bhyve`; comments identify `public` on `re0`. VM creation, OS installation, disk images, MAC/UUID identity and switch bootstrap are intentionally outside these recipes. — [bhyve.go](/home/paul/git/conf/gonf/freebsd/bhyve.go:9)
- K3s recipes describe all r0–r2 as control-plane members, with r0 initialized and r1/r2 joined to it. Node and advertise addresses use each guest's WireGuard IP. Configuration changes intentionally do not restart all nodes because that would lose etcd quorum. Binary installation/version and join credentials remain outside the recipe. — [k3s.go](/home/paul/git/conf/gonf/rnodes/k3s.go:9)
- FreeBSD f0/f1 provide CARP and relayd, forwarding the VIP's TCP 80/443 to Traefik on the Linux nodes. TLS terminates in Traefik. — [relayd.go](/home/paul/git/conf/gonf/freebsd/relayd.go:9)

### Inferences
- Recommend retaining the existing working FreeBSD + bhyve + Rocky + K3s layout for normal services, while evaluating native alternatives in a separate experiment. Do not recommend replacing this stack merely to obtain Kubernetes on FreeBSD-hosted hardware: that goal is already represented by the code.
- If additional capacity is wanted, a fresh Linux guest can join the existing K3s cluster as an agent, but must match the network/registry/storage conventions. Reusing hostnames or cloning existing K3s state is inappropriate. f3's VM cannot be assumed unused or available just because it is not in the K3s inventory.
- Three control-plane VMs only provide physical host fault tolerance when distributed across hosts; the current f0/f1/f2 mapping satisfies that topology on paper. A host failure also removes its co-located storage/ingress services, so those dependencies still matter.

### Gaps
- These are repository findings, not verified live state. We did not access hosts, tokens, certificates, secrets, or operational dashboards.
- The broad source describes K3s roles, but exact live CNI, version, admission configuration and available capacity were not checked.

## What storage, network and high-availability boundaries matter?

### Takeaway
FreeBSD can remain the storage and network host while standard Linux Kubernetes runs above it. Existing NFS and CARP integration is already substantial; introducing a new node requires reproducing these dependencies, and replacing Linux workers with FreeBSD would also require replacing or validating their storage/runtime integrations.

### Cited Findings
- All r-nodes mount `/data/nfs/k3svolumes` over NFSv4 using a local mutually authenticated stunnel client connected to f0/f1's CARP VIP. Mounts are explicitly `hard` and clients share NFSv4 identity mapping. — [nfsclient.go](/home/paul/git/conf/gonf/rnodes/nfsclient.go:9)
- Existing application PVs use hostPath under that NFS mount; examples explain identical mount paths across nodes and warn about concurrent writers. — [Navidrome PVs](/home/paul/git/conf/f3s/navidrome/helm-chart/templates/persistent-volume.yaml:1)
- f0's NFS dataset is configured for zrepl replication to f1 at one-minute intervals. — [freebsd_storage.go](/home/paul/git/conf/gonf/cluster/freebsd_storage.go:16)
- K3s includes local-path provisioning, which binds volumes to a particular node; it is not shared storage by default. — [K3s storage](https://docs.k3s.io/add-ons/storage)
- The Kubernetes NFS CSI driver consumes an existing NFSv3/v4 server and can provision subdirectories dynamically; the project lists GA status and Linux node support. — [NFS CSI upstream](https://github.com/kubernetes-csi/csi-driver-nfs)
- Embedded-etcd HA uses at least three server nodes and benefits from odd member counts. Additional agents join through a server URL and join token, while server additions must match critical configuration. — [K3s HA](https://docs.k3s.io/datastore/ha-embedded), [K3s quick start](https://docs.k3s.io/quick-start)
- K3s documents datastore-aware backup and restoration, including preservation of the server token; embedded-etcd uses `k3s etcd-snapshot`. — [K3s backup](https://docs.k3s.io/datastore/backup-restore)

### Inferences
- Keep etcd/container state on fast guest-local disk; expose shared application storage separately through the established NFS path. Do not imply host ZFS is automatically a Kubernetes storage class.
- CSI NFS is a possible future simplification, not necessary to answer this request, and not a drop-in replacement for the existing stunnel/NFS hostPath conventions without testing transport, permissions and workload semantics.
- Scheduled one-minute ZFS replication is asynchronous and is not evidence of zero data loss. Actual recovery-point exposure depends on successful replication and lag. Control-plane quorum, ingress failover and application-volume consistency are separate concerns.
- Prefer guest-local/etcd-aware backups plus application backups. Host VM snapshots alone should not be presented as a complete coordinated Kubernetes recovery strategy.
- With three existing etcd servers, add an agent for workload capacity unless there is a specific reason to change control-plane membership; a fourth server increases quorum from two to three without increasing tolerated failures.

### Gaps
- No failover exercise, storage consistency validation, CSI compatibility test or disaster recovery test was conducted. The notes do not claim those operational properties are already proven.
