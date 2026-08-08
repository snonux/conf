# f3sctl agent on the f-hosts

[`f3sctl`](https://github.com/snonux/f3sctl) runs on `pi0`/`pi1` and reaches
`f0`–`f3` over SSH to stop bhyve guests, power the host off, and export the
`zusb` backup pool. This directory sets up the restricted account it logs in
as.

It replaces the way `wol-f3s` did the same job: as `paul`, a full-shell
account, piping a shell heredoc into `doas /bin/sh -s`.

## What the account can do

Nothing but run `/usr/local/bin/f3sctl agent`, which accepts a **single bare
word** from a fixed allowlist and passes nothing to a shell:

| Verb | Needs root |
|---|---|
| `probe` | yes (`vm list`) |
| `zusb-status` | no |
| `zusb-unload` | yes |
| `poweroff` | yes |

Four things enforce that, and they are deliberately not the same mechanism:

1. **A dedicated unprivileged `f3sctl` user** with home `/var/db/f3sctl` and
   **no `~/.ssh`** — its `authorized_keys` lives in the root-owned
   `/etc/ssh/authorized_keys.d/f3sctl`, so the account cannot re-authorise
   itself even if something running as it were compromised.
2. **`from="192.168.1.125,192.168.1.126,192.168.2.203,192.168.2.204"`** on the
   key, pinning it to pi0/pi1 on both their LAN and WireGuard addresses. The
   key is useless from anywhere else, including earth.
3. **`ForceCommand` in a `Match User f3sctl` block in `sshd_config`.** This is
   the authoritative one: a `command=` key option is only as good as the key
   file, whereas this is root-owned daemon config and overrides whatever the
   key says.
4. **`doas` rules keyed to exact argv**, so the escalation for the three
   privileged verbs is as narrow as the SSH allowlist in front of it.
   `agent-root` additionally refuses to run unless `euid` is 0, so a mistake in
   `doas.conf` fails loudly instead of half-working.

A shell is still the account's login shell (`/bin/sh`) — **that is required**,
because sshd execs the `ForceCommand` *through* it. `nologin` would break the
agent rather than harden it.

## Install

Install the package (from the f3s repo) and run the setup script:

```sh
doas pkg install -y f3sctl
doas sh setup-agent-freebsd.sh 'ssh-ed25519 AAAA... f3sctl@pi'
```

The script is idempotent, validates `sshd -t` before reloading, and can be
re-run to rotate the key.

**Keep a second root session open on the host while running it.** It reloads
sshd; if that ever goes wrong on a machine whose whole purpose is being
reachable remotely, the way back in is a console.

## Verify

From `pi0`, as the user the CGI runs as:

```sh
doas -u _httpd ssh -i /var/db/f3sctl/id_ed25519 -o IdentitiesOnly=yes \
    f3sctl@192.168.1.130 zusb-status      # -> "absent" or "loaded"
```

These must all be refused:

```sh
... f3sctl@192.168.1.130 'echo pwned'              # -> expected exactly one verb
... f3sctl@192.168.1.130 'zusb-status; echo pwned' # -> expected exactly one verb
... f3sctl@192.168.1.130                           # -> no verb requested (no shell)
ssh -N -L 9999:127.0.0.1:22 f3sctl@192.168.1.130   # -> no forwarding
```

And from **earth**, the same key must be rejected outright by the `from=` pin.

## Related

- The OpenBSD gateways get an equivalent account (mute marker only, no `doas`):
  `../../../frontends/scripts/setup-f3sctl-agent-openbsd.sh`
- The pool this protects on shutdown: [`../zusb/README.md`](../zusb/README.md)
- The plug the power sequence switches: [`../shelly-fans/README.md`](../shelly-fans/README.md)
