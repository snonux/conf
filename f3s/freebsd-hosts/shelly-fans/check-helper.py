#!/usr/bin/env python3
"""Exercise the real shelly-fans-on helper against local TLS test servers.

Never touches the real plugs: every case runs "shelly-fans-on status" (the
read-only path) against a throwaway CA, leaf and HTTPS server on 127.0.0.1.
Needs curl (OpenSSL backend, --pinnedpubkey) and openssl >= 3.4 (-not_before)
on the controller. Run from anywhere: python3 -I check-helper.py

Cases (p34 review B1):
  1. normal clock: --cacert verification succeeds, no pinned-key fallback;
  2. clock before the leaf's notBefore: curl exits 60, the pinned key is used
     and succeeds (simulated with a leaf whose notBefore lies in the future);
  3. same, but the pin names another key: refused even on the pin path (90);
  4. certificate rejected (wrong CA) with a plausible clock: no fallback (60);
  5. clock implausible but no well-formed pin: no -k attempt at all (60).
"""
import base64, hashlib, http.server, os, ssl, subprocess, sys, tempfile, threading, time
from pathlib import Path

HELPER = Path(__file__).resolve().parent / 'shelly-fans-on'
PINNED_NOTE = 'using pinned key'


def sh(*args, **kw):
    return subprocess.run(args, check=True, capture_output=True, text=True, **kw)


def make_ca(d, name):
    sh('openssl', 'ecparam', '-name', 'prime256v1', '-genkey', '-noout', '-out', f'{d}/{name}.key')
    sh('openssl', 'req', '-x509', '-new', '-key', f'{d}/{name}.key', '-subj', f'/CN={name}', '-days', '2',
       '-addext', 'basicConstraints=critical,CA:TRUE', '-out', f'{d}/{name}.crt')


def make_leaf(d, name, ca, not_before=None):
    sh('openssl', 'ecparam', '-name', 'prime256v1', '-genkey', '-noout', '-out', f'{d}/{name}.key')
    sh('openssl', 'req', '-new', '-key', f'{d}/{name}.key', '-subj', '/CN=plug', '-out', f'{d}/{name}.csr')
    Path(f'{d}/{name}.ext').write_text('basicConstraints=critical,CA:FALSE\nsubjectAltName=IP:127.0.0.1\n')
    args = ['openssl', 'x509', '-req', '-in', f'{d}/{name}.csr', '-CA', f'{d}/{ca}.crt', '-CAkey', f'{d}/{ca}.key',
            '-extfile', f'{d}/{name}.ext', '-out', f'{d}/{name}.crt']
    args += ['-not_before', not_before, '-days', '30'] if not_before else ['-days', '2']
    sh(*args)


def spki_pin(cert):
    der = subprocess.run(f'openssl x509 -in {cert} -pubkey -noout | openssl pkey -pubin -outform der',
                         shell=True, check=True, capture_output=True).stdout
    return 'sha256//' + base64.b64encode(hashlib.sha256(der).digest()).decode()


def not_before_epoch(cert):
    out = sh('openssl', 'x509', '-in', cert, '-noout', '-startdate').stdout.split('=', 1)[1].strip()
    return int(time.mktime(time.strptime(out, '%b %d %H:%M:%S %Y %Z')) - time.timezone)


class Plug(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body = b'{"id":0,"output":true}'
        self.send_response(200)
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *a):
        pass


class QuietServer(http.server.ThreadingHTTPServer):
    def handle_error(self, request, client_address):
        pass  # rejected handshakes (cases 3-5) are expected


def serve(cert, key):
    srv = QuietServer(('127.0.0.1', 0), Plug)
    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    ctx.load_cert_chain(cert, key)
    srv.socket = ctx.wrap_socket(srv.socket, server_side=True)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    return srv


def helper(d, port, ca, pin, not_before):
    Path(f'{d}/pin').write_text(f'SHELLY_PIN={pin}\nSHELLY_PIN_NOT_BEFORE={not_before}\n')
    env = {'SHELLY_IP': f'127.0.0.1:{port}', 'SHELLY_CA_FILE': ca, 'SHELLY_PIN_FILE': f'{d}/pin',
           'SHELLY_PASS_FILE': f'{d}/no-password'}
    return subprocess.run(['/bin/sh', str(HELPER), 'status'], env=env, capture_output=True, text=True, timeout=60)


def expect(name, r, rc, out_ok, pinned):
    ok = r.returncode == rc and (('"output":true' in r.stdout) == out_ok) and ((PINNED_NOTE in r.stderr) == pinned)
    print(('PASS' if ok else 'FAIL'), name, f'rc={r.returncode}', r.stderr.strip().replace('\n', ' | '))
    return ok


with tempfile.TemporaryDirectory(prefix='shelly-helper-check-') as d:
    make_ca(d, 'ca')
    make_ca(d, 'other')
    make_leaf(d, 'now', 'ca')
    future = time.strftime('%Y%m%d%H%M%SZ', time.gmtime(time.time() + 86400))
    make_leaf(d, 'future', 'ca', not_before=future)
    make_leaf(d, 'stranger', 'ca')
    now_srv, future_srv = serve(f'{d}/now.crt', f'{d}/now.key'), serve(f'{d}/future.crt', f'{d}/future.key')
    now_port, future_port = now_srv.server_address[1], future_srv.server_address[1]
    past = not_before_epoch(f'{d}/now.crt')
    ahead = not_before_epoch(f'{d}/future.crt')
    assert ahead > time.time() > past
    results = [
        expect('1 normal clock uses --cacert', helper(d, now_port, f'{d}/ca.crt', spki_pin(f'{d}/now.crt'), past),
               0, True, False),
        expect('2 clock before notBefore uses the pinned key',
               helper(d, future_port, f'{d}/ca.crt', spki_pin(f'{d}/future.crt'), ahead), 0, True, True),
        expect('3 pinned path refuses another key',
               helper(d, future_port, f'{d}/ca.crt', spki_pin(f'{d}/stranger.crt'), ahead), 90, False, True),
        expect('4 wrong CA with a plausible clock: no fallback',
               helper(d, now_port, f'{d}/other.crt', spki_pin(f'{d}/now.crt'), past), 60, False, False),
        expect('5 no well-formed pin: no -k attempt',
               helper(d, future_port, f'{d}/ca.crt', 'not-a-pin', ahead), 60, False, False),
    ]
    now_srv.shutdown()
    future_srv.shutdown()
sys.exit(0 if all(results) else 1)
