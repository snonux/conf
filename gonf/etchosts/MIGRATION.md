# Hosts and DNS migration (q14 preparation, 924 rollout)

## 924 source and deployment status

The normal hosts owners now use the `accepted` renderer: verified lab/Shelly
addresses are canonical, while AP management and DHCP-only JetKVM canonical
names retain their current addresses. Old aliases remain; recovery aliases
reserve the later physical recovery network and do not prove it is connected.
Native Pi ULA addresses are explicit `lan6` aliases only. WG rows are unchanged.

`fleetfiles` merges only fleet names and its marker block, preserving local
rows and unrelated aliases. Initial adoption requires an armed `endpointfiles`
hosts transaction. Existing owners cover f/r hosts, router, frontends and Earth;
new narrow owners cover the Pis and standalone guests. Generated frontend
hosts/DNS fragments come from the same renderer (`go generate ./etchosts`).
Explicit DNS records exclude the wildcard, whose address has a separate cache
gate. Pi-hole stages validated files outside its live mounts and is changed
one resolver at a time.

These are source changes, not a fleet acceptance claim. Task924 annotations
record native deployment progress. The historical q14 preparation below
describes the original sources and proposal; it must not be replayed as a
current deployment runbook. In particular, task824 has already accepted both
storage VIP addresses on the **same existing vhid**, retaining one owner.

## Historical q14 preparation

These sources prepare the allocation in IQ's
`topics/infra/openbsdrouter-plan/ADDRESS-RECOVERY.md` (2026-11-04).
They do not install files, contact hosts, change DNS or activate services.
Normal gonf hosts tasks still render their original canonical addresses.
No migration command is registered in an aggregate.

From `gonf/`, render to a private temporary review file:

```sh
go run ./cmd/hosts-migration > /tmp/hosts-staging.json
go run ./cmd/hosts-migration -profile target -router-view lab > /tmp/hosts-target.json
go run ./cmd/hosts-migration -profile ap-target -router-view wifi > /tmp/hosts-ap-target.json
```

`staging` preserves old canonical mappings and AP management 192.168.1.101.
`target` prepares canonical lab/Shelly changes while holding the AP old.
`ap-target` additionally prepares OpenWrt 10.30.0.2; use that delta only after
d24 proves the AP rollback and independent controller path. Rendering a target
does not assert those gates have passed. All cable changes remain at the end.
The controller's current old-LAN address 192.168.1.236, SSID/radio and DHCP
configuration are untouched; that observed lease is not a new static allocation.

Each JSON result contains a fleet `HostsBlock`, a separate unchanged
`WireGuardBlock`, Pi-hole dnsmasq lines, and an authoritative LAN zone fragment.
Choose `router-view upstream` for the controller/frontends/upstream view,
`lab` for lab clients, and `wifi` only for the accepted Wi-Fi stage. Router
aliases `triggerfish.upstream.lan`, `.lab.lan` and `.wifi.lan` always identify
the explicit zone addresses. Staging requires the upstream canonical view.
The misspelled existing `triggrefish.lan.buetow.org` alias is preserved.

Moved endpoints have explicit `<host>.old.lan` and `<host>.new.lan` names,
also under `.lan.buetow.org`. Allocated management recovery names are
`<host>.recovery.lan`; the VIP, AP, Shelly and JetKVM devices have no invented
recovery aliases. These are planned mappings, not proof of connectivity.
In particular, host recovery addresses belong to the later igc1 path and
must not be treated as usable shared-igc0 recovery. F/r and router native ULA
addresses use only `<host>.lan6` / `<host>.lan6.buetow.org`; no ULA is attached
to ordinary canonical names or inferred for other VMs/Pis. wg0 IPv4/IPv6
rows come unchanged from their existing inventory, including laptop extras.

The Shelly targets are Wi-Fi 10.30.0.28/.29. JetKVM names use authenticated
device IDs outside `*.f3s.lan.buetow.org`: current .248/.241/.244/.245 map
explicitly to lab .151/.158/.191/.198, respectively. No f-host attachment
is guessed. Their stable MAC reservations and device rollback belong to s14
and the router/device recipes, not DNS. Device current leases remain evidence
dated 2026-11-04 and must be rechecked before rollout.

## Source and destination ownership

| Affected destination | Current managed source / transition owner |
| --- | --- |
| f0–f3 `/etc/hosts` | `freebsd.Base.Hosts`, `f3s/freebsd-hosts/base/hosts.tmpl`, shared `lanHosts`; preserve FreeBSD stock header |
| r0–r2 `/etc/hosts` | `rnodes.Base.Hosts`, `f3s/r-nodes/base/hosts.tmpl`, shared `lanHosts`; preserve loopback `registry.lan.buetow.org` |
| rocky and freebsd/fbsd VMs | No shared hosts recipe currently owns them; snapshot actual `/etc/hosts`, identify unmanaged affected rows, merge the candidate fleet block only; keep guest-specific rows and existing management |
| pi0/pi1 (NetBSD), pi2/pi3 (Rocky) | No shared hosts recipe currently owns these files; inventory their actual rows before guarded adoption/merge, preserve OS localhost rows; resolver containers have separate ownership below |
| triggerfish `/etc/hosts` | `triggerfish.Base.Hostname` owns the current router row; future managed merge must reconcile that keyed row rather than append conflicting router canonical names; all changes through gonf |
| blowfish/fishfinger `/etc/hosts` | `frontends.Maintenance.WireGuardHosts` owns wg0 rows only; snapshot administrator-owned LAN rows and update only affected entries, preserve public IPv4/AAAA and frontend-local names |
| earth laptop/controller `/etc/hosts` | `workstation.System.Hosts` owns `# BEGIN GONF fleet`; `EarthHostsBlock`, `earthLAN`, shared inventory; replace that block at acceptance, preserve wg1/hyperstack and outside-marker entries |
| zen / other configuration controllers | `zen` is an unchanged explicit LAN inventory row; there is no managed workstation hosts block for it. Enumerate actual additional controllers and any static rows at deployment; preserve old-IP/AP reachability and do not infer their renumbering |
| pi2/pi3 dnsmasq | `pihole.Deployment.Compose` installs `f3s/pihole/docker-pi/dnsmasq.d/99-f3s-lan-wildcard.conf`; inactive `PiHoleDNSMasq` is the replacement candidate, not yet wired to that task |
| Authoritative buetow.org DNS | `frontends/var/nsd/zones/master/buetow.org.zone.tpl` → `frontends.MailDNS.NSD` → sole publisher blowfish; fishfinger is a zone-transfer secondary. Merge `AuthoritativeLAN` records into publisher input, never replace the complete zone or edit the secondary |

This is a repository source audit, not a claim that deployed hosts files were
read or changed. Unknown administrator-owned/controller rows require the
fresh deployment inventory. Existing managed sources must be changed in the
same accepted rollout so reapply cannot restore old canonical entries.
Do not copy the complete fleet block onto a full hosts file or append it next
to existing conflicting canonical entries. Preserve each OS header, all local
rows, public frontend AAAA, wg0 IPv6 and wg1 names. New explicitly named LAN
records must not replace unrelated public service records.

## DNS publication and cache sequence for the later guarded rollout

1. Snapshot deployed hosts/resolver configuration and authoritative answers,
   including current positive and SOA negative cache lifetimes. Current zone
   source uses `$TTL 4h` and SOA negative 1h; do not assume every cache uses
   those values. Before promotion, publish only the TTL reduction (60 seconds)
   for the affected existing wildcard through its current owner, validate the
   zone, and wait out the previous maximum positive TTL. Previously absent
   aliases also require negative-cache expiry; account for SOA TTL/minimum.
   No TTL changes are performed by this preparation.
2. Prove new endpoint, old/recovery access and service acceptance with armed
   rollback first. VIP canonical/wildcard promotion is coordinated with
   CARP/NFS/stunnel/relayd acceptance (824), never ahead of the storage/ingress
   step. Quiesce writers and remove the old CARP VIP from both owners before
   adding the new one; never create a split-subnet CARP pair or duplicate
   owners. Promote DNS/hosts only after NFS and relayd/TLS acceptance.
   AP canonical promotion waits for d24. Preserve old addresses and aliases
   until the separate retirement gate.
3. Merge accepted mappings into their managed sources, remove conflicting old
   canonical rows, validate candidates before activation, and change Pi-hole
   resolvers serially with the other available. Replace the existing wildcard
   record rather than append an additional A record: target wildcard is
   10.20.0.138. The authoritative fragment intentionally contains no SOA/NS,
   public frontend records, failover tokens or serial; publish with the sole
   existing publisher so its serial/validation/rollback contract remains intact.
4. Use native `dnsmasq --test --conf-file=<candidate>` and complete-zone
   `nsd-checkzone buetow.org <rendered-zone>` checks before reload. Flush only
   the affected caches where supported, otherwise wait for expiry. Check TCP
   and UDP direct answers from each resolver and authoritative primary and
   secondary; check client system resolution separately (`getent hosts`,
   `getent ahostsv4`, or the OS equivalent) to detect hosts/NSS shadowing.
   Exercise service wildcard, device-ID JetKVM, Shelly, old/recovery and explicit
   lan6 names; verify unchanged public AAAA and wg0 IPv4/IPv6 answers.
5. If acceptance fails, restore hosts plus DNS sources/serial publication and
   resolver running configuration together with the service/address rollback
   for the current cabling. Allow for clients retaining either answer during
   the TTL window. Restore normal TTLs only after complete acceptance; remove
   old/recovery aliases only through the later guarded retirement task.

Neither JSON fragments nor native parser checks prove live resolution,
reachability, device rollback, service health or current-cabling recovery.
