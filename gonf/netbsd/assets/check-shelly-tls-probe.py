#!/usr/bin/env python3
"""Exercise the real shelly-tls-probe against local TLS servers (task s44).

Never touches the real plugs or the cluster: every case runs the probe
(ksh, as on pi0/pi1) against a throwaway CA, leaves and TLS servers on
127.0.0.1, and pushes only to a fake Pushgateway on 127.0.0.1. Needs ksh,
openssl >= 3.4 (-not_before/-not_after), curl and timeout. With PROMTOOL
set (a promtool binary or wrapper), the pushed document is also checked by
`promtool check metrics`. Run from anywhere: python3 -I check-shelly-tls-probe.py

Positive: a valid leaf verifies, a near-expiry leaf still verifies and
reports its real notAfter, the push lands as a PUT on the reporter's group
and falls back to the next address. Negative: expired and not-yet-valid
leaves (genuine, so notAfter is still reported), wrong CA, wrong IP SAN,
clientAuth-only leaf, an untrusted CA offered via SSL_CERT_FILE/SSL_CERT_DIR
(never consulted), unreachable plug, a plug that never finishes the
handshake (bounded by the timeout), missing CA file, push failure on every
address, and usage errors.
"""
import http.server
import os
import re
import socket
import ssl
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path

PROBE = Path(__file__).resolve().with_name('shelly-tls-probe')
DAY = 86400
SAMPLE = re.compile(r'^([a-z_]+)(\{[a-z_]+="[^"]*"(,[a-z_]+="[^"]*")*\})? -?[0-9]+$')


def sh(*args):
    return subprocess.run(args, check=True, capture_output=True, text=True)


def make_ca(d, name):
    sh('openssl', 'ecparam', '-name', 'prime256v1', '-genkey', '-noout', '-out', f'{d}/{name}.key')
    sh('openssl', 'req', '-x509', '-new', '-key', f'{d}/{name}.key', '-subj', f'/CN={name}',
       '-days', '30', '-addext', 'basicConstraints=critical,CA:TRUE', '-out', f'{d}/{name}.crt')


def make_leaf(d, name, ca, ip='127.0.0.1', eku='serverAuth', days=100, not_before=None, not_after=None):
    sh('openssl', 'ecparam', '-name', 'prime256v1', '-genkey', '-noout', '-out', f'{d}/{name}.key')
    sh('openssl', 'req', '-new', '-key', f'{d}/{name}.key', '-subj', '/CN=plug', '-out', f'{d}/{name}.csr')
    Path(f'{d}/{name}.ext').write_text(
        f'basicConstraints=critical,CA:FALSE\nsubjectAltName=IP:{ip}\nextendedKeyUsage={eku}\n')
    args = ['openssl', 'x509', '-req', '-in', f'{d}/{name}.csr', '-CA', f'{d}/{ca}.crt',
            '-CAkey', f'{d}/{ca}.key', '-extfile', f'{d}/{name}.ext', '-out', f'{d}/{name}.crt']
    if not_before or not_after:
        args += ['-not_before', not_before, '-not_after', not_after]
    else:
        args += ['-days', str(days)]
    sh(*args)
    return f'{d}/{name}.crt', f'{d}/{name}.key'


def not_after(cert):
    out = sh('openssl', 'x509', '-in', cert, '-noout', '-enddate').stdout.split('=', 1)[1].strip()
    return int(time.mktime(time.strptime(out, '%b %d %H:%M:%S %Y %Z')) - time.timezone)


class Quiet(http.server.BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass


class QuietServer(http.server.ThreadingHTTPServer):
    def handle_error(self, request, client_address):
        pass  # s_client hangs up without a request; rejected handshakes


def tls_plug(cert, key):
    srv = QuietServer(('127.0.0.1', 0), Quiet)
    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    ctx.maximum_version = ssl.TLSVersion.TLSv1_2  # like the Shelly firmware
    ctx.load_cert_chain(cert, key)
    srv.socket = ctx.wrap_socket(srv.socket, server_side=True)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    return srv


def silent_plug():
    """Accepts TCP but never answers the ClientHello."""
    sock = socket.socket()
    sock.bind(('127.0.0.1', 0))
    sock.listen(4)
    held = []
    threading.Thread(target=lambda: held.extend(sock.accept() for _ in range(4)), daemon=True).start()
    return sock


def closed_port():
    sock = socket.socket()
    sock.bind(('127.0.0.1', 0))
    port = sock.getsockname()[1]
    sock.close()
    return port


class Gateway(Quiet):
    pushes = []
    status = 200

    def do_PUT(self):
        body = self.rfile.read(int(self.headers['Content-Length'])).decode()
        type(self).pushes.append((self.path, body))
        self.send_response(type(self).status)
        self.send_header('Content-Length', '0')
        self.end_headers()


def gateway(status):
    handler = type('G', (Gateway,), {'pushes': [], 'status': status})
    srv = QuietServer(('127.0.0.1', 0), handler)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    return srv, handler


def parse(text):
    """Validate the exposition format and return {(name, labels): value}."""
    samples, seen, current = {}, set(), None
    for line in text.splitlines():
        if line.startswith('# HELP '):
            current = line.split()[2]
            assert current not in seen, f'family {current} twice'
            seen.add(current)
            continue
        if line.startswith('# TYPE '):
            assert line.split()[2:] == [current, 'gauge'], line
            continue
        match = SAMPLE.match(line)
        assert match and match.group(1) == current, f'bad or misplaced sample: {line!r}'
        labels = tuple(sorted(re.findall(r'([a-z_]+)="([^"]*)"', match.group(2) or '')))
        samples[(match.group(1), labels)] = int(line.rsplit(' ', 1)[1])
    return samples


def run(env, *args, timeout=60):
    started = time.time()
    r = subprocess.run(['ksh', str(PROBE), *args], env=env, capture_output=True, text=True, timeout=timeout)
    return r, time.time() - started


def key(name, **labels):
    return (name, tuple(sorted(labels.items())))


def main():
    results = []

    def check(name, ok, detail=''):
        results.append(ok)
        print('PASS' if ok else 'FAIL', name, detail if not ok else '')

    with tempfile.TemporaryDirectory(prefix='shelly-tls-probe-check-') as d:
        make_ca(d, 'ca')
        make_ca(d, 'other')
        certs = {
            'good': make_leaf(d, 'good', 'ca'),
            'near': make_leaf(d, 'near', 'ca', days=10),
            'expired': make_leaf(d, 'expired', 'ca', not_before='20200101000000Z', not_after='20200201000000Z'),
            'future': make_leaf(d, 'future', 'ca', not_before='20990101000000Z', not_after='20990201000000Z'),
            'stranger': make_leaf(d, 'stranger', 'other'),
            'wrongip': make_leaf(d, 'wrongip', 'ca', ip='127.0.0.2'),
            'client': make_leaf(d, 'client', 'ca', eku='clientAuth'),
        }
        plugs = {name: tls_plug(*pair).server_address[1] for name, pair in certs.items()}
        plugs['silent'] = silent_plug().getsockname()[1]
        plugs['closed'] = closed_port()
        trust = Path(d, 'systrust')
        trust.mkdir()
        (trust / 'other.pem').write_text(Path(d, 'other.crt').read_text())
        sh('openssl', 'rehash', str(trust))
        ok_gw, ok_handler = gateway(200)
        bad_gw, _ = gateway(500)
        state = Path(d, 'state')
        state.mkdir()
        base = {'PATH': os.environ['PATH'], 'SHELLY_TLS_CA_FILE': f'{d}/ca.crt',
                'SHELLY_TLS_STATE_DIR': str(state), 'SHELLY_TLS_REPORTER': 'pi9',
                'SHELLY_TLS_TIMEOUT': '4', 'TMPDIR': d,
                'SHELLY_TLS_PUSH_URLS': f'http://127.0.0.1:{closed_port()} http://127.0.0.1:{ok_gw.server_address[1]}'}

        def probe(*plug_names, extra=None, args=('-n',)):
            env = dict(base, SHELLY_TLS_PLUGS=' '.join(f'{p}=127.0.0.1:{plugs[p]}' for p in plug_names))
            env.update(extra or {})
            return run(env, *args)

        ca_after = not_after(f'{d}/ca.crt')

        r, _ = probe('good')
        m = parse(r.stdout)
        first_success = m.get(key('shelly_tls_probe_last_success_timestamp_seconds', plug='good'), 0)
        check('1 valid leaf verifies', r.returncode == 0
              and m[key('shelly_tls_probe_success', plug='good', address='127.0.0.1')] == 1
              and m[key('shelly_tls_cert_not_after_seconds', plug='good', cert='leaf')] == not_after(certs['good'][0])
              and m[key('shelly_tls_cert_not_after_seconds', cert='ca')] == ca_after
              and abs(first_success - time.time()) < 60
              and abs(m[key('shelly_tls_probe_last_run_timestamp_seconds')] - time.time()) < 60,
              r.stdout + r.stderr)

        r, _ = probe('near')
        m = parse(r.stdout)
        left = m.get(key('shelly_tls_cert_not_after_seconds', plug='near', cert='leaf'), 0) - time.time()
        check('2 near-expiry leaf verifies and reports ~10 days', r.returncode == 0
              and m[key('shelly_tls_probe_success', plug='near', address='127.0.0.1')] == 1
              and 9 * DAY < left <= 10 * DAY, f'{left} {r.stderr}')

        # The plug named "good" now presents the expired leaf, with the same
        # state directory: its recorded last success must not move.
        env = dict(base, SHELLY_TLS_PLUGS=f'good=127.0.0.1:{plugs["expired"]}')
        time.sleep(1.1)
        r, _ = run(env, '-n')
        m = parse(r.stdout)
        check('3 expired genuine leaf: fails, notAfter kept, last success unchanged', r.returncode == 1
              and m[key('shelly_tls_probe_success', plug='good', address='127.0.0.1')] == 0
              and m[key('shelly_tls_cert_not_after_seconds', plug='good', cert='leaf')] == 1580515200
              and m[key('shelly_tls_probe_last_success_timestamp_seconds', plug='good')] == first_success
              and 'outside its validity period' in r.stderr, r.stdout + r.stderr)

        r, _ = probe('future')
        m = parse(r.stdout)
        check('4 not-yet-valid genuine leaf fails, never succeeded = 0', r.returncode == 1
              and m[key('shelly_tls_probe_success', plug='future', address='127.0.0.1')] == 0
              and m[key('shelly_tls_cert_not_after_seconds', plug='future', cert='leaf')] == not_after(certs['future'][0])
              and m[key('shelly_tls_probe_last_success_timestamp_seconds', plug='future')] == 0,
              r.stdout + r.stderr)

        for n, plug, why in [('5', 'stranger', 'wrong CA'), ('6', 'wrongip', 'wrong IP SAN'),
                             ('7', 'client', 'clientAuth-only leaf')]:
            r, _ = probe(plug)
            m = parse(r.stdout)
            check(f'{n} {why}: fails, no leaf notAfter', r.returncode == 1
                  and m[key('shelly_tls_probe_success', plug=plug, address='127.0.0.1')] == 0
                  and key('shelly_tls_cert_not_after_seconds', plug=plug, cert='leaf') not in m
                  and 'does not verify' in r.stderr, r.stdout + r.stderr)

        r, _ = probe('stranger', extra={'SSL_CERT_FILE': f'{d}/other.crt', 'SSL_CERT_DIR': str(trust)})
        m = parse(r.stdout)
        check('8 system trust (SSL_CERT_FILE/DIR = the other CA) is never used', r.returncode == 1
              and m[key('shelly_tls_probe_success', plug='stranger', address='127.0.0.1')] == 0,
              r.stdout + r.stderr)

        r, took = probe('closed')
        m = parse(r.stdout)
        check('9 unreachable plug fails fast', r.returncode == 1 and took < 10
              and m[key('shelly_tls_probe_success', plug='closed', address='127.0.0.1')] == 0
              and 'handshake failed' in r.stderr, f'{took:.1f}s {r.stderr}')

        r, took = probe('silent')
        m = parse(r.stdout)
        check('10 silent plug is bounded by the timeout', r.returncode == 1 and 3.5 < took < 12
              and m[key('shelly_tls_probe_success', plug='silent', address='127.0.0.1')] == 0,
              f'{took:.1f}s {r.stderr}')

        r, _ = probe('good', 'stranger')
        m = parse(r.stdout)
        check('11 mixed plugs are reported per plug', r.returncode == 1
              and m[key('shelly_tls_probe_success', plug='good', address='127.0.0.1')] == 1
              and m[key('shelly_tls_probe_success', plug='stranger', address='127.0.0.1')] == 0,
              r.stdout + r.stderr)

        r, _ = probe('good', extra={'SHELLY_TLS_CA_FILE': f'{d}/missing.pem'})
        m = parse(r.stdout)
        check('12 missing CA file: fails closed, no CA notAfter', r.returncode == 1
              and m[key('shelly_tls_probe_success', plug='good', address='127.0.0.1')] == 0
              and key('shelly_tls_cert_not_after_seconds', cert='ca') not in m
              and 'missing or unreadable' in r.stderr, r.stdout + r.stderr)

        r, _ = probe('good', 'near', args=())
        path, body = ok_handler.pushes[-1] if ok_handler.pushes else ('', '')
        m = parse(body) if body else {}
        check('13 push: PUT to the reporter group, falls back past a dead address', r.returncode == 0
              and len(ok_handler.pushes) == 1 and path == '/metrics/job/shelly_tls/reporter/pi9'
              and m.get(key('shelly_tls_probe_success', plug='near', address='127.0.0.1')) == 1
              and 'push to http://127.0.0.1:' in r.stderr, f'{path} {r.stderr}')
        promtool = os.environ.get('PROMTOOL')
        if promtool and body:
            lint = subprocess.run([promtool, 'check', 'metrics'], input=body, capture_output=True, text=True)
            check('13b promtool check metrics accepts the pushed document', lint.returncode == 0,
                  lint.stdout + lint.stderr)

        r, _ = probe('good', args=(), extra={
            'SHELLY_TLS_PUSH_URLS': f'http://127.0.0.1:{closed_port()} http://127.0.0.1:{bad_gw.server_address[1]}'})
        check('14 push refused everywhere exits 2', r.returncode == 2
              and 'push failed on every Pushgateway address' in r.stderr, r.stderr)

        for n, extra, args in [('15a', {'SHELLY_TLS_REPORTER': 'pi0/../x'}, ('-n',)),
                               ('15b', {'SHELLY_TLS_STATE_DIR': f'{d}/nope'}, ('-n',)),
                               ('15c', {}, ('-x',)),
                               ('15d', {'SHELLY_TLS_TIMEOUT': '1s'}, ('-n',))]:
            r, _ = probe('good', extra=extra, args=args)
            check(f'{n} usage/config error exits 64 without output', r.returncode == 64 and not r.stdout,
                  f'rc={r.returncode} {r.stderr}')

        leftovers = [p for p in Path(d).glob('shelly-tls-probe.*')]
        check('16 scratch directories are removed', not leftovers, str(leftovers))
    print(f'{sum(results)}/{len(results)} passed')
    return 0 if all(results) else 1


if __name__ == '__main__':
    sys.exit(main())
