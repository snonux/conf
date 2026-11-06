#!/usr/bin/env python3
"""Check static WG field associations without ever using real keys."""
import hashlib
from pathlib import Path
import subprocess

AWK = Path(__file__).with_name("wg-static.awk")


def digest(text):
    result = subprocess.run(["awk", "-f", str(AWK)], input=text, text=True, capture_output=True)
    if result.returncode:
        return None
    return hashlib.sha256("\n".join(sorted(result.stdout.splitlines())).encode()).hexdigest()


base = "private public 56709 off\na psk-a 198.51.100.1:56709 192.0.2.1/32,2001:db8::1/128 100 1 2 25\nb psk-b 198.51.100.2:56709 192.0.2.2/32 200 3 4 off\n"
expected = digest(base)
assert expected is not None
reordered = "private public 56709 off\nb psk-b 198.51.100.2:56709 192.0.2.2/32 999 888 777 off\na psk-a 198.51.100.1:56709 2001:db8::1/128,192.0.2.1/32 777 123 456 25\n"
assert digest(reordered) == expected
for old, new in (("private", "different-private"), ("public", "different-public"),
                 ("56709 off", "56710 off"), ("56709 off", "56709 1"),
                 ("192.0.2.1/32", "192.0.2.9/32"), ("198.51.100.1", "198.51.100.9"),
                 (" 2 25", " 2 0")):
    assert digest(base.replace(old, new, 1)) != expected
swapped = base.replace("psk-a", "temporary").replace("psk-b", "psk-a").replace("temporary", "psk-b")
assert digest(swapped) != expected
assert digest("\n".join(base.splitlines()[:-1]) + "\n") != expected
assert digest(base + base.splitlines()[1] + "\n") is None
assert digest("") is None
assert digest("private public 56709\n") is None
assert digest(base.replace("100 1 2 25", "100 1 25")) is None
print("PASS: peer/IP order and volatile counters normalize; identities, PSK bindings, endpoints, prefixes, ports and keepalives remain strict")
