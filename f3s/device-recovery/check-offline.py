#!/usr/bin/env python3
"""Verify the discovery allowlist and secret filtering without contacting devices."""

import importlib.util
import json
from pathlib import Path


spec = importlib.util.spec_from_file_location("shelly_probe", Path(__file__).with_name("shelly-probe.py"))
probe = importlib.util.module_from_spec(spec)
spec.loader.exec_module(probe)
secret = "OFFLINE_SECRET_SENTINEL"
payload = {"sta": {"ip": "192.0.2.28", "pass": secret, "ssid": secret}, "unknown": secret}
assert probe.sanitize("Wifi.GetConfig", payload) == {"sta": {
    "enable": None, "ipv4mode": None, "ip": "192.0.2.28",
    "netmask": None, "gw": None, "nameserver": None,
}}
for method in probe.METHODS:
    output = probe.sanitize(method, {**payload, "message": secret, "password": secret})
    assert secret not in json.dumps(output), method
    assert probe.sanitize(method, {"code": 500, "message": secret}) == {"error": "rpc_error"}
assert not any("Set" in method or "Start" in method or "Reboot" in method for method in probe.METHODS)
for method, fields in probe.FIELDS.items():
    for malformed in [{"password": secret}, [secret], secret + "\nINJECTED"]:
        assert secret not in json.dumps(probe.sanitize(method, dict.fromkeys(fields, malformed)))
for key in probe.NETWORK_FIELDS:
    for malformed in [{"password": secret}, [secret], secret, "192.168.1.999", "::1"]:
        assert probe.sanitize("Wifi.GetConfig", {"sta": {key: malformed}})["sta"][key] is None
assert probe.scalar("output", 1) is None
assert probe.scalar("uptime", True) is None
assert probe.scalar("uptime", -1) is None
assert probe.scalar("initial_state", "off; password=" + secret) is None
assert probe.scalar("ipv4mode", "static") == "static"
assert probe.scalar("gw", "192.0.2.1") == "192.0.2.1"
try:
    probe.sanitize("Switch.Set", {})
except ValueError:
    pass
else:
    raise AssertionError("mutation method accepted")
print("offline discovery allowlist and redaction checks passed")
