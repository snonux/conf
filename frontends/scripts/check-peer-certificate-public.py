#!/usr/bin/env python3
"""Read-only strict TLS/SNI, public HTTP and denied-Host acceptance for ro3.

Default records the present state; --expect-refreshed checks reviewed owner pins
and >=30 days on the selected frontend. Re-review pins after any owner renewal.
Only public certificate metadata, status and body hashes are printed.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import http.client
import json
import socket
import ssl

HOSTS = {'blowfish': ('23.88.35.144', '2a01:4f8:c17:20f1::42'),
         'fishfinger': ('46.23.94.99', '2a03:6000:6f67:624::99')}
PINS = {
    'standby.ecat.buetow.org': '567fc8880dca99d274a18d0aad5814e87e911ae2dedc1142b7ab0a1706a84701',
    'standby.alt.irregular.ninja': 'd5baa12feb85607960a88d25c49e4fa4582d7e3586dbe610c33ff54155051266',
    'standby.gogios.buetow.org': '6c9f917268a4bf74593a7544af733067f958af45725681d7b7464bde3aac4bd4',
    'gogios.buetow.org': 'dfa7af738d6717c67645fe8d72d1fab92eda6ca7f2ea8a1e5f1da90e2b763e46',
    'www.gogios.buetow.org': 'dfa7af738d6717c67645fe8d72d1fab92eda6ca7f2ea8a1e5f1da90e2b763e46',
}


def request(context, address, name, headers):
    with socket.create_connection((address, 443), timeout=7) as raw:
        with context.wrap_socket(raw, server_hostname=name) as tls:
            certificate = tls.getpeercert()
            pin = hashlib.sha256(tls.getpeercert(binary_form=True)).hexdigest()
            tls.sendall(('GET / HTTP/1.1\r\n' + headers +
                         'Connection: close\r\n\r\n').encode('ascii'))
            response = http.client.HTTPResponse(tls)
            response.begin()
            body = response.read(8 * 1024 * 1024 + 1)
            assert len(body) <= 8 * 1024 * 1024, 'public response exceeds fixture bound'
            return certificate, pin, response.status, response.getheader('Strict-Transport-Security'), body


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--host', choices=(*HOSTS, 'all'), default='all')
    parser.add_argument('--expect-refreshed', action='store_true')
    args = parser.parse_args()
    context = ssl.create_default_context()
    for host, addresses in HOSTS.items():
        if args.host not in ('all', host):
            continue
        for address in addresses:
            for name, expected in PINS.items():
                cert, pin, status, hsts, body = request(context, address, name, 'Host: ' + name + '\r\n')
                expiry = datetime.fromtimestamp(ssl.cert_time_to_seconds(cert['notAfter']), timezone.utc)
                if args.expect_refreshed:
                    assert pin == expected, (host, name, 'reviewed public leaf pin mismatch')
                    assert (expiry - datetime.now(timezone.utc)).total_seconds() >= 2592000
                assert status == 200, (host, name, status)
                assert hsts == 'max-age=31536000', (host, name, 'HSTS changed')
                print(json.dumps({'host': host, 'address': address, 'name': name,
                                  'sha256': pin, 'expiry_utc': expiry.isoformat(),
                                  'sans': cert['subjectAltName'], 'http_status': status,
                                  'body_sha256': hashlib.sha256(body).hexdigest()}))
            for label, headers, expected in (
                    ('unknown', 'Host: unknown.example\r\n', 403),
                    ('LAN', 'Host: immich.f3s.lan.buetow.org\r\n', 403),
                    ('missing', '', 403),
                    ('duplicate', 'Host: gogios.buetow.org\r\nhOsT: gogios.buetow.org\r\n', 400)):
                _, _, status, _, _ = request(context, address, 'gogios.buetow.org', headers)
                assert status == expected, (host, label, status)
                print(json.dumps({'host': host, 'address': address, 'denied_host': label, 'http_status': status}))


if __name__ == '__main__':
    main()
