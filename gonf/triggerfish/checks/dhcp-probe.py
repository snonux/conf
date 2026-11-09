#!/usr/bin/env python3
"""Send one DHCPDISCOVER (optionally a REQUEST) on a wire and print the reply.

Linux only, root (AF_PACKET). Stdlib only, so it runs on a Rocky lab host or
the controller laptop without installing anything. The probe never changes
the host's own addresses: frames are built and read raw, nothing binds UDP 68
and no lease is applied. The BOOTP broadcast flag is set, so the server
answers to ff:ff:ff:ff:ff:ff instead of unicasting to CHADDR, which may be
another device's MAC (a reservation check never disturbs that device; a
DISCOVER alone commits no lease).

Usage:
  dhcp-probe.py IFACE CHADDR discover
  dhcp-probe.py IFACE CHADDR request REQUESTED_IP SERVER_ID
Prints one JSON object: the reply type (OFFER/ACK/NAK or none), yiaddr,
server identifier, router, DNS and the frame's source addresses. Exit 0 when
any reply arrived, 1 when none did within the timeout (5 s).
"""
import json
import os
import socket
import struct
import sys
import time

TYPES = {1: 'DISCOVER', 2: 'OFFER', 3: 'REQUEST', 5: 'ACK', 6: 'NAK'}


def checksum(data):
    s = sum(struct.unpack(f'!{len(data) // 2}H', data))
    s = (s >> 16) + (s & 0xffff)
    return ~(s + (s >> 16)) & 0xffff


def bootp(mac, xid, options):
    head = struct.pack('!BBBBIHH4s4s4s4s16s64s128s', 1, 1, 6, 0, xid, 0, 0x8000,
                       bytes(4), bytes(4), bytes(4), bytes(4), mac.ljust(16, b'\0'), b'', b'')
    body = head + b'\x63\x82\x53\x63' + options + b'\xff'
    return body.ljust(300, b'\0')


def frame(src_mac, payload):
    udp = struct.pack('!HHHH', 68, 67, 8 + len(payload), 0) + payload
    ip = struct.pack('!BBHHHBBH4s4s', 0x45, 0, 20 + len(udp), 0, 0, 64, 17, 0,
                     bytes(4), b'\xff' * 4)
    ip = ip[:10] + struct.pack('!H', checksum(ip)) + ip[12:]
    return b'\xff' * 6 + src_mac + b'\x08\x00' + ip + udp


def parse_options(data):
    opts, i = {}, 0
    while i < len(data) and data[i] != 255:
        if data[i] == 0:
            i += 1
            continue
        opts[data[i]] = data[i + 2:i + 2 + data[i + 1]]
        i += 2 + data[i + 1]
    return opts


def reply(raw, xid):
    if len(raw) < 14 + 20 + 8 + 240 or raw[12:14] != b'\x08\x00' or raw[23] != 17:
        return None
    ihl = (raw[14] & 0x0f) * 4
    udp = 14 + ihl
    if struct.unpack('!H', raw[udp + 2:udp + 4])[0] != 68:
        return None
    b = raw[udp + 8:]
    if b[0] != 2 or struct.unpack('!I', b[4:8])[0] != xid:
        return None
    opts = parse_options(b[240:])
    ip4 = lambda v: socket.inet_ntoa(v[:4]) if v else None
    return {'type': TYPES.get(opts.get(53, b'\0')[0], 'other'), 'yiaddr': socket.inet_ntoa(b[16:20]),
            'server_id': ip4(opts.get(54)), 'router': ip4(opts.get(3)), 'dns': ip4(opts.get(6)),
            'src_ip': socket.inet_ntoa(raw[26:30]), 'src_mac': raw[6:12].hex(':')}


def main():
    iface, chaddr, mode = sys.argv[1:4]
    mac = bytes.fromhex(chaddr.replace(':', ''))
    xid = int.from_bytes(os.urandom(4), 'big')
    options = b'\x37\x04\x01\x03\x06\x36'  # parameter request: mask, router, DNS, server id
    if mode == 'discover':
        options = b'\x35\x01\x01' + options
    else:
        options = (b'\x35\x01\x03' + b'\x32\x04' + socket.inet_aton(sys.argv[4]) +
                   b'\x36\x04' + socket.inet_aton(sys.argv[5]) + options)
    s = socket.socket(socket.AF_PACKET, socket.SOCK_RAW, socket.htons(0x0003))
    s.bind((iface, 0))
    s.settimeout(0.5)
    own = s.getsockname()[4]
    s.send(frame(own, bootp(mac, xid, options)))
    res, end = None, time.time() + 5
    while res is None and time.time() < end:
        try:
            res = reply(s.recv(2048), xid)
        except socket.timeout:
            pass
    out = {'iface': iface, 'chaddr': chaddr, 'mode': mode, 'xid': f'{xid:08x}', 'reply': res or 'none'}
    print(json.dumps(out))
    sys.exit(0 if res else 1)


if __name__ == '__main__':
    main()
