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
