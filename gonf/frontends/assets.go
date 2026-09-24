package frontends

import "codeberg.org/snonux/conf/gonf/paths"

// This package reads controller-local source assets from two trees, and each
// has exactly one path-join implementation in the module, in package paths.
// The two wrappers below only pick which tree; they never join paths
// themselves, so renaming either tree needs an edit in paths alone.

// frontendAsset returns the path of an asset under this package's own
// gonf/frontends/assets directory (the rendered *.tmpl templates and the
// gonf-only configs). It wraps the shared paths.GonfAsset helper, which the
// openbsd, freebsd and netbsd packages use for their own assets directories.
func frontendAsset(name string) string {
	return paths.GonfAsset("frontends", name)
}

// legacyFrontendAsset returns the path of an asset under the repository's
// top-level frontends/ tree (etc/, scripts/, var/, ...), which predates the
// gonf recipes and is still shared with the Rexfile. It wraps
// paths.FrontendAsset, the same helper the rocky, openbsd, freebsd and netbsd
// packages use for the frontends/scripts and frontends/systemd assets.
func legacyFrontendAsset(name string) string {
	return paths.FrontendAsset(name)
}
