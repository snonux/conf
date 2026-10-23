#!/usr/bin/env python3
"""Exercise real Gonf renderers and reject missing TLS/alias/verification policy.

Uses plan only: no deployment, CA request, production key access or daemon start.
Temporary plan/template files are retained and their location is printed.
Native OpenBSD parsing is documented in frontends/docs/legacy-tls.md.
"""
import copy
import argparse
import hashlib
import json
import ipaddress
import os
from pathlib import Path
import re
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[3]
HOSTS = ('joern.buetow.org', 'dory.buetow.org', 'ecat.buetow.org')
PREFIXES = ('', 'www.', 'standby.')

# Render destination templates through the production RenderTemplate engine,
# rather than reconstructing acmeData or substituting template directives.
RENDERER = r'''package main
import("bufio";"encoding/base64";"encoding/json";"fmt";"os";"github.com/snonux/gonf/api")
func main(){f,e:=os.Open(os.Args[1]);if e!=nil{panic(e)};defer f.Close();s:=bufio.NewScanner(f);s.Buffer(make([]byte,1024),4<<20)
for s.Scan(){var d map[string]any;if e=json.Unmarshal(s.Bytes(),&d);e!=nil{panic(e)};if d["op"]!="file"{continue};p,_:=d["path"].(string)
if p!="/etc/acme-client.conf"&&p!="/usr/local/bin/acme.sh"&&p!="/etc/httpd.conf"&&p!="/etc/relayd.conf"&&p!="/etc/gogios.json"{continue}
var c string;if d["template"]==true{c,e=api.RenderTemplate(d["template_param"].(string),d["template_data"])}else{var b []byte;b,e=base64.StdEncoding.DecodeString(d["content_b64"].(string));c=string(b)}
if e!=nil{panic(e)};b,e:=json.Marshal(map[string]string{"path":p,"content":c});if e!=nil{panic(e)};fmt.Println(string(b))};if e=s.Err();e!=nil{panic(e)}}
'''


def validate(config):
    acme = config['/etc/acme-client.conf']
    renewal = config['/usr/local/bin/acme.sh']
    relay = config['/etc/relayd.conf']
    http = config['/etc/httpd.conf']
    monitoring = json.loads(config['/etc/gogios.json'])['Checks']
    public_addresses = {ipaddress.ip_address(address).version: address
                        for address in re.findall(r'listen on (\S+) port 443 tls', relay)}
    assert set(public_addresses) == {4, 6}
    strict = {k: v for k, v in monitoring.items()
              if k.startswith('Check Legacy HTTPS ')}
    assert len(strict) == 18, len(strict)
    for host in HOSTS:
        assert f'domain {host} {{\n\talternative names {{ www.{host} ' in acme
        assert f'domain standby.{host} {{' in acme
        assert f'\n{host} site\nstandby.{host} standby\n' in renewal
        assert f'tls keypair {host}\n' in relay
        assert f'tls keypair standby.{host}\n' in relay
        for prefix in PREFIXES:
            name = prefix + host
            assert f'header "Host" value "{name}" forward to <localhost>' in relay
            # Each alias needs both the HTTP-01 challenge and the real root.
            assert len(re.findall(r'server "' + re.escape(name) + r'" \{', http)) == 2
            assert re.search(r'server "' + re.escape(name) + r'" \{.*?port 80\n.*?root "/acme"', http, re.S)
            for proto in (4, 6):
                matches = [(k, v) for k, v in strict.items()
                           if k.startswith(f'Check Legacy HTTPS IPv{proto} {name} on ')]
                assert len(matches) == 1
                _, check = matches[0]
                args = check['Args']
                assert check['Plugin'].endswith('/check_curl')
                assert '-D' in args and '-S' in args and '--sni' in args
                assert args[args.index('-H') + 1] == name
                address = args[args.index('-I') + 1]
                assert address == public_addresses[proto]
                assert args[args.index('-e') + 1] == 'HTTP/1.1 200'
                assert args[args.index('-C') + 1] == '20,7'
                assert '--continue-after-certificate' in args
                assert check['RunInterval'] == 300
                assert not check.get('OnlyIfNotExists')
                assert check.get('Local') is True
                assert not check.get('DependsOn')


def policy(protocol, headers):
    """Narrow oracle for the reviewed 7.9 rules, not a daemon emulator."""
    hosts = [value for name, value in headers if name.lower() == 'host']
    if len(hosts) > 1:
        return 400
    status = 200
    for line in protocol.splitlines():
        line = line.strip()
        if line == 'block request':
            status = 403
        elif line.startswith('pass request header "Host"'):
            allowed = re.fullmatch(r'pass request header "Host" value "([^\"]+)"(?: forward to <[^>]+>)?', line)
            assert allowed, line
            if hosts == [allowed[1]]:
                status = 200
        elif line.startswith('pass header '):
            field = re.fullmatch(r'pass header "([^\"]+)"', line)
            assert field, line
            if any(name.lower() == field[1].lower() for name, _ in headers):
                status = 200
        elif line.startswith(('pass ', 'block ')):
            raise AssertionError('Unmodeled policy: ' + line)
    return status


def validate_policy(relay):
    protocols = dict(re.findall(r'http protocol "([^\"]+)" \{\n(.*?)\n\}', relay, re.S))
    for protocol in protocols.values():
        assert protocol.count('    block request\n') == 1
        assert '    return error\n' in protocol
        allowed = re.findall(r'pass request header "Host" value "([^\"]+)"', protocol)
        assert len(allowed) == len(set(allowed))
        for name in allowed:
            assert '.lan.' not in name and not any(c in name for c in '*?[]')
            assert policy(protocol, [('Host', name)]) == 200
        websocket = [('Connection', 'Host, Upgrade'), ('Upgrade', 'websocket')]
        for denied in ['unknown.example', 'immich.f3s.lan.buetow.org',
                       'joern.buetow.org.attacker.example', 'JOERN.BUETOW.ORG',
                       'joern.buetow.org:443', 'joern.buetow.org.']:
            assert policy(protocol, [('Host', denied)] + websocket) == 403
        assert policy(protocol, websocket) == 403
        assert policy(protocol, [('Host', allowed[0]), ('hOsT', allowed[0])]) == 400
        # Native relayd starts permissive, so both regressions must be caught.
        assert policy(protocol.replace('    block request\n', ''), [('Host', 'unknown.example')]) == 200
        assert policy(protocol + '\npass header "Connection"', [('Host', 'unknown.example')] + websocket) == 200
    for prefix in PREFIXES:
        assert policy(protocols['https'], [('Host', prefix + 'joern.buetow.org')]) == 200
    assert policy(protocols['forgejo-alt'], [('Host', 'joern.buetow.org')]) == 403


def compare_live(configs, directory):
    """Prove the complete rendered files contain only reviewed Joern deltas."""
    for frontend, config in zip(('blowfish', 'fishfinger'), configs):
        http, relay = (directory / (frontend + '-live-web.txt')).read_text().split('log connection\n', 1)
        relay = 'log connection\n' + relay
        stripped_http = config['/etc/httpd.conf']
        stripped_relay = config['/etc/relayd.conf']
        for prefix in PREFIXES:
            name = prefix + 'joern.buetow.org'
            pattern = r'^server "' + re.escape(name) + r'" \{\n  listen on \* port 80\n.*?^}\n'
            stripped_http, count = re.subn(pattern, '', stripped_http, flags=re.M | re.S)
            assert count == 1
            rule = f'    pass request header "Host" value "{name}" forward to <localhost>\n'
            assert stripped_relay.count(rule) == 1
            stripped_relay = stripped_relay.replace(rule, '')
        for name in ('joern.buetow.org', 'standby.joern.buetow.org'):
            keypair = f'     tls keypair {name}\n'
            assert stripped_relay.count(keypair) == 1
            stripped_relay = stripped_relay.replace(keypair, '')
        assert stripped_http == http, frontend + ' unexpected httpd delta'
        assert stripped_relay == relay, frontend + ' unexpected relayd delta'
        acme, script = (directory / (frontend + '-live-acme.txt')).read_text().split('#!/bin/sh\n', 1)
        stripped_acme, count = re.subn(r'\ndomain joern\.buetow\.org \{.*?\n}\ndomain standby\.joern\.buetow\.org \{.*?\n}\n', '', config['/etc/acme-client.conf'], flags=re.S)
        assert count == 1 and stripped_acme == acme
        stripped_script = config['/usr/local/bin/acme.sh'].replace('joern.buetow.org site\nstandby.joern.buetow.org standby\n', '')
        assert stripped_script == '#!/bin/sh\n' + script
        old = json.loads((directory / (frontend + '-live-gogios.json')).read_text())
        new = json.loads(config['/etc/gogios.json'])
        added = set(new['Checks']) - set(old['Checks'])
        assert len(added) == 29
        assert all(name.startswith('Check Legacy HTTPS ') or 'joern.buetow.org' in name for name in added)
        for name in added:
            del new['Checks'][name]
        assert new == old, frontend + ' unexpected monitoring delta'
        print(f'PASS {frontend}: only 3 HTTP blocks, 2 keypairs, 3 Host rules, 2 ACME domains, 2 renewal rows, 29 added checks')
        print('  unchanged live web SHA256:', hashlib.sha256(http.encode()).hexdigest(), hashlib.sha256(relay.encode()).hexdigest())


def rejected(config):
    try:
        validate(config)
        validate_policy(config['/etc/relayd.conf'])
    except AssertionError:
        return
    raise AssertionError('regression negative control passed unexpectedly')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--compare-live', type=Path, help='directory of read-only public config snapshots')
    args = parser.parse_args()
    scratch = Path(tempfile.mkdtemp(prefix='qo3-render-'))
    env = dict(os.environ, GOFLAGS='-p=1', GOMAXPROCS='2')
    subprocess.run([str(ROOT / 'gonf.sh'), 'plan', '-o', str(scratch / 'plan'),
                    'frontends_acme', 'frontends_httpd', 'frontends_relayd',
                    'frontends_gogios'], env=env, check=True)
    helper = scratch / 'render.go'
    helper.write_text(RENDERER)
    rendered = subprocess.check_output(['go', 'run', str(helper),
                                        str(scratch / 'plan/plan.jsonl')],
                                       cwd=ROOT / 'gonf', env=env, text=True)
    configs = [{}, {}]
    counts = {}
    for line in rendered.splitlines():
        entry = json.loads(line)
        path = entry['path']
        index = counts.get(path, 0)
        assert index < 2
        counts[path] = index + 1
        configs[index][path] = entry['content']
        (scratch / (('blowfish', 'fishfinger')[index] + '-' + Path(path).name)).write_text(entry['content'])
    assert len(counts) == 5 and set(counts.values()) == {2}
    for config in configs:
        validate(config)
        validate_policy(config['/etc/relayd.conf'])
        missing = copy.deepcopy(config)
        missing['/etc/relayd.conf'] = missing['/etc/relayd.conf'].replace(
            'tls keypair joern.buetow.org\n', '')
        rejected(missing)
        missing = copy.deepcopy(config)
        missing['/etc/acme-client.conf'] = missing['/etc/acme-client.conf'].replace(
            'alternative names { www.joern.buetow.org ', 'alternative names { wrong.example ')
        rejected(missing)
        missing = copy.deepcopy(config)
        checks = json.loads(missing['/etc/gogios.json'])
        for name, check in checks['Checks'].items():
            if name.startswith('Check Legacy HTTPS '):
                check['Args'].remove('-D')
                break
        missing['/etc/gogios.json'] = json.dumps(checks)
        rejected(missing)
        for field, value in [('Local', False), ('DependsOn', ['Check Ping4 master.buetow.org'])]:
            missing = copy.deepcopy(config)
            checks = json.loads(missing['/etc/gogios.json'])
            for name, check in checks['Checks'].items():
                if name.startswith('Check Legacy HTTPS '):
                    check[field] = value
                    break
            missing['/etc/gogios.json'] = json.dumps(checks)
            rejected(missing)
        missing = copy.deepcopy(config)
        missing['/etc/relayd.conf'] = missing['/etc/relayd.conf'].replace('    block request\n', '')
        rejected(missing)
        missing = copy.deepcopy(config)
        missing['/etc/relayd.conf'] = missing['/etc/relayd.conf'].replace(
            '    block request\n', '    block request\n    pass request header "Host" value "unknown.example"\n')
        rejected(missing)
    if args.compare_live:
        compare_live(configs, args.compare_live)
    print('PASS: actual rendered TLS/HTTP-01/Host/36 strict checks; missing keypair, SAN and verification negatives')
    print(f'Retained synthetic render fixture: {scratch}')


if __name__ == '__main__':
    main()
