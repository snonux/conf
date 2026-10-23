"""Exercise the actual scanner against private temporary Git repositories."""
import base64
import os
from pathlib import Path
import secrets
import shutil
import string
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCANNER = ROOT / "tools/check-secrets.py"


class ScannerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="conf-secrets-test-")
        self.addCleanup(self.tmp.cleanup)
        self.directory = Path(self.tmp.name)
        self.repo = self.directory / "repo"
        self.repo.mkdir()
        self.env = {**os.environ, "GIT_CONFIG_GLOBAL": os.devnull,
                    "GIT_CONFIG_NOSYSTEM": "1"}
        self.git("init", "-q")
        (self.repo / "README").write_text("clean fixture\n")
        self.git("add", "README")
        self.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
                 "-c", "core.hooksPath=/dev/null", "commit", "-qm", "fixture")
        self.values = []

    def git(self, *args):
        return subprocess.run(["git", *args], cwd=self.repo, env=self.env,
                              check=True, capture_output=True, text=True)

    def stage(self, filename, data):
        path = self.repo / filename
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(data)
        self.git("add", "--", filename)
        return path

    def token(self):
        value = "ghp_" + "".join(secrets.choice(string.ascii_letters + string.digits) for _ in range(36))
        self.values.append(value)
        return value

    def scan(self, env=None):
        result = subprocess.run(["python3", str(SCANNER), "--staged"],
                                cwd=self.repo, env={**self.env, **(env or {})},
                                capture_output=True, text=True, timeout=20)
        for value in self.values:
            self.assertFalse(value in result.stdout + result.stderr,
                             "scanner disclosed fixture credential contents")
        return result

    def test_clean_staged_content(self):
        self.stage("notes.txt", "ordinary configuration\n")
        self.assertEqual(self.scan().returncode, 0)

    def test_real_private_key_is_blocked(self):
        key = self.directory / "fixture-key"
        subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(key)],
                       check=True, capture_output=True)
        data = key.read_text()
        self.values.extend(data.splitlines()[1:-1])
        self.stage("private.key", data)
        result = self.scan()
        self.assertEqual(result.returncode, 1)
        self.assertIn('"file": "private.key"', result.stderr)

    def test_staged_secret_is_detected_after_worktree_is_cleaned(self):
        path = self.stage("token.env", 'token="' + self.token() + '"\n')
        path.write_text('token="${GITHUB_TOKEN}"\n')
        self.assertEqual(self.scan().returncode, 1)

    def test_unstaged_secret_is_not_mistaken_for_a_staged_change(self):
        path = self.stage("token.env", 'token="${GITHUB_TOKEN}"\n')
        path.write_text('token="' + self.token() + '"\n')
        self.assertEqual(self.scan().returncode, 0)

    def test_documented_placeholders_are_allowed(self):
        self.stage("docs/examples.txt", "aws_access_key_id=AKIAIOSFODNN7EXAMPLE\n"
                   "aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\n"
                   'token="ghp_your_token_here"\nPrivateKey = ${WIREGUARD_PRIVATE_KEY}\n')
        self.assertEqual(self.scan().returncode, 0)

    def test_real_key_beside_documentation_placeholder_is_not_exempt(self):
        self.stage("docs/examples.txt", "aws_access_key_id=AKIAIOSFODNN7EXAMPLE\n"
                   'token="' + self.token() + '"\n')
        self.assertEqual(self.scan().returncode, 1)

    def test_wireguard_key_is_blocked(self):
        value = base64.b64encode(secrets.token_bytes(32)).decode()
        self.values.append(value)
        self.stage("wg0.conf", "[Interface]\nPrivateKey = " + value + "\n")
        result = self.scan()
        self.assertEqual(result.returncode, 1)
        self.assertIn("wireguard-private-key", result.stderr)

    def test_inline_allow_comment_does_not_hide_a_new_secret(self):
        self.stage("token.env", 'token="' + self.token() + '" # gitleaks:allow\n')
        self.assertEqual(self.scan().returncode, 1)

    def test_repository_ignore_file_does_not_bypass_gate(self):
        self.stage("token.env", 'token="' + self.token() + '"\n')
        self.stage(".gitleaksignore", "token.env:github-pat:1\n")
        result = self.scan()
        self.assertEqual(result.returncode, 2)
        self.assertIn(".gitleaksignore is unsupported", result.stderr)

    def test_missing_or_wrong_scanner_fails_closed(self):
        self.assertEqual(self.scan({"GITLEAKS_BIN": str(self.directory / "missing")}).returncode, 2)
        fake = self.directory / "fake-scanner"
        fake.write_text('#!/bin/sh\nprintf "0.0.0\\n"\n')
        fake.chmod(0o755)
        self.assertEqual(self.scan({"GITLEAKS_BIN": str(fake)}).returncode, 2)

    def test_scanner_error_output_never_reaches_the_terminal(self):
        value = self.token()
        fake = self.directory / "fake-scanner"
        fake.write_text('#!/bin/sh\nif [ "$1" = version ]; then echo 8.30.1; exit 0; fi\n'
                        'printf "%s\\n" "$CONF_TEST_SECRET"\n'
                        'printf "%s\\n" "$CONF_TEST_SECRET" >&2\nexit 2\n')
        fake.chmod(0o755)
        self.assertEqual(self.scan({"GITLEAKS_BIN": str(fake), "CONF_TEST_SECRET": value}).returncode, 2)

    def test_actual_pre_commit_hook_blocks_then_allows_a_clean_commit(self):
        (self.repo / "tools").mkdir()
        shutil.copy2(SCANNER, self.repo / "tools/check-secrets.py")
        for name in (".gitleaks.toml", ".gitleaks-version"):
            shutil.copy2(ROOT / name, self.repo / name)
        self.git("add", "tools/check-secrets.py", ".gitleaks.toml", ".gitleaks-version")
        self.stage("token.env", 'token="' + self.token() + '"\n')
        before = self.git("rev-parse", "HEAD").stdout
        command = ["git", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
                   "-c", "core.hooksPath=" + str(ROOT / ".githooks"), "commit", "-qm", "hook fixture"]
        result = subprocess.run(command, cwd=self.repo, env=self.env,
                                capture_output=True, text=True, timeout=20)
        self.assertFalse(any(v in result.stdout + result.stderr for v in self.values),
                         "hook disclosed credential contents")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.git("rev-parse", "HEAD").stdout, before)
        self.stage("token.env", 'token="${GITHUB_TOKEN}"\n')
        result = subprocess.run(command, cwd=self.repo, env=self.env,
                                capture_output=True, text=True, timeout=20)
        self.assertEqual(result.returncode, 0)
        self.assertNotEqual(self.git("rev-parse", "HEAD").stdout, before)


if __name__ == "__main__":
    unittest.main()
