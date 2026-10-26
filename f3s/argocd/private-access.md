# ArgoCD private access migration (8n3)

**Rolled out 2026-10-26:** public `argocd.f3s.buetow.org` retired; UI is
`https://argocd.f3s.lan.buetow.org` only. Frontend relayd/gogios/acme/NSD
updated (fishfinger then blowfish); `just private-access` deleted the public
ingress and patched `argocd-cm` URL. LAN ingress UID
`226048a6-c4b5-4bce-8d88-c4949c462483` retained. Authoritative DNS NXDOMAIN
for the public name; public Host/SNI probes return relayd 403. Private LAN
UI and mesh `/healthz` remain 200. Baseline:
`~/.local/state/conf-worktrees/8n3-private-20261025/live/baseline/`.

Prepared 2026-10-25 from conf `6738ecd`. Owner selected LAN/WireGuard-only.
The staged approval included restoring the captured pre-cutover baseline
during the rollout window if acceptance failed. Procedure below remains the
reference for rollback.

## Intended boundary and scope

Use `https://argocd.f3s.lan.buetow.org`. LAN DNS resolves the CARP VIP
192.168.1.138; TLS remains on Traefik's `websecure` entrypoint using cicd's
existing `f3s-lan-tls`. Keep the existing LAN CA trusted on clients. On WireGuard,
route to a reachable Traefik HTTPS endpoint using this same TLS/Host name (for
example a client DNS override to its reachable r-node WG address). Do not assume
LAN VIP routing or DNS works through a roaming tunnel. Verify on the actual
laptop `.200` and phone `.201`; source-policy allowance alone is insufficient.
Traefik currently has one endpoint on r0; do not pin/move the workload to test it.

Changes are deliberately limited to:

- Delete cicd Ingress `argocd-server-ingress`; keep
  `argocd-server-ingress-lan` byte-equivalent in spec/annotations.
- Merge only `data.url` in `argocd-cm`; set Helm `global.domain` to the private
  hostname for subsequent normal upgrades. No Helm upgrade is needed now.
- Remove public ArgoCD and stale Pi-hole names from the frontend site inventory,
  including the Pi-hole expected-404 exception. This removes their generated
  relayd routes/keypair references, httpd blocks, public DNS records, ACME renewal
  entries, and public Gogios HTTP/HTTPS/certificate checks (including aliases).
  Existing certificate files are not deleted or revoked; private certificates
  and their renewal are unchanged. Retired public names may remain cached.
- Retain ArgoCD's existing Prometheus ServiceMonitors, rules and dashboard checks.
  Gogios continues importing those alerts. No replacement public UI probe is
  appropriate; private UI/CLI checks are explicit acceptance gates below.

The ArgoCD commit makes no changes to passwords, SSO, SSH, VPN peers/routes, NetworkPolicies, node
firewalls, storage, workloads, or Application revisions. Keep the cicd chart
policy overlay in both install and upgrade. Preserve Gogios 0640 permissions
and its protected cluster-down marker. No broad frontend aggregate deployment.

## Read-only preflight evidence

On 2026-10-25, public and LAN HTTPS returned 200. The LAN probe used `curl -k`,
so this establishes reachability, not client CA trust. The existing check_http
on fishfinger reached `/healthz` using SNI/Host private name and WG r0:443 (200).
The service is ClusterIP. Helm release 21 is chart 10.10.5, app v3.5.5;
`argocd-cm.data.url` still uses the public hostname. Five scoped cicd policies
remain present. LAN wildcard Certificate reports Ready=True. The public/LAN
Ingress UIDs are respectively `81137299-3f5c-44f6-b9c1-66a0bb3defe7` and
`226048a6-c4b5-4bce-8d88-c4949c462483`. No resource was changed.

Read-only `ip route get` on r0 returned `.200 dev wg0 src 192.168.2.120`,
but `.201 via 192.168.1.1 dev enp0s5 src 192.168.1.120`. Fishfinger's active PF
NAT applies on public `vio0`, not the WG-to-WG path. IQ describes roaming
clients peering with the gateways and the laptop's return via fishfinger.
Thus the expected path is client `.200`/`.201` → gateway WG forwarding →
r0 WG `192.168.2.120:443` → Traefik → ArgoCD, with a matching return route.
Live Traefik policy retains both exact source /32s. Laptop packet tests are
still pending; phone return routing is a concrete unresolved prerequisite.
The clients' documented public DNS resolvers also do not establish private
name resolution. Before withdrawing public access, verify private DNS/CA and
both devices; if repair is needed, propose that exact network change separately.
The companion routing proposal below addresses the phone prerequisite; client DNS and CA setup remain explicit device gates.

## Companion phone return-route proposal

The existing generator at `~/git/wireguardmeshgenerator` (origin
`git@github.com:snonux/wireguardmeshgenerator.git`, base `19279ea`) already
supports `reachable_via`; earth uses `fishfinger`. The isolated companion branch
`agent/8n3-phone-return-20261025` adds only `pixel7pro.reachable_via: fishfinger`
and a comment. Both phone address families are confirmed in inventory and the
live fishfinger peer: `192.168.2.201/32` and `fd42:beef:cafe:2::201/128`.
No keys, generated files, gateway configurations, or phone tunnels change.

After separate review and approval as part of this rollout, first stage only
r0/r1/r2: add those two exact prefixes to their existing fishfinger peer and
matching host routes, retaining every current AllowedIPs prefix (especially
laptop `.200`/`::200`), key, PSK, endpoint and keepalive. Capture the current
peer prefix set and route state privately before each host; use two independent
SSH transports and a bounded automatic rollback before activation. Use a
scoped live update plus the persistent wg0 configuration; do not run the
whole-mesh generator installer or restart wg0. Resolve each gateway peer by its
existing public identity without exposing private keys. A fresh live diff must
show only the phone prefixes/routes before each host is activated.

The generator's existing semantics will add these prefixes to fishfinger peers
on other infrastructure hosts on future regeneration too. That is a declared
source-level consequence, not authorization to deploy them now. No arbitrary
source ranges or new NetworkPolicy allowances are added. Rollback removes only
the two added phone prefixes/routes from the three changed hosts and removes the
new YAML field; preserve all existing laptop mappings and other concurrent edits.

Private cluster access on the phone requires **p7p-fishfinger** active. Its
p7p-blowfish tunnel remains selectable for internet access, but private cluster
return traffic cannot be symmetric through both gateways using the same client
address; this proposal does not claim to support private access through blowfish.
Do not delete or replace phone profiles or its unrelated tunnel.

Before removing the public ingress, on laptop and phone with the intended
fishfinger tunnel active: confirm the private hostname resolves to a reachable
WG HTTPS endpoint (currently r0 `.120`, not an unreachable LAN VIP), open
`https://argocd.f3s.lan.buetow.org`, verify the LAN CA is trusted and log in/read
applications. Confirm fresh gateway handshake and increasing counters for that
actual device, and no new public route. Test the laptop still works after the
phone route change. Public DNS resolvers in the phone profile are not proof of
private DNS: configure an existing trusted client DNS/override only after
identifying its available mechanism; do not publish a new public alias or alter
VPN defaults to work around this gate. Record device tests as pending until
actually performed; keep public removal gated on success.

Offline generator method checks on r0/r1/r2 confirmed that the only fishfinger
AllowedIPs additions are phone /32 and /128; blowfish mappings and both laptop
prefixes remain unchanged. The checks did not instantiate KeyTool or generate
keys/configurations. The original working tree's untracked cmd.sh is untouched.

## Staged rollout after approval

1. Recheck current source/live drift and both ingress UIDs, URL, endpoint and
   certificate health. Save the two ingress specs, the old URL and affected
   frontend configurations in a private rollback directory; avoid exporting
   whole Helm values or Secrets to logs. Record scoped policy UIDs, healthy
   metrics targets and application readiness. Establish working private UI/CLI
   access first, including actual roaming clients when available.
2. Review gonf plans for just `frontends_gogios`, `frontends_httpd`,
   `frontends_relayd`, `frontends_acme`, and `frontends_nsd`. NSD publication is
   restricted to the existing publisher workflow (blowfish); use the existing
   zone/serial/failover procedure, not manual zone replacement. Require diffs
   to contain only retired names and required serial updates. Secret-backed
   plans must stay private. Do not call `frontends_acme_invoke`.
3. On each frontend, deploy the reviewed Gogios/site/ACME changes and validated
   relayd/httpd configurations, standby first. Removing the shared inventory
   also removes public certificate references; retain certificate files for
   rollback. Check unaffected public services and protected monitoring files
   after each host. Config changes can restart relayd/httpd and affect existing
   connections; schedule accordingly. Complete the reviewed DNS publication.
4. From `f3s/argocd`, run `just private-access`. This applies the unchanged LAN
   ingress, merges only the URL, then explicitly deletes the old public ingress.
   Merely applying the smaller ingress.yaml would leave the public ingress live.
   Do not run `just upgrade`, uninstall, a broad sync, or a rollout restart.
5. Verify the private UI, `/healthz`, API version and normal authenticated CLI
   read using the trusted LAN CA. Confirm the ConfigMap URL, retained ingress
   UID, certificate, scoped policy UIDs and workload identities/readiness. Check
   Argo reconciliation via read-only Application status and metrics. Test actual
   LAN and laptop/phone WG clients; record unavailable device tests as pending.
6. From outside WG, probe each frontend's public IPv4/IPv6 with public ArgoCD
   Host/SNI and private Host overrides, including www/standby aliases. Expect
   no ArgoCD response: relayd's default block can produce 403, while the removed
   Traefik web route produces 404. Once DNS is removed, a normal lookup may fail;
   use `curl --resolve` to test the listener. With retired certificate references
   gone, TLS hostname verification may fail; a separate `-k` boundary probe must
   still be denied. Require unaffected public routes to succeed. Check two normal
   monitoring cycles for new critical/unknown/stale results.

## Rollback

Keep private access available. If only the URL change fails, restore only the
old `data.url`; do not roll back Helm release 21 or the deployed policy overlay.
If acceptance fails during the approved rollout window, restore public access
as part of that same approval; no second permission request is needed. The
approval explicitly accepts temporarily returning to the captured password-only
public exposure while the private-access failure is resolved. Reapply only the
saved public ingress and old URL, restore the two site entries and Pi-hole
exception from the previous source,
then redeploy only the corresponding validated frontend configuration and DNS
publication. Use retained public certificate files. Never restore whole older
Gogios/relayd configurations across intervening changes, or roll back unrelated
policy/permission fixes. Verify health and monitoring after any rollback, report the restored public
exposure, and leave the security finding open. This rollback authority is limited
to the captured baseline and the approved rollout window; it does not authorize
a later reopening after successful acceptance.


## Local validation

Go build, vet, errcheck and `go test -p 1 ./...` passed (recipe packages have
no tests; none were added). gofmt and git diff checks passed. Parsed YAML
comparison confirms the remaining ingress equals the previous LAN document.
Rendering pinned chart 10.10.5 with both values files gives the private URL
and no broad Helm NetworkPolicies. The existing network-policy suite passed
14 tests with two optional checks skipped; its chart-render test was then
rerun with the local chart and passed. The optional saved-OpenAPI-schema check
was not run. `just --dry-run private-access` expands to exactly the three
listed resource operations. No live dry-run admission request or mutation was
performed; authenticated client acceptance and native frontend config validation
remain rollout gates.
