# Taskwarrior sync to Garage S3

The Fedora laptop runs stock Fedora `task` 3.4.2 and syncs to the Garage S3
cluster.

| | |
|---|---|
| Database | `~/.task/taskchampion.sqlite3` |
| Endpoint | `https://garage.f3s.buetow.org` |
| Bucket | `taskwarrior` |
| Wire hostname | `taskwarrior.garage.f3s.buetow.org` |
| Garage | `root_domain = ".garage.f3s.buetow.org"` on f0/f1/f2 |
| Credentials | `~/.config/garage/taskwarrior-sync.env` (0600) |
| Sync | `tasksync` (fish), or `AWS_ENDPOINT_URL_S3="$GARAGE_ENDPOINT" task sync` |
| Config | `dotfiles:taskwarrior/taskrc`, dotfiles gonf task `home_taskwarrior` (Linux only) |

## How it works

3.4.2 has no `sync.aws.endpoint_url` or `sync.aws.force_path_style` (both
landed upstream after v3.5.0). So:

- The endpoint comes from the AWS SDK variable `AWS_ENDPOINT_URL_S3`, set per
  invocation. Never export it: it would redirect every SDK client on the
  machine, and `~/.aws` has a live `[default]` profile.
- Without path style the SDK addresses `<bucket>.<endpoint-host>`. Garage's
  `root_domain` makes `taskwarrior.garage.f3s.buetow.org` the bucket. That
  name is an ordinary f3s host: DNS, Let's Encrypt cert and relayd keypair
  come from the same frontends loops, and relayd routes the subtree to Garage
  by suffix.
- New bucket hostname: add it to `garageBuckets` in `gonf/frontends/data.go`,
  then `./gonf.sh cluster frontends frontends`.
- Path style still works (the `watchos-app` consumer uses it). An IP endpoint
  such as `http://192.168.1.130:3900` gets path style automatically, LAN only.

`taskrc` holds no secrets; it expands `$GARAGE_*` and `$TASK_SYNC_SECRET`.

## Fleet

| Host | State |
|---|---|
| Fedora laptop | 3.4.2, syncs to Garage (the only 3.x replica, so sync is an encrypted off-machine backup for now) |
| macOS work laptop | 2.6.2, no sync; exchanges tasks via the JSON tag-routing channel in `~/git/worktime` |
| rocky VM | 2.6.2 built from source; JSON channel only |
| OpenBSD frontends | Taskwarrior retired |

Syncthing still replicates `~/.task` including the SQLite DB, by choice.
SQLite runs in WAL mode, so a mid-write capture can be inconsistent and a
conflict copy can't be merged. The old 2.x files are in `~/.task/v2-archive/`
(3.x warns on every command otherwise).

## Sharp edges

- 2.6.2 and 3.x round-trip via JSON export/import in both directions; only
  undo history is lost. Import is slow (17 min for ~18k records).
- Backups from the migration: `~/taskwarrior-2.6.2-backup-*.tar.gz`,
  `~/taskwarrior-migrate-*.json`.
- Phone client settings: `~/Notes/TaskwarriorPhoneSync.md`. The encryption
  secret is not an S3 setting; a wrong one makes sync look fine but show
  nothing.
- Garage admin on f0: SSH port 22 (not the `*.buetow.org` default 2), and the
  `garage` CLI needs doas (`garage.toml` isn't world-readable).
- Fallback if the vhost route ever has to go: the custom build with
  `force_path_style` at `https://code.f3s.buetow.org/snonux/temp-taskwarrior`.

Related: `f3s-workloads` skill `references/garage.md`,
`dotfiles:fish/conf.d/tasksync.fish`. Investigation record:
[`docs/archive/f3s/docs/taskwarrior-s3-sync.md`](../../docs/archive/f3s/docs/taskwarrior-s3-sync.md).
