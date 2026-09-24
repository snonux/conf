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
- `paths.FrontendSecret("etc/goprecords/<host>.token")` reads the vault
  entry `Infra/goprecords-token-<host>` (Password field) for blowfish and
  fishfinger; its legacy copy is
  `gonf/secrets/frontends/etc/goprecords/<host>.token`. A frontend added
  later has no entry and reads this directory.

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

`vault` is a `secret/foostore` provider whose `foostore.Items` table maps
four references to their KeePass entries: the NSD TSIG key, the Garage RPC
secret and the blowfish and fishfinger goprecords upload tokens (see the list
above). foostore unlocks the store with its own configured `kdbx_pass_file`;
no passphrase passes through gonf. The vault is read only when a task
resolves a secret, so `-list` and secret-free tasks never run foostore.

Controller prerequisites for a task that resolves a mapped secret (gonf's
`docs/secrets.md`, "Operator prerequisites"):

- `foostore` with the machine read contract (`foostore read`) on `PATH`;
- `~/.config/foostore.json` for the user running gonf, setting
  `kdbx_pass_file` explicitly (foostore never uses its `~/.master.pass`
  default for machine reads) and `kdbx_path` if the store is not at
  foostore's default `~/Documents/Keepass/master.kdbx`;
- the pass file: a regular file owned by that user, mode `0600` (or `0400`).

Without them a mapped reference fails the plan record as unavailable (it
never falls back to this directory). Check one entry without printing it:
`foostore read --backend keepass --exact --raw --non-interactive --field
Password -- Infra/garage-rpc >/dev/null; echo $?` (0 means readable).

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

The per-host goprecords tokens are mapped too (conf commit 59393f6): they
were imported from the Rex-era copies, which match the tokens already on the
hosts, so `frontends_goprecords` now manages `/etc/goprecords-upload.token`
and the uploader. `OptionalSecret` still skips a host that has neither a
vault entry nor a token file here.

Task ze2 verified the cutover (all 61 recorded plans byte-identical to the
file-only baseline, a fallback drill, a locked-vault drill and a rotation
drill; see that task's annotations); mapping the goprecords tokens left
every other task's plan byte-identical. The legacy copies of the migrated
secrets are kept in this directory and in the Rex-era roots: deleting them is
a further, separately authorized step.

The Goprecords per-host token additionally has an explicit "keep" policy for
what happens to an already-deployed `/etc/goprecords-upload.token` once its
controller secret is genuinely not found (as opposed to the store being
unavailable, which still fails loudly): see the doc comment on
`Maintenance.Goprecords` in `frontends/maintenance.go` for the full
keep/disable/remove comparison and why "keep" — no automatic deletion of
legacy copies — is the one in effect today.
