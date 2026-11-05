# IPvlan L2 — first runtime proved, remaining native gates open

The two macvlan runtime trials failed fixed-channel usability and passed
independent expiry/cleanup. v26 subsequently passed IPvlan fixed HTTP/SSH/media and independent expiry.
This is first-device runtime acceptance only; no primary transition follows.
The [runtime evidence](NATIVE-RUNTIME-2026-11-05.md) remains authoritative.

Root-approved read-only v20 on the same first device observed kernel 5.10.160
armv7l and actual driver `rk_gmac-dwmac`, platform `/ethernet@ffa80000`, compatible
`rockchip,rv1106-gmac` / `snps,dwmac-4.20a`. No driver/module symlink was present.
The regular root-owned OEM module files were:

| Module | Bytes | SHA256 |
| --- | ---: | --- |
| ipvlan.ko | 24960 | `8e076dc464de7291124530fafca4d83d079471cf34164752927a5938dda4bc39` |
| macvlan.ko | 19596 | `2ab8771638a2db37173b96fcb736109a55fcdc58a79f29b9c04debc982915ec1` |

Both were unloaded; both modules.dep rows had no dependencies. Parent flags
0x1003, MTU1500 and link-up state remained unchanged. During the three-second
read-only interval RX/TX counters increased without new error/drop counters;
the historical rx_dropped=4000 did not increase and does not establish a cause.
Original developer access was closed and fresh HTTP/WebRTC/USB/video checks
passed. Presence/hash is not loadability or IPvlan network proof.

Pinned system source `dc2b6d967fc16aa59b412ae306eb34f4c1ec3d0b` enables
CONFIG_IPVLAN=m. Its `drivers/net/ipvlan/ipvlan_main.c` defaults new links to L3,
inherits the physical MAC, accepts u16 IFLA_IPVLAN_MODE and exports that mode.
The pinned BusyBox1.36.1 tar SHA256 is
`b8cc24c9574d809e7279c3be349795c5d5ceb6fdf19ca709f80cde50e47de314`;
its iplink parser accepts arbitrary type names but only parses type options for
vlan/vrf. A bare IPvlan create, or an ignored `mode l2` argument, is insufficient.

The source candidate adds optional `kind`, empty by default with original JSON
bytes/hash and macvlan behavior preserved. Only explicit macvlan/ipvlan are
accepted. IPvlan requires the expected factory parent MAC and actual eth0 match
before mutation. It selects the fixed IPvlan module path/query/unload, preserves
pre-existing/foreign modules, and emits one fixed RTM_NEWLINK request with L2=0,
CREATE|EXCL|ACK, exact parent index and child name. It does not set MAC or parent
flags. Typed readback must observe actual L2, matching parent/name/index/MAC and
positive child index before proceeding. All IPs, policy table/priority, workers,
authorization/deadline, expiry and boot protocols remain unchanged. Any pre-existing eth0 upper-link relationship is refused before creation because
IPvlan mode belongs to the shared physical port. Root must require globally absent IPvlan/macvlan modules, absent parent upper
links and serialized writers before the first kernel trial. Global module
absence excludes an existing IPvlan port in any namespace; namespace APIs were
unavailable and are not claimed verified. No namespace probing is planned. No fallback
to L3, promisc, ARP policy change or vendor configuration write is included.

## Completed file-only installation

Root reviewed and v25 completed this file-only transition. See the
[native receipt](NATIVE-RUNTIME-2026-11-05.md#ipvlan-prerequisite-and-file-only-installation).
The separate v26 runtime/expiry trial then passed; confirmation/owner/boot trials
still require source review and root coordination.
The preserved installation protocol was:

1. Freeze all Jet writers. Use the proven temporary access lifecycle, original
   enable-future terminal cleanup and fresh old-path postchecks. Before any file
   change verify exact CPU/ABI/ext4, original parent semantic baseline, absent
   child/table1000/rule1140, both modules unloaded, no hook/primary/receipt, and
   native IPvlan file hash above. Reject every unexpected path/symlink/owner/mode.
2. Validate the existing seven-file private store against saved exact bytes and
   hashes: binary/profile, lock, authorization/active/owned/worker. Active must
   be same-boot expired, unconfirmed, without boot allowance; owned intents must
   all be empty and worker PID/birth gone. Preserve the original generation and
   authorization bytes. Refuse pending, foreign or unknown files.
3. Copy the complete old terminal store into an exclusive controller-private
   archive with hashes and durable completion. Under the old stable lock, with
   writers frozen and no hooks, atomically rename the whole old device directory
   to a reviewed unique sibling on the same userdata filesystem. Keep all seven
   files intact; do not delete active/owned/lock or transplant their old hashes.
   The archived path is inert evidence, never a startup source.
4. Exclusively create a fresh default directory0700, new binary0700 and
   profile0600. Candidate profile changes only explicit kind=ipvlan and MAC to
   actual factory `30:52:53:08:97:97`; retain device e10a0883debf7d4a, shared-igc0,
   eth0/jkrecover, .11.140/24, gateway .11.2, controller10.99.0.2/32, table1000,
   priority1140. Prepared profile is 242 bytes, SHA256
   `2815fbf4031cda9346619e76b9037a29c0ce2f544fdb12a20f8bcb05913a3d91`.
   Root must review these exact bytes and inventory implication.
   Copy via stdin, verify inode/root ownership/modes/SHA, run only `-version`.
   No authorization/state/kernel resource is created by file-only installation.
5. On staging failure remove only newly created inode-owned named files, then
   rmdir only an empty new directory. Restore the exact old directory only if
   the default path is absent and both identities remain verified. Unknown
   drift stops cleanup. Never recursive-delete, reset old journals, or mint a
   receipt. An interruption at either rename/staging boundary leaves vendor
   DHCP/app startup unchanged because no hook exists.
6. Root reviews a separate fresh 110-second additive trial after file staging
   succeeds. Reuse serial original-auth cleanup before fixed-media takeover,
   forced .11.140/10.99.0.2 proofs, independent origin exit/expiry and read-only
   postexpiry inspection. No network operation, keep, boot allowance, hook,
   primary change or deadline extension is authorized by this candidate.
