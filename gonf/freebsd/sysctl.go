package freebsd

import . "github.com/snonux/gonf/api"

// sysctlsPreservingIDs keeps the existing file and live-command identities.
// Sysctls uses new sysctl.<key> command names; this consumer adapter retains
// the established names until a deliberate resource-identity migration.
func sysctlsPreservingIDs(kv []string, liveName func(string) string, opts ...FileOption) {
	pairs, err := ParseKV(kv)
	if err != nil {
		Refuse("File", sysctlConf, err)
		return
	}
	for _, pair := range pairs {
		opts = append(opts, WithKeyedLine(pair[0]+"=", pair[0]+"="+pair[1]))
	}
	File(sysctlConf, opts...)
	for _, pair := range pairs {
		name, value := pair[0], pair[1]
		commandOpts := []CommandOption{
			OnlyIf("sh", List("-c", `[ "$(sysctl -n `+name+`)" != "`+value+`" ]`)),
		}
		if liveName != nil {
			commandOpts = append(commandOpts, WithName(liveName(name)))
		}
		Command("sysctl", List(name+"="+value), commandOpts...)
	}
}
