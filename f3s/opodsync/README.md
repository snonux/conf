# opodsync (retired)

Task a04 retires the gPodder sync service at `gpodder.f3s.buetow.org`
and `gpodder.f3s.lan.buetow.org`. The chart is retained for recovery;
its ArgoCD Application is disabled in `../argocd-apps/services/`.
Do not apply that manifest or run deployment commands in the Justfile unless
intentionally restoring this service.

## Retained data

Keep `services/opodsync-data-pvc`, `opodsync-data-pv` (reclaim policy `Retain`)
and `/data/nfs/k3svolumes/opodsync/data`, including existing storage snapshots
and backups. Retirement does not authorize deleting any of these.
The chart's manual prerequisite is that this NFS directory exists.

## Retirement procedure

After publishing the disabled Application and the frontend/policy changes:

1. Orphan the Application's resources before deleting it. Its usual
   `resources-finalizer.argocd.argoproj.io` finalizer would delete the PVC/PV.
   Recheck that this is its only finalizer, then remove it and delete only
   `cicd/opodsync`:

   ```sh
   kubectl -n cicd patch application opodsync --type=merge -p '{"metadata":{"finalizers":[]}}'
   kubectl -n cicd delete application opodsync
   ```

2. Delete only its running application resources, retaining both volume objects:

   ```sh
   kubectl -n services delete deployment opodsync
   kubectl -n services delete ingress opodsync-ingress opodsync-ingress-lan
   kubectl -n services delete service opodsync-service
   kubectl -n services delete configmap opodsync-app-config opodsync-nginx-config
   ```

3. The live `network-policies-services` Application has no automated sync
   policy and is pinned to `a1112b97d273c67c5f8ca3257592d2e64fc698a3`.
   After publishing the reviewed retirement commit, record every services
   NetworkPolicy UID, and advance **only** this existing Application's
   `spec.source.targetRevision` to that exact published commit SHA. Use a
   JSON Patch with a test of the previous revision to detect concurrent edits:

   ```sh
   # Set approved_revision to the exact reviewed commit published to Forgejo.
   kubectl -n cicd patch application network-policies-services --type=json \
     -p "[{\"op\":\"test\",\"path\":\"/spec/source/targetRevision\",\"value\":\"a1112b97d273c67c5f8ca3257592d2e64fc698a3\"},{\"op\":\"replace\",\"path\":\"/spec/source/targetRevision\",\"value\":\"$approved_revision\"}]"
   kubectl -n services delete networkpolicy opodsync-web
   kubectl -n cicd annotate application network-policies-services argocd.argoproj.io/refresh=hard --overwrite
   ```

   Verify `status.sync.status=Synced`, both spec and status revisions equal
   the approved SHA, and every other services policy retains its original UID
   and specification. Do not sync other namespaces or run a broad policy apply.
   The disabled `network-policies/services.yaml.disabled` remains a manual
   activation template: its comment already requires overriding the revision
   with an approved published SHA. The live pin is an explicit operational
   override, avoiding a self-referential commit containing its own hash.

   Before frontend deployment, generate candidate configurations from the
   reviewed checkout and compare them with both live frontends. The only
   accepted semantic differences are removal of gpodder's records, renewals,
   routing and checks. Stop on unrelated drift; do not overwrite live user
   changes with a clean checkout's older configuration. The user-owned
   triggerfish inventory and hosts changes are excluded from this retirement.

   Deploy `frontends_nsd`, `frontends_acme`, `frontends_httpd`,
   `frontends_relayd` and `frontends_d_tail_checks` with the gonf frontend recipes.
   Despite its historical task name, `frontends_d_tail_checks` publishes only
   `/etc/gogios.json`, avoiding unrelated package/plugin/account updates from
   the full `frontends_gogios` task. Review the targeted remote preview for
   any incidental recipe resources before applying; stop on unrelated drift.
   Removing the service from `f3sHosts` removes public A/AAAA records for its
   bare, `www.` and `standby.` names, certificate renewal configuration,
   HTTP fallback/SNI configuration and monitoring checks. Existing certificate
   files can remain. The shared LAN DNS wildcard and LAN wildcard certificate
   remain for other services; the retired LAN host therefore still resolves,
   but has no service ingress.

4. Verify Application, Deployment, Pods, Service, both Ingresses, both ConfigMaps
   and NetworkPolicy are absent. Verify the PV/PVC remain Bound and their path
   is unchanged. Check both authoritative NSD servers return no A/AAAA answers
   for all three public names (the zone's wildcard MX can produce NOERROR/NODATA
   instead of NXDOMAIN). Record the SOA serial before publication, verify it
   increases on blowfish, then wait for fishfinger to serve the same new serial
   and absent records. Verify frontend daemons/configuration remain healthy.
   Test an unrelated service through each frontend and the LAN ingress.

## Recovery

Rename the Application back to `.yaml`, restore its `f3sHosts` entry and
`opodsync-web` policy, publish and apply the Application, then deploy frontend
configuration again. The retained Bound PVC is reused. Do not overwrite the
retained data or create an empty replacement volume.
