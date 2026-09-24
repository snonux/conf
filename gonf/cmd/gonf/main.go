package main

import (
	"fmt"
	"os"

	"codeberg.org/snonux/conf/gonf/cluster"
	"codeberg.org/snonux/conf/gonf/paths"
	"codeberg.org/snonux/conf/gonf/tasks"
	"github.com/snonux/gonf/api"
	"github.com/snonux/gonf/cli"
	"github.com/snonux/gonf/secret"
	"github.com/snonux/gonf/secret/foostore"
)

func main() {
	if err := setSecretProvider(); err != nil {
		fmt.Fprintln(os.Stderr, "gonf: secret provider:", err)
		os.Exit(1)
	}
	cluster.Register()
	tasks.Register()
	os.Exit(cli.CLI())
}

// setSecretProvider wires the staged foostore/KeePass cutover (task ze2).
// References listed in the table resolve from the foostore vault (unlocked
// through foostore's own kdbx_pass_file; no passphrase passes through gonf).
// Every other reference is foostore's ErrNotFound, so secret.NewFallback
// reads it from gonf/secrets/ through secret.FileProvider exactly as before.
// A mapped reference whose vault read fails for any other reason (locked,
// corrupt, foostore missing) fails the plan loudly and never falls back to a
// possibly stale file copy. The vault is only read when a task resolves a
// secret, so tasks without secrets and -list never run foostore.
//
// The per-host goprecords tokens (etc/goprecords/<host>.token) are left
// unmapped on purpose: they are currently unset, so OptionalSecret skips
// them through the file fallback; map them here once real tokens exist in
// the vault.
func setSecretProvider() error {
	items, err := foostore.Items(map[secret.Ref]foostore.Item{
		secret.Ref(paths.FrontendSecret("var/nsd/etc/nsd_key.txt")): foostore.Field("Infra/nsd-tsig-key", "Password"),
		secret.Ref(paths.GarageSecret("rpc_secret")):                foostore.Field("Infra/garage-rpc", "Password"),
	})
	if err != nil {
		return err
	}
	vault, err := foostore.New(foostore.Config{Lookup: items})
	if err != nil {
		return err
	}
	// SetSecretProvider reports misuse as a declaration error, which the
	// CLI refuses to run with, so it needs no error check here.
	api.SetSecretProvider(secret.NewSnapshot(secret.NewFallback(vault, secret.FileProvider{})))
	return nil
}
