package paths

import "path"

// TriggerfishSecret returns the logical reference for private router input.
func TriggerfishSecret(relativePath string) string {
	return path.Join("triggerfish", relativePath)
}
