#!/usr/bin/env python3
"""Exercise real Gonf renderers and reject missing TLS/alias/verification policy.

Uses plan only: no deployment, CA request, production key access or daemon start.
Temporary plan/template files are retained and their location is printed.
Native OpenBSD parsing is documented in frontends/docs/legacy-tls.md.
"""
import copy
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


def rejected(config):
    try:
        validate(config)
    except AssertionError:
        return
    raise AssertionError('regression negative control passed unexpectedly')


def main():
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
    print('PASS: actual rendered TLS/HTTP-01/Host/36 strict checks; missing keypair, SAN and verification negatives')
    print(f'Retained synthetic render fixture: {scratch}')


if __name__ == '__main__':
    main()
