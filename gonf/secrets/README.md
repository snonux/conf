# Gonf controller secrets

`api.MustSecret` and `api.OptionalSecret` resolve through the provider
`cmd/gonf/main.go` configures (task ze2): the foostore/KeePass vault for the
references it maps, and below this directory, through gonf's file secret
provider (`secret.FileProvider`, see gonf's `docs/secrets.md`), for every
other reference. They never read the legacy Rex-era secret roots directly.
This keeps a gonf plan's controller inputs explicit and makes the logical
paths used by recipes stable:

- `paths.FrontendSecret("var/nsd/etc/nsd_key.txt")` reads the vault entry
  `Infra/nsd-tsig-key` (Password field); its legacy copy is
  `gonf/secrets/frontends/var/nsd/etc/nsd_key.txt`.
- `paths.GarageSecret("rpc_secret")` reads the vault entry `Infra/garage-rpc`
  (Password field); its legacy copy is `gonf/secrets/garage/rpc_secret`.
- `etc/goprecords/<host>.token` is unmapped and reads this directory.

Run gonf through the repository's `gonf.sh` wrapper (from any working
directory: it changes into this checkout's `gonf` directory itself and passes
its arguments through verbatim), or run `cd gonf && go run ./cmd/gonf …`.
Do not run `go run ./gonf/cmd/gonf` from the repository root: `MustSecret`
deliberately resolves its `secrets` directory relative to the recipe process
working directory, and the canonical root is `gonf/secrets`. With a gonf release that
includes the typed provider contract, running from the wrong directory fails
recording with `secrets directory "secrets" not found in the working
directory` for both helpers; older releases treated that as every secret
missing, so `OptionalSecret` silently dropped the goprecords tokens.
`OptionalSecret` skips a host only when its own token file is absent below an
existing `gonf/secrets`.

Bootstrap a checkout before running any task that needs a secret. Run these
commands from the repository root; they copy files without printing their
contents. The `rsync` exclusion is required because `frontends/secrets/.git`
is a legacy metadata directory, not secret input.

```sh
umask 077
install -d -m 700 gonf/secrets/frontends gonf/secrets/garage
rsync -a --exclude='.git/' frontends/secrets/ gonf/secrets/frontends/
install -m 600 f3s/garage/secrets/rpc_secret gonf/secrets/garage/rpc_secret
find gonf/secrets -type d -exec chmod 700 {} +
find gonf/secrets -type f ! -name .gitignore ! -name README.md -exec chmod 600 {} +
```

Rex itself is retired (conf task v42), but keep the legacy Rex-era secret
roots (`frontends/secrets`, `f3s/garage/secrets`): deleting or moving them
needs its own explicit authorization, never this bootstrap. `gonf/secrets/.gitignore` ignores all
secret payloads, and no command above emits a secret value. Do not add secrets
to task names, inventory values, plan fixtures, or commits.

## Typed provider references and cutover policy (tasks 262, ze2)

`cmd/gonf/main.go` sets the provider once, at the composition root:

```go
api.SetSecretProvider(secret.NewSnapshot(secret.NewFallback(vault, secret.FileProvider{})))
```

`vault` is a `secret/foostore` provider whose `foostore.Items` table maps the
NSD TSIG key and the Garage RPC secret to their KeePass entries (see the list
above). foostore unlocks the store with its own configured `kdbx_pass_file`;
no passphrase passes through gonf. The vault is read only when a task
resolves a secret, so `-list` and secret-free tasks never run foostore.

`secret.NewFallback` (gonf's `docs/secrets.md`, "secret.NewFallback")
resolves through the vault and falls back to this directory only when the
vault reports the reference not found — the answer for a reference the table
does not list. Migrating one more secret is one more table row; every recipe
keeps passing the exact same logical path strings. Crucially, `NewFallback`
never falls back to this directory when the vault answers with anything other
than "not found": a locked, unauthenticated, corrupt or otherwise unavailable
vault (or a missing `foostore` binary) fails the whole plan record loudly for
a mapped reference instead of silently serving this directory's possibly
stale copy of an already-migrated secret.

The per-host goprecords tokens stay unmapped: they are currently unset, and
`OptionalSecret` skips a host whose token file is absent here. Map them once
real tokens exist in the vault.

Task ze2 verified the cutover (all 61 recorded plans byte-identical to the
file-only baseline, a fallback drill, a locked-vault drill and a rotation
drill; see that task's annotations). The legacy copies of the migrated
secrets are kept in this directory and in the Rex-era roots: deleting them is
a further, separately authorized step.

The Goprecords per-host token additionally has an explicit "keep" policy for
what happens to an already-deployed `/etc/goprecords-upload.token` once its
controller secret is genuinely not found (as opposed to the store being
unavailable, which still fails loudly): see the doc comment on
`Maintenance.Goprecords` in `frontends/maintenance.go` for the full
keep/disable/remove comparison and why "keep" — no automatic deletion of
legacy copies — is the one in effect today.
