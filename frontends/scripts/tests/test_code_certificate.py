"""Exercise the staged-material validator with real, disposable PKI fixtures."""

import hashlib
import os
from pathlib import Path
import ssl
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "check-code-certificate.sh"


class CertificateTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.directory = tempfile.TemporaryDirectory()
        cls.root = Path(cls.directory.name)
        cls.extension = cls.root / "extensions.cnf"
        cls.extension.write_text(
            "subjectAltName=DNS:code.f3s.buetow.org\n"
            "extendedKeyUsage=serverAuth\n"
            "basicConstraints=critical,CA:FALSE\n")
        for name in ("ca", "other-ca"):
            cls.openssl("req", "-x509", "-newkey", "rsa:2048", "-nodes",
                        "-keyout", name + ".key", "-out", name + ".pem",
                        "-days", "365", "-subj", "/CN=" + name,
                        "-addext", "basicConstraints=critical,CA:TRUE")
        cls.openssl("req", "-new", "-newkey", "rsa:2048", "-nodes",
                    "-keyout", "leaf.key", "-out", "leaf.csr",
                    "-subj", "/CN=code.f3s.buetow.org")
        for name, ca, days, serial in (("good", "ca", "90", "1"),
                                       ("short", "ca", "1", "2"),
                                       ("untrusted", "other-ca", "90", "3")):
            cls.openssl("x509", "-req", "-in", "leaf.csr", "-CA", ca + ".pem",
                        "-CAkey", ca + ".key", "-set_serial", serial,
                        "-days", days, "-extfile", str(cls.extension),
                        "-out", name + ".pem")
        cls.openssl("pkey", "-in", "leaf.key", "-aes-256-cbc",
                    "-passout", "pass:fixture", "-out", "encrypted.key")
        (cls.root / "fullchain.pem").write_text(
            (cls.root / "good.pem").read_text() + (cls.root / "ca.pem").read_text())
        cls.openssl("req", "-new", "-newkey", "ec", "-nodes",
                    "-pkeyopt", "ec_paramgen_curve:prime256v1",
                    "-keyout", "ec.key", "-out", "ec.csr",
                    "-subj", "/CN=code.f3s.buetow.org")
        cls.openssl("x509", "-req", "-in", "ec.csr", "-CA", "ca.pem",
                    "-CAkey", "ca.key", "-set_serial", "4", "-days", "90",
                    "-extfile", str(cls.extension), "-out", "ec.pem")
        (cls.root / "malformed.pem").write_text("not a certificate\n")

    @classmethod
    def openssl(cls, *args):
        subprocess.run(["openssl", *args], cwd=cls.root, check=True,
                       capture_output=True, timeout=15)

    @classmethod
    def tearDownClass(cls):
        cls.directory.cleanup()

    def pin(self, name):
        der = ssl.PEM_cert_to_DER_cert((self.root / name).read_text())
        return hashlib.sha256(der).hexdigest()

    def validate(self, certificate="good.pem", key="leaf.key", pin=None):
        # All keys are disposable fixtures. Nothing is copied to a frontend.
        inputs = [p for p in self.root.iterdir() if p.is_file()]
        before = {p.name: hashlib.sha256(p.read_bytes()).digest() for p in inputs}
        result = subprocess.run(
            ["sh", str(SCRIPT), str(self.root / certificate), str(self.root / key),
             pin if pin is not None else self.pin(certificate),
             str(self.root / "ca.pem")],
            env={**os.environ, "TMPDIR": str(self.root)}, capture_output=True,
            text=True, timeout=10)
        after = {p.name: hashlib.sha256(p.read_bytes()).digest()
                 for p in self.root.iterdir() if p.is_file()}
        self.assertEqual(before, after, "validator modified an input")
        self.assertEqual(list(self.root.glob("code-cert-check.*")), [])
        for marker in ("PRIVATE KEY", "BEGIN CERTIFICATE"):
            self.assertNotIn(marker, result.stdout + result.stderr)
        return result

    def test_valid_pair_is_accepted_without_changing_inputs(self):
        result = self.validate()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("key match: OK", result.stdout)

    def test_different_private_key_is_rejected(self):
        result = self.validate(key="ca.key")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("do not match", result.stderr)

    def test_multi_pem_chain_is_accepted(self):
        result = self.validate(certificate="fullchain.pem", pin=self.pin("good.pem"))
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_ec_key_is_accepted(self):
        result = self.validate(certificate="ec.pem", key="ec.key")
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_short_lifetime_is_rejected(self):
        result = self.validate(certificate="short.pem")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("30 days", result.stderr)

    def test_untrusted_chain_is_rejected_even_with_matching_pin_and_key(self):
        result = self.validate(certificate="untrusted.pem")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("not trusted", result.stderr)

    def test_different_public_leaf_is_rejected(self):
        result = self.validate(pin=self.pin("short.pem"))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("pin mismatch", result.stderr)

    def test_invalid_pin_is_rejected(self):
        for value in ("", "0" * 63, "0" * 65, "A" * 64, "../leaf.key"):
            with self.subTest(pin=value):
                result = self.validate(pin=value)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("invalid public leaf pin", result.stderr)

    def test_encrypted_key_fails_without_prompting(self):
        result = self.validate(key="encrypted.key")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("cannot read candidate public key", result.stderr)

    def test_missing_and_malformed_inputs_are_rejected(self):
        for name in ("missing.pem", "malformed.pem"):
            with self.subTest(certificate=name):
                result = self.validate(certificate=name, pin=self.pin("good.pem"))
                self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__":
    unittest.main()
