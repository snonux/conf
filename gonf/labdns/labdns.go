// Package labdns names the DNS resolver every lab host uses (task u54).
//
// The freebsd, netbsd and rocky Resolver recipes all render this one address,
// so a later resolver move is a one-line change here.
package labdns

// Resolver is triggerfish's unbound on the lab gateway address. It forwards
// to public resolvers (9.9.9.9, 1.1.1.1) and admits 10.20.0.0/24.
//
// Why this address and nothing else:
//   - The ISP CPE 192.168.1.1, the former resolver, is unreachable from the
//     lab once the b24 cable move leaves it alone on router port igc0.
//   - 10.20.0.1 answers the lab on igc0 today and on igc1 after c24 moves
//     the lab prefix, with the same PF and unbound rules.
//   - It answers like the CPE did: public names, the public *.f3s.lan and
//     *.lan.buetow.org records, no ad blocking. The fleet names stay in
//     /etc/hosts.
//   - No k3s dependency. The Pi-holes on pi2/pi3 are no fallback: CoreDNS
//     forwards to the node's resolv.conf and picks an upstream at random,
//     so pods would get ad-blocked answers some of the time. Pi-hole also
//     forwards public names through the router, so a fallback would only
//     help when unbound alone is down.
const Resolver = "10.20.0.1"
