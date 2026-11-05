#!/usr/bin/env python3
"""Read-only, allowlisted Shelly recovery discovery; never prints raw responses.

Usage: python3 shelly-probe.py 192.168.1.28 192.168.1.29
Digest password: first line of ~/.shelly_plug. No mutation RPC is implemented.
"""

import argparse
import ipaddress
import json
from pathlib import Path
import re
import urllib.error
import urllib.request


FIELDS = {
    "Shelly.GetDeviceInfo": ("id", "model", "gen", "ver", "auth_en"),
    "Switch.GetStatus?id=0": ("output",),
    "Switch.GetConfig?id=0": ("initial_state",),
    "Matter.GetConfig": ("enable",),
    "Matter.GetStatus": ("num_fabrics", "commissionable"),
    "Sys.GetStatus": ("unixtime", "uptime", "restart_required"),
}
METHODS = (*FIELDS, "Script.List", "Schedule.List", "Wifi.GetConfig")
NETWORK_FIELDS = ("enable", "ipv4mode", "ip", "netmask", "gw", "nameserver")
BOOLEAN_FIELDS = {"auth_en", "output", "enable", "commissionable", "restart_required"}
INTEGER_FIELDS = {"gen", "num_fabrics", "unixtime", "uptime"}
ENUM_FIELDS = {
    "initial_state": {"off", "on", "restore", "match_input"},
    "ipv4mode": {"dhcp", "static"},
}
TEXT_FIELDS = {
    "id": r"[a-z0-9-]{1,80}",
    "model": r"[A-Z0-9-]{1,40}",
    "ver": r"[0-9][a-zA-Z0-9.+-]{0,63}",
}


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def sanitize(method, data):
    """Extract only evidence fields; discard credentials and arbitrary errors."""
    if method not in METHODS:
        raise ValueError("RPC method is not allowlisted")
    if not isinstance(data, dict):
        return {"error": "unexpected_response"}
    if "code" in data:
        return {"error": "rpc_error"}
    if method in FIELDS:
        return {key: scalar(key, data.get(key)) for key in FIELDS[method]}
    if method == "Script.List":
        scripts = data.get("scripts")
        return {"count": len(scripts)} if isinstance(scripts, list) else {"error": "unexpected_response"}
    if method == "Schedule.List":
        jobs = data.get("jobs")
        return {"count": len(jobs)} if isinstance(jobs, list) else {"error": "unexpected_response"}
    if method == "Wifi.GetConfig":
        return {
            station: {key: scalar(key, data[station].get(key)) for key in NETWORK_FIELDS}
            for station in ("sta", "sta1")
            if isinstance(data.get(station), dict)
        }
    raise ValueError("RPC method is not allowlisted")


def scalar(key, value):
    """Reject nested or malformed field values rather than echoing them."""
    if value is None:
        return None
    if key in BOOLEAN_FIELDS:
        return value if type(value) is bool else None
    if key in INTEGER_FIELDS:
        return value if type(value) is int and 0 <= value <= 2**53 - 1 else None
    if key in ENUM_FIELDS:
        return value if type(value) is str and value in ENUM_FIELDS[key] else None
    if key in TEXT_FIELDS:
        return value if type(value) is str and re.fullmatch(TEXT_FIELDS[key], value) else None
    if key in {"ip", "netmask", "gw", "nameserver"} and type(value) is str:
        try:
            return str(ipaddress.IPv4Address(value))
        except ipaddress.AddressValueError:
            return None
    return None


def probe(address, password):
    base = "http://" + str(ipaddress.IPv4Address(address))
    manager = urllib.request.HTTPPasswordMgrWithDefaultRealm()
    manager.add_password(None, base, "admin", password)
    opener = urllib.request.build_opener(
        urllib.request.ProxyHandler({}), NoRedirect(),
        urllib.request.HTTPDigestAuthHandler(manager),
    )
    result = {"address": address}
    for method in METHODS:
        try:
            with opener.open(base + "/rpc/" + method, timeout=6) as response:
                data = json.load(response)
            result[method] = sanitize(method, data)
        except urllib.error.HTTPError as exc:
            result[method] = {"http_status": exc.code}
        except (OSError, ValueError):
            result[method] = {"error": "read_failed"}
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("addresses", nargs="+", type=ipaddress.IPv4Address)
    args = parser.parse_args()
    password = (Path.home() / ".shelly_plug").read_text().splitlines()[0]
    for address in args.addresses:
        print(json.dumps(probe(str(address), password), sort_keys=True))


if __name__ == "__main__":
    main()
