# Duplicate Pi goprecords uploads (xn3)

Local preparation dated 2026-10-22, refreshed 2026-10-23. Fresh independent
review passed on 2026-10-23; live deployment awaits approval. Both pi0 and pi1
need the cleanup. No live cron, script, token, daemon, upload or service changes
have been made for xn3.

## Current evidence

Each Pi's root crontab has these two minute-17 jobs (substitute its short
hostname for `HOST`):

```cron
17 * * * * env GOPRECORDS_HOST=HOST /usr/pkg/bin/goprecords-upload-client.sh >/dev/null 2>&1
# BEGIN GONF Cron[goprecords-upload]
17 * * * * /usr/bin/env GOPRECORDS_HOST=HOST /usr/local/bin/goprecords-upload-client.sh 2>&1 | logger -t goprecords-upload
# END GONF Cron[goprecords-upload]
```

On both hosts the old `/usr/pkg/bin` script byte-matches the managed
`/usr/local/bin` script (2060 bytes, POSIX cksum 198191107). Their token files
are mode 0600. The duplicate clients use the same host identity and can
upload the same kind concurrently; the older job hides failures in
`/dev/null`.

The application logs contain these adjacent records-upload outcomes on pi1:

| UTC on 2026-10-22 | Host/kind | HTTP | Duration |
| --- | --- | --- | --- |
| 09:17:01.150012 | pi1/records | 204 | 19 ms |
| 09:17:01.154188 | pi1/records | 400 | 27 ms |

This pair precedes the 09:35 NFS failover incident (ao3). The later global
upload 500s were investigated in pn3 and must be distinguished from the
intermittent 400s here.

A read-only refresh on 2026-10-23 found the current `goprecords:0.6.1` pod
Running and Ready. Four ordinary hourly cycles produced 16 records requests:
eight successful HTTP 204 responses for each Pi, still two per Pi per cycle.
These observations supersede the assumption that the daemon remains broken
after startup; they do not prove the duplicate schedules have been removed.
No production upload was forced. Derived status/timestamp metadata is retained
outside Git with xn3's rollout evidence; no raw logs or credentials were saved.

## Receiver race

At goprecords source `65d98c0`, `internal/daemon/upload.go:writeUploadBody`
creates **the same `path + ".tmp"` file for every upload to one host/kind**.
Concurrent writers truncate/share it. After one renames it to the final
path, the other can mutate that accepted file through its open descriptor
and then fail its own rename with ENOENT, returning HTTP 400.

A controller-only reproduction used the actual daemon, fresh temporary keys,
loopback HTTP and two authorized generic-host uploads. Each sent 4096 bytes
of its 65536-byte A/B body, then paused. Inspection of the owned local
process's descriptors confirmed both had opened the shared temporary path.
Finishing the first request then the second produced:

- HTTP 204 followed by HTTP 400, with a missing-temp-file rename error.
- Mixed bytes in the first accepted payload.
- Further changes to that accepted file from the second writer.

This is a confirmed receiver data-integrity defect, tracked separately as
**goprecords eo3**. Its fix needs per-request temporary files and synchronized
generic regression tests. Removing the duplicate cron entries mitigates the
known Pi trigger; it does not fix receiver concurrency for all clients.

The historical production response body was not retained, so the precise
cause of every old 400 cannot be proved. The exact duplicate jobs, matching
400/204 pair and deterministic matching receiver failure strongly support
this explanation for the observed pair.

## Prepared recipe change

`netbsd_goprecords_upload` now supplies the exact old command to
`WithLegacyCommand` on the existing present `CronAt` resource. Gonf's pinned
v0.25.2 adoption runs even when the managed block already exists; it removes
matching unmanaged entries while preserving other commands and managed
blocks. No raw crontab editing is needed.

The exact `rm -f -- /usr/pkg/bin/goprecords-upload-client.sh` command depends
on that cron resource. Its read-only guard also requires the path to exist
and the root crontab to be readable with no remaining reference to that
path. This preserves the old script if adoption is refused (for example,
because another cron block is malformed) or another job still uses it.
Even a comment referencing the path conservatively prevents deletion;
inspect any skipped cleanup rather than deleting the script manually.
An already absent script skips removal on subsequent applies.
The managed client, token path, minute-17 schedule and logging command stay
the same. Existing optional-token policy is preserved: a missing/empty
controller token declares neither adoption nor legacy file deletion;
provider failures abort recording. Other users' crontabs are outside this
resource's scope.

Five fixture tests record the actual recipe with dummy secrets and check
adoption metadata, deletion dependency/order, missing/empty token preservation
and provider-failure refusal. The native shell guard also checks remaining
references, failed crontab reads and already absent files against temporary
fixtures. Generic cron adoption/merge behavior is tested
in gonf's own package rather than adding library tests under conf/gonf.

```sh
python3 -m unittest discover -s f3s/pi-netbsd/tests -p test_goprecords_plan.py
cd gonf
go test github.com/snonux/gonf/resource/cron \
  -run 'TestAdoptLegacyCommand|TestAdoptUnmanaged|TestApplyMocked' -count=1
```

## Approval and live acceptance still required

1. Self-review and fresh expert ROUND1 passed with no findings. Five real
   recipe/guard fixtures and the pinned library's cron-adoption contracts pass;
   the live gates below remain outstanding.
2. Obtain explicit approval for the pinned recipe's serial pi1, then pi0
   rollout, including adoption of the two exact old root commands and removal
   of only `/usr/pkg/bin/goprecords-upload-client.sh` on each host. Inspect
   actual per-host plans first. The task's uptimed prerequisite may restart
   uptimed if its configuration has drifted; include that possibility in
   approval rather than assuming a broad aggregate is harmless.
3. On each Pi, verify the token is still private, the managed script exists,
   the legacy script is absent and the root crontab contains one managed
   minute-17 job with no exact unmanaged duplicate. Compare the remaining
   crontab privately so unrelated jobs/environment are preserved. Reapply
   once to verify convergence. Do not recreate the old job as a rollback
   while the receiver race remains unfixed; stop on a failed adoption.
4. After cleanup, observe ordinary :17 uploads from both Pis. Require one
   records request per Pi and successful
   payload completion (204), fresh timestamps and no 400s. A readiness 200
   or persisting 500 cannot satisfy this acceptance. No forced production
   upload, app restart or storage change is implied by the cron cleanup.
5. Track receiver-wide prevention in goprecords eo3 and keep xn3 open until
   its own reviewed cleanup and real scheduled-upload gates pass.

The cleanup has no dependency on completing the entire ao3 NFS HA redesign.
Its upload-health acceptance still requires a new ordinary scheduled cycle
after cleanup; the successful preflight uploads do not satisfy that gate.
Do not widen this task into CARP tests, reboots or SSH policy.
