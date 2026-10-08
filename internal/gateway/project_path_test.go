package gateway

import "ai-dev-manager-v2/internal/pathutil"

// Code-intelligence requests may use the Environment's canonical Windows long
// path even when the temporary test root was returned through a short 8.3 path.
func sameProjectRoot(actual any, root string) bool {
	path, ok := actual.(string)
	return ok && pathutil.Same(path, root)
}
