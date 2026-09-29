package goprecords

import "github.com/snonux/conf/gonf/paths"

// ClientHost is a host's goprecords identity for EachHost: Host is the
// GOPRECORDS_HOST uploads are filed under and the name its token was issued
// for (--create-client-key).
type ClientHost struct {
	Host string
}

// PiToken is the logical secret reference of a Pi's upload token
// (secrets/pis/goprecords/<host>.token via FileProvider).
func PiToken(host string) string {
	return paths.PiSecret("goprecords/" + host + ".token")
}
