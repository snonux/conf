# check_syncthing

Nagios/Gogios plugin that reports out-of-sync Syncthing folders through the
REST API. Go, standard library only: no curl, jq or other runtime packages.
It prints one summary line and exits with the Nagios code.

| Exit | When |
|---|---|
| 0 OK | every unpaused folder has nothing to pull, or matches an ignore rule |
| 1 WARNING | a folder still pulls (`syncing`, `scanning`, ... with need > 0) or has pull errors |
| 2 CRITICAL | a folder is stuck (`idle` with `needTotalItems` > 0), in state `error`, or a WARNING lasted `-escalate-after`; also a missing, empty or rejected API key (HTTP 401/403), so a broken key pages at once |
| 3 UNKNOWN | bad flags, bad ignore file, API unreachable, timeout, HTTP error (redirects are not followed, so the key never leaves for another host), bad JSON, or a state-dir error (unless already CRITICAL) |

```
SYNCTHING CRITICAL: 2 of 12 folders out of sync: Galaxy Tab S10 FE DCIM idle need=4 (20.2 MB) pullErrors=4, Sync with earth syncing need=273 (157.3 MB)
SYNCTHING WARNING: 1 of 12 folders out of sync: Sync with earth syncing need=332 (167.0 MB) out of sync since 0m; ignored: Galaxy Tab S10 FE DCIM idle need=4 (20.2 MB) pullErrors=4
```

## Usage

```sh
check_syncthing -url https://syncthing.f3s.buetow.org \
    -api-key-file /etc/gogios/syncthing.key \
    -ignore-file /etc/gogios/check_syncthing.ignore \
    -state-dir /var/db/check_syncthing -escalate-after 1h -timeout 8s
```

| Flag | Default | Meaning |
|---|---|---|
| `-url` | `http://127.0.0.1:8384` | GUI/REST base URL |
| `-api-key-file` | | file holding the API key, whitespace trimmed (wins over the env var) |
| `-api-key-env` | `SYNCTHING_API_KEY` | environment variable with the API key |
| `-ignore-file` | | known out-of-sync folders (see below) |
| `-state-dir` | | writable directory for `check_syncthing.json`; empty disables durations and escalation |
| `-escalate-after` | `1h` | a WARNING that lasts this long becomes CRITICAL; `0` disables |
| `-timeout` | `30s` | total time for all API calls |
| `-mode` | `folders` | `folders` (above) or `freshness` (below) |
| `-period` | `24h` | freshness: WARNING when no peer sent data for this long |
| `-device` | | freshness: `<device ID or name>=<duration>`, WARNING when that device was neither connected nor seen for longer; repeatable |
| `-version` | | print the version |

The API key is sent only in the `X-API-Key` header and never printed;
errors name the key file or variable, never the value. It is the `<apikey>`
in Syncthing's `config.xml` (config PV, see `f3s/syncthing/README.md`).

`/rest/db/status` is an expensive call (one per folder). Run the check every
few minutes, not more often. Right after a cluster start one run took 14 s
for 12 folders (later runs about 1.5 s), so keep `-timeout` below the
caller's own limit (Gogios `CheckTimeoutS` is 10 s on the frontends).

## Ignore file

One rule per line, `#` comments:

```
# <folder ID or label>[; max-items=<n>][; max-bytes=<n>]
Galaxy Tab S10 FE DCIM; max-items=4; max-bytes=20226448
# device: <device ID or name>   (freshness mode only)
device: Galaxy Tab S10 FE
```

A folder whose ID or label matches is reported as `ignored` and keeps the
check OK, as long as its `needTotalItems`/`needBytes` stay within the
limits. If it falls further behind, or enters state `error`, it alerts
again. The rule above is the known case from `f3s/syncthing/README.md`:
four photos whose only source, the Galaxy Tab S10 FE, is disconnected.

A `device:` rule keeps that device out of the freshness mode: its bytes
do not count as received data and a `-device` rule for it never fires.

## Freshness mode

`-mode freshness -state-dir DIR` proves that Syncthing actually receives
data, which the folder mode cannot see (an idle folder is normally healthy):

- **Transfer:** each run stores every peer's `inBytesTotal` from
  `/rest/system/connections` in `DIR/check_syncthing_freshness.json`. Data
  arrived when a counter grew since the last run. The counter covers the
  peer's current connections only (0 while disconnected, lower after a
  reconnect or a Syncthing restart), so a counter that shrank, or a new
  peer, only sets a baseline. No data for `-period` is WARNING, and
  CRITICAL once that lasted `-escalate-after` more. The counters include
  protocol traffic, so in practice this alerts when no peer was connected
  for the period. A peer that connects, syncs and disconnects entirely
  between two runs is not seen.
- **Baseline:** the first run, and the first after a pause of more than 2h
  between runs (check paused during an f3s takedown, frontend failover),
  only records a baseline and reports OK, so stale state never alerts.
- **Peers:** each `-device` peer must be connected or have a `lastSeen`
  (`/rest/stats/device`) within its duration, else WARNING. An unknown
  device name is UNKNOWN.

```
SYNCTHING FRESHNESS OK: last data from a peer 12m ago
SYNCTHING FRESHNESS CRITICAL: no data from any peer for 49h00m (limit 24h00m) (escalated); Pixel 7 Pro not seen for 80h00m (limit 72h00m)
```

## Escalation state

With `-state-dir`, the check stores when each (not ignored) folder was
first seen out of sync, adds `out of sync since 2h05m` to its entry and
forgets it once the folder is in sync. An UNKNOWN run leaves the state as
it is. Gogios mails WARNING changes only at the 07:02 renotify, so a stuck
transfer pages through the escalation. Use a persistent directory owned by
the check's user (e.g. `/var/db/...`, not `/var/run`, which a reboot
empties and so restarts the clock).

## Build and test

```sh
mage test           # go test ./...
mage build          # ./check_syncthing
mage buildOpenBSD   # ./check_syncthing.openbsd-amd64 for the frontends
```
