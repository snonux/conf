package wireguard

import (
	. "github.com/snonux/gonf/api"
)

// groupOtherBits matches a file with any group or other permission bit. It
// is spelled as six "-perm -bit" alternatives because OpenBSD find(1) knows
// neither GNU's "-perm /077" nor BSD "-perm +077"; the form works with
// every find in the mesh.
var groupOtherBits = List("(",
	"-perm", "-040", "-o", "-perm", "-020", "-o", "-perm", "-010", "-o",
	"-perm", "-004", "-o", "-perm", "-002", "-o", "-perm", "-001", ")")

// groupOtherBitsSh is groupOtherBits quoted for sh -c.
const groupOtherBitsSh = `\( -perm -040 -o -perm -020 -o -perm -010 -o -perm -004 -o -perm -002 -o -perm -001 \)`

// StripGroupOther declares the sweep that removes every group/other bit
// from the regular files in dir except the public keys (*.pub). The config
// directories keep dated backups (wg0.conf.bak.YYYYMMDD-*) and hand-made
// key copies whose names differ per host and run, so they cannot be named
// as File resources; this covers them without naming, deleting or reading
// them. "chmod go=" leaves the owner bits (0644 -> 0600, 0400 stays).
//
// The command only runs while such a file exists, so a clean host shows it
// as skipped. Call it inside a WhenPathExists(dir) guard: without dir the
// guard's find fails and the command is skipped anyway.
func StripGroupOther(dir string) Resource {
	find := append(List(dir, "-type", "f", "!", "-name", "*.pub"), groupOtherBits...)
	return Command("find", append(find, "-exec", "chmod", "go=", "{}", "+"),
		OnlyIf("sh", List("-c", "find "+dir+" -type f ! -name '*.pub' "+groupOtherBitsSh+" | grep -q .")),
		WithName("wireguard-strip-group-other"))
}
