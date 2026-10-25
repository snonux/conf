# f3 DTail repair and native acceptance — 25 October 2026

The owner approved the f3-only proposal at source **d340a9a**. That isolated
candidate was published to GitHub and Forgejo, then applied only to f3. The
other seven-BSD and Linux rollouts in task **3n3** remain unapproved/pending;
this report does not close the fleet-wide task.

## Deployed result

The existing volatile SSH identity was read as dserver into host-local,
root-private staging, checked against the admin-attested served fingerprint,
and installed in persistent `/var/db/dserver/ssh_host_key`. The key stayed on
f3 throughout. Its fingerprint is unchanged:

`SHA256:D45UZ7edwGAk4FCtxdxAuIFpwrYnb+AG0QQU2c/9E+Y`

The original root-controlled binary/config/helper/rc files and staged identity
were retained privately for rollback. The old daemon-owned runtime was renamed
into root-only quarantine on the same filesystem. No recursive root repair or
old vulnerable helper was used.

The verified existing f0 FreeBSD/amd64 static binary replaced only
`/usr/local/bin/dserver` on f3, as root:wheel 0755:

`3f146b14d293df15ff019817e30f6be4f3bbf88f2bc60e5ca3bd4683cb5f8573`

The scoped Gonf `freebsd_dtail_config` applied five changes through normal
SSH/doas: safe helper, protected cache refresh, host-specific log-only config,
startup rc hook and dserver start. It performed no package/account operation,
cluster aggregate, system SSH/PF/routing change, guest operation or reboot.
DTail remains enabled; the final observed daemon PID was 17239.

Its only TCP 2222 listener is `192.168.1.133`. Runtime/cache are root:dserver
0750 and the paul authorization file is root:dserver 0640. Persistent key/dir
are dserver:dserver 0600/0700. Log access is restricted to `/var/log/` and
`/var/www/logs/`; nonlog access is not reinstated for compatibility.

## Native data and negative checks

All payload comparisons used the unchanged existing laptop clients, strict
private host pins and the existing login keys. Production log payloads were
captured only in RAM and not printed or retained.

| Check | Native result |
|---|---|
| Current dcat, final sample | exit 0, 386,358 log bytes, first 4096 bytes match stable same-user OS prefix |
| Current dgrep `.` | exit 0, 386,358 bytes, prefix matches the same native filtered source |
| Three post-startup readiness reads | 3/3 exit 0 with stable matching OS prefixes and preserved public fingerprint |
| OS-readable `/etc/fstab` | exit 2, new native permission-denial record, only known diagnostic and no file payload |
| OS-readable disposable nonlog file | exit 2, new native permission-denial record, no marker or payload |
| Deliberately incorrect host pin | exit 1, no actual log prefix; global trust unchanged |
| Native cache access as dserver | user cache readable; symlink creation/cache write denied |
| Root-only synthetic sentinel | unchanged bytes, root ownership and 0600 after two safe refreshes |
| Fresh administrative SSH plus normal doas | successful in 0.184 seconds |

The file-denial oracle checks only a count of matching permission-denial
records, not raw server logs. It allows bounded time for buffered logging and
requires the client output to contain only the known generic read-failure
diagnostic. The initial test wrongly expected a literal permission message
from the client; the real server intentionally emits a generic warning. The
oracle was corrected using the native denial record and OS-readable source.
An initial immediate counter check also ran before log flushing; the final
bounded checks passed. These were test-harness failures, not accepted denials.

## Startup verification and observation limits

A second, approved dserver-only stop/start tested the installed startup path.
The protected runtime/cache were renamed into a separate root-only quarantine,
leaving them absent before `service dserver start`; no manual mkdir or cache
refresh occurred between removal and start. The hook recreated both directories
and paul's cache with the required ownership/modes. Distinct old/new inodes were
checked. The persistent fingerprint stayed unchanged. A subsequent Gonf preview
found no config/helper/rc changes or service restart; its sole would-run action
was the deliberately unconditional safe cache refresh.

The direct SSH observation of this startup exceeded its 40-second client
bound. The operation was not retried: the independent administrator channel
confirmed the new daemon, recreated cache and completed fingerprint check.
One initial post-startup payload assertion did not pass and its detailed result
was not retained, so its cause is not claimed. The later explicit readiness
gate passed three consecutive full log reads; final dcat/dgrep, denial and
wrong-pin checks also passed. This establishes the observed steady state,
not guaranteed successful reads at the instant a new daemon starts. No reboot
was performed as a test.

## Health comparison and owner-confirmed guest changes

Before and after the repair, all three Kubernetes nodes were Ready, all 54
non-Job application pods ready, all 40 applications Healthy/Synced and all 43
Prometheus targets up. Both Gogios summaries had zero critical, unknown or
stale checks. The pre-existing KubeJobFailed warning remained; the startup
K3sVersionCheckCoverage warning began clearing on the normal monitoring cycle.

The extra f3 guest-state comparison **did not pass**: at preflight, rocky was
running and freebsd stopped; afterward rocky was stopped and hfs-test0,
hfs-test1 and hfs-test2 were running. No VM-management command was part of this
repair. The owner subsequently confirmed these changes were intentional, then explicitly
requested that all FreeBSD guests be stopped and Rocky started again. The three
hfs-test guests were shut down gracefully; freebsd was already stopped. The
final inventory shows all four FreeBSD guests stopped and rocky running
(PID 28967), with strict SSH and normal sudo verified inside rocky. This resolves
the pending guest-state clarification. The historical comparison remains a
recorded change, not an unchanged-inventory pass. No guest was stopped/started
without the owner's new instruction. Cluster/monitoring health and DTail
acceptance above are recorded separately from that intentional difference.

## Remaining work and retained material

The executable is a temporary package-file override of `dtail-4.3.2-ng`;
pkg metadata/checksums were intentionally not altered. Task **mp3** owns a
proper distinct versioned FreeBSD artifact/package and eventual removal of this
override. Reinstalling the old same-name package can restore the old failure;
a later package upgrade must preserve this repair and repeat native data tests.
Application follow-up **lp3** in project dtail owns generic old-server/current-
client compatibility and false-success behavior. Neither follow-up authorizes
package publication or another production rollout by itself.

The disposable nonlog probe and local synthetic wrong-test private key were
removed after testing. Host-local root-private backups, the staged identity and
both runtime quarantines remain. The `/var/run` quarantines are volatile;
the root-private staging/rollback identity under `/var/db` is persistent.
No backup/key material was deleted or copied off f3. Phone/laptop WireGuard
device packets were not tested by these LAN checks.

Private controller receipts and operational scripts:
`~/.local/state/conf-worktrees/kp3-f3-read-20261024/live-20261025/`.
`migration-state.json` records exact backup/quarantine paths;
`native-acceptance.json`, `final-native-acceptance.json`,
`native-startup-acceptance.json`, `post-startup-repeated-reads.json`,
`f3-convergence-preview.txt`, `owner-restored-guest-state.json` and the pre/post
health receipts record the checks.
The failed guest comparison and startup observation are retained, not rewritten
as passing runs. Reviewed core code was unchanged; only this acceptance report
and links were added after deployment, so no new build/full-suite claim is made.
