# Duplicate Pi goprecords uploads (xn3)

Prepared on 2026-10-22 and independently reviewed on 2026-10-23. The user
approved source `dcde1d2` and the serial pi1, then pi0 rollout. Both live
cleanups, convergence checks and the next ordinary scheduled-upload acceptance
passed on 2026-10-23. The receiver-wide race remains tracked separately as
goprecords eo3.

## Evidence before cleanup

Before cleanup, each Pi's root crontab had these two minute-17 jobs
(substitute its short hostname for `HOST`):

```cron
17 * * * * env GOPRECORDS_HOST=HOST /usr/pkg/bin/goprecords-upload-client.sh >/dev/null 2>&1
# BEGIN GONF Cron[goprecords-upload]
17 * * * * /usr/bin/env GOPRECORDS_HOST=HOST /usr/local/bin/goprecords-upload-client.sh 2>&1 | logger -t goprecords-upload
# END GONF Cron[goprecords-upload]
```

On both hosts the old `/usr/pkg/bin` script byte-matched the managed
`/usr/local/bin` script (2060 bytes, POSIX cksum 198191107). Their token files
are mode 0600. The duplicate clients used the same host identity and could
upload the same kind concurrently; the older job hid failures in
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

## Recipe change

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

## Approved rollout and live acceptance

Self-review and fresh expert ROUND1 passed with no findings. Five real
recipe/guard fixtures and the pinned library's cron-adoption contracts passed.
The approved `dcde1d2` source was compiled privately, with its isolated assets
selected through `GONF_CONF_ROOT`. The process ran from the canonical
`gonf/` directory so the official provider resolved existing secrets; none
were copied or linked into the worktree.

Fresh per-host dry runs preceded the serial applies: pi1 finished at
15:00:55 UTC, then pi0 at 15:02:04 UTC on 2026-10-23. The strict preview
initially refused the older remote gonf 0.25.0 without changing resources.
The normal dry runs upgraded each remote runner to the pinned 0.25.2 before
showing the expected cron update. No copy of the old runner was captured.

Each Pi retains root-only backups of its previous root crontab and retired
script in `/root/.local/state/xn3-rollout-20261023T1505Z` (directory 0700,
files 0600). Private byte comparisons proved that each resulting crontab was
exactly its previous contents minus the exact legacy line. All unrelated
root jobs and environment lines were preserved. Each old `/usr/pkg/bin`
client is absent; the managed client and token bytes are unchanged, and
the token remains root:wheel 0600. Uptimed configuration matched the source,
so neither daemon restarted; its PID was preserved on each host. Both
scoped reapplies reported zero changes and zero pending changes, with the
absent-path retirement guard skipping further removal.

The next ordinary :17 cycle passed the acceptance gate. The observation
window was 15:16:30–15:18:30 UTC, checked after it closed:

| Host | Records PUT completed (UTC, 2026-10-23) | Records requests | HTTP | Final cpuinfo.txt PUT completed (UTC) |
| --- | --- | --- | --- | --- |
| pi1 | 15:17:00.668218 | 1 | 204 | 15:17:02.263918 |
| pi0 | 15:17:01.032205 | 1 | 204 | 15:17:02.657279 |

All ten requests across the two clients returned 204, with no duplicate
records PUT and no 400. Only derived host/kind/status/timestamp metadata
was retained locally; raw application logs, root crontabs and token contents
were not printed or saved locally. No production upload was forced. Other
users' crontabs, server authentication keys, application/storage settings,
SSH, reboot and power policy were untouched.

The receiver-wide concurrency defect still belongs to goprecords eo3;
historical 400 attribution retains the evidence limits described above.
Do not restore duplicate jobs as routine rollback while that race remains
unfixed. The cleanup does not depend on completing the entire ao3 NFS HA
redesign and does not authorize CARP tests, reboots or SSH policy changes.
