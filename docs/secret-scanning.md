# Leaked-key audit and staged secret scanning (kn3)

Preparation dated 2026-10-22, with host coverage refreshed on 2026-10-23 and
prepared for review on an isolated branch based on approved commit `86e26cb`.
Independent review, hook activation/publication and the remaining key-inventory
checks remain pending.
No keys, SSH policy, production configuration or Git history were changed.

## Fingerprint coverage

The previously derived public fingerprint is
`SHA256:O6Y3HIEDggtnaEcAJ3bxejyfAl6EM2mS/ZdPyaecbDY`. Its private key was
committed at `eff2c9c` and removed at `5f06401`; no disk copy of that
historical private key was made for this check. Public history remains intact.

Fresh checks found no matching fingerprint in:

| Source | Verified coverage |
| --- | --- |
| Host key files, 22 October | f0, f1, f2, r0, r1, r2, pi0, pi1, pi2, blowfish, fishfinger |
| Host key files, 23 October | f3 and rocky: seven nonempty file visits, no parse errors or target matches; standard key paths, no global external key command |
| GitHub | 3 user keys; deploy-key lists of all 99 owned repositories, containing 1 deploy key |
| Codeberg | 2 keys from the published `snonux.keys` endpoint |
| Forgejo | Read-only DB query: 3 public keys, 0 deploy-key rows |
| ArgoCD | Active `cicd/git-server-repo-creds` repository key parsed and different |

Host inspection covered common key-file roots and account `.ssh` directories
without following unrelated filesystem mounts. All inspected hosts reported
standard AuthorizedKeysFile paths and no global AuthorizedKeysCommand; find
completed and key parsing reported no errors. The 22 October inspection made
55 file visits, including 16 empty files; overlapping home/root visits were
not deduplicated.
This records inspected-file coverage, not an unrestricted proof about every
possible authentication mechanism or external machine.

Only counts, booleans and fingerprints were handled in audit output. The
ArgoCD key was fingerprinted through an anonymous mode-0600 RAM file and
closed without a disk copy. An initial stdin fingerprint attempt failed and
was treated as unverified until the RAM-file attempt succeeded. Codeberg's
API required authentication, so its public `.keys` response provided user-key
coverage; private Codeberg repository deploy-key lists were not audited.

The owner separately authorized ordinary rack startup on 23 October. The same
read-only probe then completed on f3 and rocky: account key roots were enumerated,
find returned zero, and their one and six visited key files respectively
contained no target fingerprint. The earlier 55 visits describe the 22 October
checks; these seven visits are additional.

Rocky's initial probe timed out after 40 seconds. A fresh verified connection
with client connection sharing disabled completed both `true` and the complete
key audit. This changed only the diagnostic command's SSH options, not SSH
configuration or authentication policy. These dated checks cover 13 of the 14
known hosts. Sanitized evidence remains outside Git in the original preparation's
`/home/paul/.local/state/conf-worktrees/kn3-secret-scanning-20261022/evidence/`
directory: `authorized-key-audit-20261023.json`, `rocky-ssh-access-20261023.json`
and `rocky-authorized-key-audit-20261023.json`. The initial timeout is retained
alongside the completed result; this review branch does not relocate the evidence.

**Still unverified:** pi3's earlier SSH probe did not complete and only that
owned client process was terminated after inspection. A fresh 23 October
probe with connection sharing disabled also exceeded its 40-second bound;
`pi3-authorized-key-audit-20261023.json` in that same evidence directory records
that failure without claiming key coverage. Private Codeberg deploy-key
inventory also remains open.
No power operation, gateway access or SSH-auth change was performed by these
key checks. These gaps prevent claiming that the leaked key is unusable everywhere.

## Scanner preparation

`.gitleaks.toml` extends the standard Gitleaks rules and adds a WireGuard
PrivateKey assignment rule. The only added global value exceptions are the
two exact published AWS example credentials. Tests also verify normal
`${VAR}` and `ghp_your_token_here` placeholders, and prove a real credential
beside an allowed example is still blocked. No entire test/doc directory,
rule or historical commit is exempted by this preparation.

`tools/check-secrets.py --staged` scans the Git index using Gitleaks
**8.30.1**, pinned in `.gitleaks-version`. It checks staged changes even when
the working file was subsequently cleaned. Unstaged changes are excluded.
Missing tools, wrong versions, scanner failures/timeouts or inconsistent
reports block the commit instead of silently passing.

The wrapper suppresses scanner console output and uses a custom report
template containing only rule, file, line, commit and finding fingerprint.
Secret values, matching lines, commit messages and authors are excluded even
from its temporary report. JSON quoting protects location output from
embedded control characters. Temporary reports are removed after each run.

Inline `gitleaks:allow` comments are disabled. The wrapper rejects a repository
`.gitleaksignore` file: this Gitleaks version loads that file automatically,
even when another ignore path is supplied. An initially failing native test
exposed that behavior; the explicit refusal closes that bypass. Baselines
and ambient config overrides do not suppress this gate.

The wrapper reads `.gitleaks.toml` and `.gitleaks-version` beside its own
script in the working tree. These files and the hook remain locally editable;
this is a local commit guard, and does not enforce an immutable policy or
prevent intentional hook bypass. Review policy changes with the scanner code.

The native scanner archive used for verification was downloaded from the
[official 8.30.1 release](https://github.com/gitleaks/gitleaks/releases/tag/v8.30.1)
and matched its published Linux x64 SHA-256:
`551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb`.
Install a trusted platform-appropriate binary on PATH; the hook does not
download or install software automatically. `GITLEAKS_BIN` can point to an
explicit trusted binary. See the
[upstream usage/configuration reference](https://github.com/gitleaks/gitleaks/blob/v8.30.1/README.md).

## Verification and history findings

Twelve native tests pass against temporary Git repositories, including real
generated ed25519 private keys, generated GitHub/WireGuard credentials,
staged-versus-working-tree cases, narrow placeholders, ignore-file/inline
bypass refusal, missing/wrong tools and scanner error-output redaction. An
actual `git commit` with the prepared hook is rejected while a secret is
staged, leaves HEAD unchanged, then succeeds once the staged file is clean.
Fixture keys and repositories are removed; no production credential is used.

```sh
GITLEAKS_BIN=/path/to/gitleaks python3 -m unittest discover \
  -s tools/tests -p test_check_secrets.py
python3 tools/check-secrets.py --staged
```

The original 22 October metadata-only scan of history reachable from source
commit `0f8661f` reported **13 potential findings**: 8 generic API-key
detections, 2 curl user-auth detections, 2 curl header-auth detections and the
known historical private key. This is not a clean history result. The prior audit records
rotation of ArgoCD admin material and replacement of the old repository key;
the key-inventory gaps above remain open. The seven non-Argo detections were
triaged in **fo3**; its bounded local report completed independent review on
23 October at `de9edcc:docs/historical-secret-triage.md`. Follow-up **go3**
completed review of the historical LiteLLM token's dated retirement/reuse
checks at `c5e4664:docs/vllm-token-verification.md`. Both report objects resolve
in this repository but are outside this branch's ancestry and owned file set;
inspect them locally with `git show <commit>:<path>`. Neither investigation
establishes universal revocation or a clean history. No finding was
blanket-ignored, printed as a value, or used to justify a history rewrite.

`--history` is an explicit audit mode and scans all ancestry reachable from
HEAD. It intentionally returns nonzero for those findings. The staged gate
does not suppress new secrets merely because a historical scan has findings.
Default upstream rule coverage and exclusions still apply; a passing scan
cannot prove that every possible secret format is absent.

## Activation and completion still pending

Read-only inspection on 23 October found no Gitleaks executable on the active
PATH, no configured `core.hooksPath` and no `.git/hooks/pre-commit` in the
canonical `/home/paul/git/conf` checkout. `~/.local/bin` is on that PATH and
has no existing `gitleaks` entry. No tool installation or hook activation has
been performed.

After fresh review, first make the verified pinned tool persistently available.
Recheck these facts, verify the archive checksum above and compare the extracted
binary with the verified archive member. Create a new versioned executable at
`~/.local/lib/gitleaks/8.30.1/gitleaks` and a new `~/.local/bin/gitleaks` link,
refusing to overwrite an existing file or link. Verify version and executable
identity through PATH before activating the hook. Do not depend on the scanner
under the temporary preparation's state/worktree directory or change global
Git configuration.

Then place the reviewed seven paths in the canonical checkout and choose one
hook integration, preserving any hooks/configuration discovered on reinspection:

```sh
# Native Git hook; first inspect the existing hooksPath/hook setup.
git config --local core.hooksPath .githooks

# Or, if the pre-commit framework is already used:
pre-commit install
```

Both routes call the same staged-index gate. Configuring hooksPath affects
the repository's other worktrees too; it has not been enabled during this
preparation. Publishing these changes requires the task's explicit approval
and an isolated branch; evolving conf master contains unrelated unapproved
preparations and must not be pushed wholesale.

After activation, run the staged gate in the canonical checkout and repeat the
native fixture suite with the persistent tool. Its temporary-repository commit
test checks rejection/unchanged HEAD and clean-commit success without committing
a fixture into this repository.

Fresh independent review of this branch has not run. Finish Pi3 verification
when normal access is available and record the private Codeberg deploy-key
inventory limit. Completed bounded investigations fo3 and go3 are independent
of this staged-hook/key-revocation task. Any necessary revocation/rotation
or history rewrite needs its own explicit authorization. The user's SSH
policy and recovery-boot/power-cycle opt-outs remain in effect; the separately
authorized ordinary rack startup does not permit further power operations.
kn3 stays unfinished.
