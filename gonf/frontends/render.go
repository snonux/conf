package frontends

import (
	"fmt"

	. "github.com/snonux/gonf/api"
)

// renderControllerTemplate renders the native text/template asset at
// assetPath on the controller through api.RenderTemplate, the same engine
// web.go's renderHTTPD/renderRelayd use, so every frontend asset shares one
// template dialect: gonf's helper functions (join, lower, upper, trim,
// replace) and no conf-local extras. There is deliberately no "quote" func:
// a value that needs Go's %q quoting is pre-quoted with strconv.Quote by its
// data builder instead (renderNSDKey, renderNSDConfig,
// renderDNSPublisherConfig), so moving a stanza between assets never breaks
// on a function only one renderer defines.
//
// api.RenderTemplate JSON-encodes data once (a defensive snapshot; the whole
// value is also available as .Data), so the typed data structs here must stay
// JSON-compatible: exported string/bool/slice fields, no json tags and no
// methods a template calls. Templates therefore see the decoded maps, which
// is also what makes "missingkey=error" effective: a misspelled field name
// fails the render instead of rendering empty.
//
// Unlike WithTemplateData, which renders on the destination host from facts
// detected there at apply time, this always runs here while the recipe
// records: SMTPD/NSD candidate content must be identical regardless of which
// frontend later applies it, so the frontend Go renderers deliberately keep
// the controller-render distinction the consumer DSL review asked to preserve
// (see gonf's docs/design/consumer-dsl-simplification-plan.md, "External templates and
// consumer layout"). A missing or malformed asset is returned, wrapped with
// the asset path, for the caller to report as a declaration error (see
// renderBatch and refuseRender) instead of panicking.
func renderControllerTemplate(assetPath string, data any) (string, error) {
	rendered, err := RenderTemplate(assetPath, data)
	if err != nil {
		return "", fmt.Errorf("render frontend template %q: %w", assetPath, err)
	}
	return rendered, nil
}

// renderBatch collects the controller renders one recipe block needs before
// it declares anything, so a render failure can refuse the whole block (no
// resource is registered with empty content) without an if-err ladder per
// file. The first failure wins: later renders are skipped, and refused
// reports that failure once, as the declaration error of the live file path
// it was rendered for.
type renderBatch struct {
	path string
	err  error
}

// render runs fn for the live file at path unless an earlier render of the
// batch failed, and returns its content ("" after a failure, which the
// caller never declares because refused reports true).
func (b *renderBatch) render(path string, fn func() (string, error)) string {
	if b.err != nil {
		return ""
	}
	content, err := fn()
	if err != nil {
		b.path, b.err = path, err
		return ""
	}
	return content
}

// refused reports the batch's first render failure through refuseRender and
// returns true when there was one; the caller then declares none of the
// block's resources.
func (b *renderBatch) refused() bool {
	if b.err == nil {
		return false
	}
	refuseRender(b.path, b.err)
	return true
}
