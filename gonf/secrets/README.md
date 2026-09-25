# Gonf controller secrets

Recipes read secrets only with `api.MustSecret` / `api.OptionalSecret` and
the logical paths from `gonf/paths`. `cmd/gonf/main.go` sets the provider:

```go
api.SetSecretProvider(secret.NewSnapshot(secret.NewFallback(vault, secret.FileProvider{})))
```

`vault` is foostore (KeePass). References in its `foostore.Items` table come
from the vault; everything else comes from files below this directory.

| Logical path | Vault entry (Password field) | File fallback / legacy copy |
|---|---|---|
| `paths.FrontendSecret("var/nsd/etc/nsd_key.txt")` | `Infra/nsd-tsig-key` | `gonf/secrets/frontends/var/nsd/etc/nsd_key.txt` |
| `paths.GarageSecret("rpc_secret")` | `Infra/garage-rpc` | `gonf/secrets/garage/rpc_secret` |
| `paths.FrontendSecret("etc/goprecords/<host>.token")`, blowfish and fishfinger | `Infra/goprecords-token-<host>` | `gonf/secrets/frontends/etc/goprecords/<host>.token` |
| `paths.FHostSecret("goprecords/<host>.token")` (`freebsd.GoprecordsToken`), f0-f3 | `Infra/goprecords-token-<host>` | none (imported from the hosts' `/etc/goprecords-upload.token`, task lk2) |
| `paths.FHostSecret("carp/vhid1.pass")` (`freebsd.CarpPassSecret`), f0/f1 | `Infra/carp-vhid1-pass` | none (imported from the hosts' `ifconfig_re0_alias0` in `/etc/rc.conf`, task gk2; rotated 2026-09-25, task 1l2) |

A frontend added later has no vault row and reads the file. Migrating another
secret is one more table row; recipes keep the same logical path.

## Fallback rules

- Fallback to files happens only when the vault says "not found", i.e. the
  reference isn't in the table.
- For a mapped reference, any other vault failure (locked, corrupt, auth,
  `foostore` missing) fails the plan record. It never serves a stale file
  copy.
- The vault is only touched when a task resolves a secret; `-list` and
  secret-free tasks never run foostore.
- `OptionalSecret` skips a host only when its own file is absent below an
  existing `gonf/secrets`. For goprecords that means: no token, no uploader
  declared, existing `/etc/goprecords-upload.token` left alone (the "keep"
  policy, see `goprecords.Client` in `gonf/goprecords/client.go`, shared by
  `frontends_goprecords` and `freebsd_goprecords_upload`).

## Controller prerequisites (mapped secrets)

- `foostore` with `foostore read` on `PATH`.
- `~/.config/foostore.json` with `kdbx_pass_file` set explicitly (machine
  reads never use `~/.master.pass`) and `kdbx_path` if the store isn't at
  `~/Documents/Keepass/master.kdbx`.
- The pass file: regular file owned by the gonf user, mode 0600 or 0400.

Check one entry without printing it (0 means readable):

```sh
foostore read --backend keepass --exact --raw --non-interactive --field Password -- Infra/garage-rpc >/dev/null; echo $?
```

Details: gonf's
[`docs/design/secrets.md`](https://github.com/snonux/gonf/blob/main/docs/design/secrets.md)
("Operator prerequisites", "secret.NewFallback").

## Working directory

Run gonf through `./gonf.sh` (any cwd; it changes into `gonf/`) or
`cd gonf && go run ./cmd/gonf ...`. Never `go run ./gonf/cmd/gonf` from the
repo root: the secrets root resolves relative to the process cwd, and the
record fails with `secrets directory "secrets" not found in the working
directory`.

## Bootstrap a checkout

From the repo root; copies files without printing them. The rsync exclusion
skips `frontends/secrets/.git`, which is legacy metadata.

```sh
umask 077
install -d -m 700 gonf/secrets/frontends gonf/secrets/garage
rsync -a --exclude='.git/' frontends/secrets/ gonf/secrets/frontends/
install -m 600 f3s/garage/secrets/rpc_secret gonf/secrets/garage/rpc_secret
find gonf/secrets -type d -exec chmod 700 {} +
find gonf/secrets -type f ! -name .gitignore ! -name README.md -exec chmod 600 {} +
```

`gonf/secrets/.gitignore` ignores every payload. Keep the Rex-era roots
(`frontends/secrets`, `f3s/garage/secrets`) and the legacy copies of migrated
secrets: deleting them needs separate, explicit approval.

Never put secret values in task names, inventory values, plan fixtures, logs
or commits.
