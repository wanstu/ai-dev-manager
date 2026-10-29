//go:build !windows

package sandbox

func requiresExplicitToolGrant(string) bool {
	return true
}
