# Gonf controller secrets

`api.MustSecret` and `api.OptionalSecret` intentionally read only below this
directory, through gonf's default file secret provider (`secret.FileProvider`,
see gonf's `docs/secrets.md`); this repository configures no other provider.
They never read the legacy Rex secret roots directly. This keeps a
gonf plan's controller inputs explicit and makes the logical paths used by
recipes stable:

- `paths.FrontendSecret("var/nsd/etc/nsd_key.txt")` reads
  `gonf/secrets/frontends/var/nsd/etc/nsd_key.txt`.
- `paths.GarageSecret("rpc_secret")` reads
`gonf/secrets/garage/rpc_secret`.

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

Keep the existing Rex roots while their tasks remain unported; do not remove
or move them as part of this bootstrap. `gonf/secrets/.gitignore` ignores all
secret payloads, and no command above emits a secret value. Do not add secrets
to task names, inventory values, plan fixtures, or commits.

## Typed provider references and cutover policy (task 262)

Every logical reference this repository resolves — `FrontendSecret`,
`GarageSecret`, and the per-host `etc/goprecords/<name>.token` references
`Maintenance.Goprecords` builds directly — still reads exactly this
directory, through gonf's default `secret.FileProvider`, exactly as before
this task. No `api.SetSecretProvider` call exists anywhere in this
repository yet, and this task does not add one: switching the active
provider means real store credentials and a real reachable vault, which is a
live-vault change needing its own, separately authorized rotation/recovery
exercise (see `docs/consumer-dsl-simplification-plan.md`, P9), not something
to bundle into a mechanism task.

What this task adds is the mechanism a later, authorized cutover uses:
gonf-core's `secret.NewFallback(primary, secondary)` (see gonf's
`docs/secrets.md`, "secret.NewFallback"). It resolves through `primary` and
falls back to `secondary` only when `primary` reports the reference not
found — the answer a reference not yet listed in `primary`'s own lookup
table gives — so a future cutover looks like:

```go
items, err := foostore.Items(map[secret.Ref]foostore.Item{
    // add one logical reference at a time as each secret is migrated,
    // e.g. "garage/rpc_secret": foostore.Field("<real vault entry>", "<real field>"),
})
provider, err := foostore.New(foostore.Config{Lookup: items})
api.SetSecretProvider(secret.NewSnapshot(secret.NewFallback(provider, secret.FileProvider{})))
```

with an empty (or partial) `items` table every reference not yet listed keeps
reading this directory unchanged — byte for byte, since `secret.FileProvider`
never moves and every recipe keeps passing the exact same logical path
strings documented above. Crucially, `NewFallback` never falls back to this
directory when the new store answers with anything other than "not found":
a locked, unauthenticated, corrupt or otherwise broken store fails the whole
plan record loudly instead of silently serving this directory's possibly
stale copy of an already-migrated secret (see gonf's `docs/secrets.md` for
why that distinction, and not merely "did the read succeed", is the one that
matters). Wiring the actual `foostore.Items` table with real vault entries,
calling `api.SetSecretProvider` from `cmd/gonf/main.go`, and rotating any
secret once real access is authorized are explicitly out of this task's
scope; see the follow-up task filed for that work.

The Goprecords per-host token additionally has an explicit "keep" policy for
what happens to an already-deployed `/etc/goprecords-upload.token` once its
controller secret is genuinely not found (as opposed to the store being
unavailable, which still fails loudly): see the doc comment on
`Maintenance.Goprecords` in `frontends/maintenance.go` for the full
keep/disable/remove comparison and why "keep" — no automatic deletion of
legacy copies — is the one in effect today.
