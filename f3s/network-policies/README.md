# Namespace ingress isolation (task 7n3)

Deployed on 2026-10-24: all eleven namespace stages are active. The laptop and phone requirements add one scoped
phone read-proxy policy, making 55 policies across eleven namespaces.
Each namespace is a separate rollout stage. One task-7n3 approval may authorize
a named, bounded sequence with the gates below, or only a canary. There is no
all-namespace installer. SSH authentication and recovery boot
configuration are outside this task.

The policy set uses **Ingress only**. It leaves egress unrestricted. Replies to
allowed connections remain allowed; these policies do not restrict external
connections initiated by applications, including feed fetches, Syncthing relays,
Proton Mail, Git remotes or CronJob publishing. New receiving workloads start
closed until their necessary sources/ports are reviewed.

## Evidence and allowed paths

Read-only inventory on 2026-10-22 found eleven namespaces, 58 pods, 57 Services,
41 Ingresses, three CronJobs and fourteen retained Jobs. Pod counts and addresses
change; policies use stable labels, never pod names, addresses or template hashes.
`tests/inventory.json` retains selected labels/ports and resolved Ingress backend
ports from that inventory, without config, credentials or environment values.

| Namespace | Receiving paths preserved |
|---|---|
| `default`, `kube-node-lease`, `kube-public`, `test` | No resident pods or pod-backed receiving services; default deny only. The default Kubernetes API Service targets host API servers and is outside pod ingress isolation. |
| `player-trash-test` | Traefik in kube-system → its existing player backend on 8080. This is an active test deployment, not an empty namespace. |
| `infra` | Traefik → pkgrepo **8080** (Service port 80); registry **5000** behind NodePort 30001 through its existing node paths. |
| `services` | Traefik → the nineteen web backends at their actual endpoint ports; Immich server → its Postgres 5432, Valkey 6379 and machine-learning 3003; Miniflux server → its Postgres 5432; ArgoCD repo-server in cicd → Forgejo 3000; Prometheus → goprecords 8080. Existing Anki, Forgejo SSH and Bridge NodePorts keep the node paths described below. |
| `monitoring` | Prometheus → Alertmanager 9093 and reloader 8080, operator 10250, kube-state-metrics 8080, Pushgateway 9091, its own reloader 8080; Alertmanager → its own TCP/UDP mesh 9094; host API servers → operator webhook 10250. The existing 6n3 policies remain separately owned and unchanged. |
| `cert-manager` | Host API servers → webhook 10250; Prometheus may read its three exact metrics endpoints on 9402. The current self-signed CA issuers and LAN wildcard certificates have no HTTP01 solver; new ACME/solver modes need review. |
| `cicd` | Traefik → ArgoCD server 8080; server/controller/repo-server → Redis 6379; server/controller/ApplicationSet → repo-server 8081; Prometheus → controller 8082, repo-server 8084 and server 8083. No active ApplicationSet webhook Ingress or rollouts ServiceMonitor was found. See the mandatory chart-policy transition below. |
| `kube-system` | Pods in all namespaces → CoreDNS TCP/UDP 53; Prometheus → CoreDNS 9153 and Traefik 9100; host API servers → metrics-server 10250; frontends/LAN relays/ServiceLB → Traefik web ports, scoped below. |

Web ports use the destination container/endpoint port, not the Service port:
Forgejo 3000, pkgrepo 8080, koreader 17200 and opodsync 8081 are examples.
Traefik is scoped by **both** its kube-system namespace and workload labels.
Database allows select the owning app within services: an unrelated services
pod, another app, Traefik or a matching label in another namespace cannot use them.
Wallabag and Forgejo have no separate DB Service in this inventory; no invented
Wallabag database allowance is added. Immich's global DB environment variables
also appear on its ML/Valkey containers; that alone is not evidence that these
components need Postgres access. Only the server is a DB/Valkey client.

Read-only `/proc/net/nf_conntrack` metadata on r0/r1/r2 corroborated Miniflux's
5432 connection, Immich's Valkey connection, ArgoCD Redis traffic and Prometheus
scrapes/Alertmanager delivery. Snapshots miss short-lived, daily and manual
traffic; recheck the table and application workflows before each stage.

Services CronJobs are `beets-art`, `shuriken`, `shuriken-sync`; retained Jobs
also include the k3s Traefik installers. They have no receiving Services and need
no new ingress allowance. Their NFS volumes and outgoing publication paths are
unchanged. Syncthing's existing connection to earth on 22000 is initiated by the
pod and remains usable. Its GUI remains reachable through Traefik; arbitrary
incoming pod access to the data port is not granted.

Pushgateway `/metrics` contained no `push_time_seconds` or
`push_failure_time_seconds` groups. Through the existing authorized frontend read
proxy, both `push_time_seconds` and `max_over_time(push_time_seconds[10d])`
returned no series. Its r2 local CNI connection is consistent with the configured
10s readiness and 20s liveness HTTP probes. No production writer is evidenced;
authenticated Kubernetes port-forward remains the manual push path. Repeat both
checks before the monitoring stage and stop if any producer needs identifying.
Do not add an API-server exception to Prometheus merely for diagnostic access:
a service-proxy GET to its direct 9090 endpoint was denied during this inventory.

## NodePort, node and host-network limits

NodePort destinations use **pod ports**: registry 5000 (30001), Forgejo SSH 2222
(30222), Anki 8080 (30800), Bridge IMAP 143 (30143) and SMTP 25 (30025).
These Services currently use `externalTrafficPolicy: Cluster`. Conntrack showed
frontend WG sources 192.168.2.110/.111 connecting to registry/SSH/Anki on all
three nodes, and node/CNI sources after translation. Existing probes/clients must
remain usable; exact r-node LAN/WG and CNI gateway addresses are allowed only to
those receivers. Earth/laptop WG `192.168.2.200` and phone WG `192.168.2.201`
are explicit direct clients. Both are also admitted to Traefik/ServiceLB web
ports. This is network reachability only: the separate HTTP-header trust still
admits only the two frontends. Neither device gets database or raw Prometheus
receiver access. Both identities were confirmed in the actual gateway peer
configuration. The laptop profile is inactive; phone-side packets have not been
tested. The existing Bridge tunnel is a Kubernetes port-forward, with IMAP and
SMTP STARTTLS checked against its pinned certificate. Direct frontend WG access
to Bridge on its receiving node is intentionally absent from the allowlist. No services-wide,
namespace-wide or pod-CIDR allowance is used to keep these paths alive.

The exact trusted-node addresses are r0/r1/r2 LAN 192.168.1.120–122, WG
192.168.2.120–122, and CNI/flannel 10.42.{0,1,2}.{0,1}. NodePort translation
can replace an external client's source with one of these identities. These
exceptions **do not enforce the original external-client allowlist**.

Read-only r0 NAT rule tracing made the ordinary-pod bypass concrete: each
`KUBE-NODEPORTS` entry for 30001/30222/30800/30143/30025 jumps to a `KUBE-EXT`
chain that unconditionally calls `KUBE-MARK-MASQ`, then its Service/endpoint
DNAT chain. `KUBE-POSTROUTING` sees mark 0x4000 and runs MASQUERADE. On r2,
kube-router's active receiver rules match **source** ipsets in the forwarding
path; the existing proxy source set contains only .110/.111/.200, as expected.
For a **cross-node** NodePort path, translation occurs on the entry node before
policy evaluation on the remote destination node. An ordinary pod can therefore
address an r-node NodePort and arrive at its remote receiver with an admitted
node/CNI source, bypassing the direct-Service source restriction. For example,
an r0 pod can target r0:30001 while the registry endpoint is on r2, producing
a node/CNI identity that the registry's necessary source exception admits.
Same-node evaluation can differ because FORWARD precedes local POSTROUTING;
do not infer one path's behavior from another.

This is a **rules-derived bypass**, not an executed packet probe: no ordinary
probe pod was created in preparation. The tests explicitly retain the distinction
between a rejected ordinary source and an admitted rewritten CNI source. It
applies to registry, Bridge, Forgejo SSH and Anki NodePort receivers, not just
an unspecified external client. Approved runtime acceptance must exercise
Service/Pod and cross-node NodePort paths separately and record the residual
bypass. A subsequent NodePort/source-preservation/host-control redesign is
needed for stronger receiver isolation; this ingress-only task does not change
those Services or introduce egress policy.

**Database isolation is separate:** neither Postgres 5432 nor Immich Valkey/ML
is exposed by these NodePorts. Their policy rules admit only the owning server
pod selector, with no node/CNI ipBlock. The original unrelated-pod → database
Service/Pod negative test on all nodes remains mandatory and must pass.

Traefik and its ServiceLB accept only their web ports from the two frontend WG
addresses, the f0/f1 LAN relay addresses 192.168.1.130/.131 (and CARP VIP .138),
and the existing node/CNI paths. Traefik's web allow also admits the narrowly
selected kube-system ServiceLB pods. Prometheus alone gets its metrics port.
This is network access permission, **not** HTTP-header trust. The separate
4n3/5n3 forwarding-trust/Local-service rollout must be re-inventoried before
activating kube-system policies; no additional forwarded-header trust is added.

Kubernetes allows local-node/pod traffic; host-network semantics depend on the
implementation. The three node exporters use hostNetwork and listen on 9100;
these manifests do **not** isolate them, the API servers, kubelets, etcd or host
services. An authenticated port-forward bypasses pod ingress policy. Restrict
node access and Kubernetes exec/port-forward permissions independently.
Ordinary workload labels are not a cryptographic identity: anyone able to create
pods with trusted labels in a trusted namespace can impersonate that source.

Read-only r0 inspection found 91 `KUBE-NWPLCY` and 145 `KUBE-ROUTER` references
in the filter rules; no `--disable-network-policy` flag was present in k3s's
running ExecStart. This confirms a programmed controller, **not** packet proof
for these new policies. Per-node packet acceptance is pending approved rollout.

## Preserve the deployed Prometheus boundary

Read [the 6n3 exposure runbook](../prometheus/exposure.md) before monitoring.
These manifests add no raw receiver allowance. The separately managed
`prometheus-api` admits only same-namespace proxy/self pods on 9090, and
`prometheus-read-proxy` admits .110/.111/.200 WG sources on 8080. A separate
`prometheus-read-proxy-phone` policy adds only the authorized phone .201 on
8080, without changing ownership or bytes of the deployed 6n3 policies.
The additional
Prometheus-reloader rule permits self only on 8080. Prometheus scraping other
components does not imply those components may contact its receiver.

Policies are additive: test the **union** with 6n3 and all active policies.
Never add a monitoring-wide allow, all-pod metrics range, API-server receiver
exception or general services → monitoring allow. The ordinary-pod denials and
proxy source checks in the tests cover that union. Node/hostNetwork exceptions
remain outside those claims.

## Local checks

The tests are an offline selector/port model and local Helm/schema validation.
They create no pods, connections or writes in the cluster. Python needs PyYAML;
the optional native schema check also needs jsonschema. The cluster OpenAPI GET
below only saves public API schema, not workload data.

```sh
helm pull https://github.com/argoproj/argo-helm/releases/download/argo-cd-10.10.5/argo-cd-10.10.5.tgz --destination /tmp
kubectl get --raw /openapi/v3/apis/networking.k8s.io/v1 > /tmp/7n3-networking-openapi.json
ARGOCD_CHART=/tmp/argo-cd-10.10.5.tgz NETWORK_POLICY_OPENAPI=/tmp/7n3-networking-openapi.json /usr/bin/python3 -m unittest discover -s f3s/network-policies/tests -v
```

Without the two environment variables, the offline reachability tests still run;
the chart and native-schema checks explicitly skip. Run the complete command
for review. No Go tests are added under gonf.

## Staged approval and rollout

Application templates are `.yaml.disabled`, have no automated sync, no finalizer
and only one namespace source path. The generic recursive Application install
therefore skips them. **Do not rename/apply all templates or recursively apply
the policies.** Publication must be included in the approval; publish only an
isolated reviewed commit, never evolving main. Pin the selected Application's
`targetRevision` to the exact approved published commit before applying it.
Copy its disabled template to a temporary filename ending `.yaml`; do not
activate other templates in the repository. Manual sync must name exactly that
Application. Each namespace is manually synced and checked before the next approved stage.

1. Refresh read-only inventory and policy specs. Check current labels, Ingress
   backend ports, metrics targets, Jobs, external clients, node/CNI source paths
   and existing policy ownership. Stop for unexpected differences. Record
   readiness, restarts, Prometheus target/rule health and two normal Gogios cycles
   as the baseline; existing failing targets/Jobs are not new failures.
2. Obtain task-7n3 approval for the **explicit named stage sequence** (or a
   canary-only scope), its allows/default denies, manual Application creation/
   syncs, and the bounded acceptance probes below. A useful sequence is empty
   `test` first, `player-trash-test`,
   `infra`, `services`, `monitoring`, `cert-manager`, `cicd`, then `kube-system`;
   empty default/kube-public/kube-node-lease each get their own stage as well.
   The operator may change that sequence after inventory. System namespaces are
   not bulk enabled as a side effect of the application stages.
3. For the approved namespace only, reconcile explicit allows first (wave -1),
   then its default deny (wave 0). Each source file also puts the deny last for
   a specifically approved manual file apply. Adding a narrow allow begins
   isolating its selected workload immediately. Each receiver therefore has one
   complete allow policy containing all of its necessary source/port rules;
   its first selecting API write includes the full reviewed set. The offline
   prefix-apply test checks that transition. Verify the required paths after
   controller convergence; no blanket temporary allow is used, and no zero-gap
   availability claim is made for asynchronous rule programming.
4. Wait for kube-router rules on the destination nodes; inspect counters. Use
   bounded, explicitly approved ordinary probe pods with no hostNetwork, no
   trusted application labels and no privileges, on each r-node. Record the
   resolved service endpoints and timeout exit codes. Remove only these probes
   after capturing drop/reject counters; do not alter existing workloads.
5. Verify every approved positive path: both public and LAN web routes with real
   login/read workflows; owning app DB reads/health and Immich ML; Argo git fetch/
   refresh, Redis and UI in cicd; registry's existing clients;
   Anki/Forgejo/Bridge users and Gogios NodePorts; DNS TCP/UDP, cert-manager
   webhook/issuance and metrics API when their stages are reached. Do not create
   a production certificate, write mail, push image/data, or force a CronJob just
   to test a TCP policy. Approve a bounded functional write separately if needed.
6. For services, unrelated pods on **each node** must fail both ClusterIP and
   actual Pod-IP TCP access to immich-postgres:5432 and miniflux-postgres:5432.
   Wrong-app/wrong-namespace sources must fail; the existing owning clients must
   continue succeeding. Include Valkey and ML negative checks. For every stage,
   check a denied wrong port and cross-namespace source. For NodePorts exercise
   direct endpoint and hairpin paths separately and report any trusted-node
   bypass explicitly. Do not claim original-source isolation on Cluster policy.
7. For monitoring, repeat the complete deployed 6n3 positive/negative matrix:
   .110/.111/.200 proxy reads succeed on all three nodes; unrelated ordinary pods
   fail raw Service/Pod 9090 and proxy Pod/NodePort 8080/30090; disallowed LAN/WG
   sources fail. Retain the receiver flags and authenticated historic-import
   path. Verify all previously healthy metrics targets/rules, Alertmanager and
   Watchdog, plus two normal active/passive Gogios cycles. Check Pushgateway
   groups/history again before applying. All stages retain their baseline
   healthy targets and application Jobs; no restart is part of this rollout.
8. Record this namespace's policy specs, per-node allow/deny packet evidence,
   health comparisons and remaining limitations before continuing. Continue only
   to the next explicitly approved stage after every gate passes; stop on a failed
   gate, new scope or an unidentified necessary client. Ask for further approval
   only when the next action falls outside the approved sequence. API objects
   and the offline model alone are not task completion.

### Mandatory cicd chart transition

The active argo-cd 10.10.5 chart creates four policies. Its server policy contains
`ingress: [{}]`, and its metrics policies admit all namespaces. Default deny
cannot override these additive allows. The replacement cicd policies therefore
require a coordinated Helm transition:

1. During cicd's approval window, apply/sync its explicit policies and default
   deny first. Replacement server/Redis/repo policies have distinct `-scoped`
   names so they coexist with the Helm-owned originals; otherwise Helm would
   delete the replacements during its removal step. Leave the chart's old policies until the replacement allow rules
   are present; verify Argo's required paths.
2. Upgrade the **same pinned chart/version and existing values** with the extra
   `-f f3s/network-policies/cicd-chart-values.yaml`. The overlay disables all
   chart-generated policies; Helm removes only the four chart-owned policies.
   Do not steal their ownership or delete unrelated policies. Check the planned
   Helm diff for unrelated runtime changes and stop if any exist.
3. Retain the overlay on **every subsequent** ArgoCD install/upgrade. The ordinary
   `f3s/argocd/Justfile` includes it in both install and upgrade commands as of
   commit `d6687e8`, published before the approved Helm transition. The retained
   `cicd-activation.patch` updates older operator checkouts; its applicability
   test also recognizes an already-applied patch. This is mandatory: a plain
   upgrade with existing values otherwise recreates the allow-all policy.
   Do not run that plain workflow between activation and the persistence change.
4. Inspect the live union: only the new scoped cicd policies must remain, then
   prove unrelated pod → Argo server/Redis/repo-server denial and required UI,
   git, controller/cache and metrics success.

The overlay does not authorize an unrelated Helm
upgrade before cicd's stage. Local chart rendering confirms the current broad
server policy exists and that the reviewed overlay emits no chart policies.

### Recovery within an approved stage

Keep authenticated kubectl access and an independent operator session. These
policies do not change SSH or recovery boot. On a new failure, stop the sequence
and inspect selectors, endpoint ports, NAT and counters. Prefer a reviewed,
precise missing allow. If the approved window includes availability rollback,
remove only this stage's policies (the selected namespace source file), restoring
its prior policy set; also restore cicd's prior Helm settings if that transition
was part of the stage. A no-finalizer Application deletion alone leaves policies
in place. Removing only default deny leaves selected receivers isolated by their
allow policies, so it is not a complete rollback. Never remove deployed 6n3
policies, open Prometheus 9090, delete storage, reschedule pods or restart hosts
as policy recovery. Report any rollback as restored baseline, not successful
isolation.

References: [Kubernetes NetworkPolicy semantics and node exceptions](https://kubernetes.io/docs/concepts/services-networking/network-policies/),
[Kubernetes Service source-IP behavior](https://kubernetes.io/docs/tutorials/services/source-ip/).

## Live acceptance on 2026-10-24

All eleven manual Applications are pinned to published policy commit `a1112b9`.
The live union contains exactly 55 matching task policies and the two existing
6n3 Prometheus policies, whose UIDs and specs were preserved. Application
synchronization/health and the namespace policy specs were checked after each
stage. At this original acceptance point, co3 still owned the NodePort bypass;
its later completed guard rollout is recorded below.

- Each namespace had three temporary listening receivers, one per node. Six
  ordinary clients in test/services made 18 fresh connections before activation
  and were denied in all 18 cases after activation: 198 positive baseline cases
  and 198 actual denials across the eleven stages. The initial test-stage native
  REJECT counters increased on all three receiving nodes.
- Services: 48 unrelated-client attempts to Service and Pod addresses for both
  Postgres instances, Valkey and ML were denied. The actual Immich/Miniflux
  containers retained eight positive connections; two wrong-app attempts failed.
  Another 48 direct Anki/Forgejo SSH/Bridge receiver attempts were denied, and
  infra had 24 direct Service/Pod denials.
- Monitoring: 66 unrelated-client Service/Pod/NodePort attempts were denied.
  Both real frontend WG sources retained their existing registry, Forgejo SSH,
  Anki and read-proxy access. Each final gateway matrix had 24 TCP and twelve
  read-only HTTP cases. Current and ten-day Pushgateway group counts were zero.
- Argo chart 10.10.5 stayed at the same version, release revision 20 to 21. The
  native server dry run changed only removal of the four old chart policies;
  the five replacements kept their UIDs. No hooks or workload-template changes
  ran. Ten fresh connections from real Argo components, 36 unrelated-client
  denials and an actual successful hard refresh verified cache/repo/GitOps paths.
- DNS returned real A answers over both UDP and TCP from all six clients.
  All four existing Certificates remained Ready; server-side Certificate
  admission dry runs persisted nothing. The native node metrics API worked.
- All 55 baseline running pod UIDs/readiness/restart counts remained unchanged.
  All 43 healthy targets and 42 rule groups remained, including Watchdog.
  Fifty-nine TLS-verified public/LAN GET results matched baseline. Two additional
  forced standby paths (bgtutor-mcp and gpodder via fishfinger) already had
  certificate-name mismatches before the production policy stages and remain
  unverified; these were never counted as successful TLS paths.

The existing laptop Bridge port-forward passed pinned-certificate IMAP and SMTP
STARTTLS/NOOP checks without authentication or sending mail. An early test
incorrectly expected direct frontend WG access to Bridge; the corrected test
uses the reviewed receiver contract, without widening production policies.
Traefik keeps its existing single replica and Local NodePort: direct NodePort
checks use its ready endpoint node, rather than assuming all nodes host Traefik.
Anonymous direct/public requests retain their authentication challenge.

Device-side traffic from laptop `.200` and phone `.201` is not claimed tested:
the laptop profile is inactive and the phone was not available for those tests.
Their explicit /32 policies and positive/negative model cases are in place;
Traefik forwarded-header trust remains only `.110` and `.111`.

At the original 7n3 acceptance, the co3 bypass was also packet-observed: an ordinary r0 pod reached
r1 Cluster NodePorts 30001, 30222, 30143, 30025 and 30800 after activation. This
was a remaining limitation at that point, not a successful isolation result. No NodePort
traffic-policy redesign, application restart, storage change or host/VPN change
was performed for this rollout.

All 39 owned probes were removed with UID preconditions; the three operator SSH
masters were closed. Two distinct normal active/passive Gogios cycles were
observed after cleanup (10:20/10:21 and 10:25 UTC). The temporary probe CPU warning
cleared naturally. Final views both report critical/unknown/stale zero and one
pre-existing `KubeJobFailed` warning, with 608 OK entries. Final health still has
55 unchanged running workload pods, 40 healthy/synced Applications, 43 healthy
targets and 42 rule groups. No monitoring was muted and no failed Job was reset.

The f3s skill update is published as dotfiles commit `ab3a381` on both active
forges and installed in the canonical local skill checkout. Unrelated local
working files were preserved. All 16 policy tests passed, including native
OpenAPI validation and pinned chart rendering, with no skips. The original
fresh preparation review passed; necessary later roaming/Helm-name fixes were
self-reviewed and covered by the updated tests and native rollout evidence.

## Follow-up NodePort guard (co3, 2026-10-24)

The separate pre-NAT pod-source guard is now active, confirmed and boot-enabled
on r0, r1 and r2. It closes the observed ordinary-pod bypass of registry,
Forgejo SSH, Anki and Bridge NodePorts. All 180 final LAN/WG NodePort attempts
and 90 additional CNI-gateway attempts were denied. A real temporary Service
transitioned r0-only to r0+r1 to r1-only while ordinary clients stayed denied
and both gateways retained access through every node. The existing 57
NetworkPolicies retain their exact UIDs/specs; this adds no receiver allowance.

See [the co3 runbook](../docs/nodeport-pod-source-guard.md) for native packet,
client, rollback-timer, convergence and cleanup evidence. Existing non-pod
client SNAT and hostNetwork/node trust remain under sm3's separate scope.
Laptop `.200` and phone `.201` access is preserved; their actual device-side
WireGuard packets have not been tested while the tunnels are inactive.
Two normal post-cleanup active/passive monitoring cycles passed with no
critical, unknown or stale checks; only the pre-existing failed-Job warning
remains. No production workload restart or storage/SSH/VPN change was made.
