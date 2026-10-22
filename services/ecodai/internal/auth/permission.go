package auth

import "strings"

// Allows accepts an exact CDP Hub grant or its literal namespace grant.
func AllowsGrants(grants []string, required string) bool {
	if required == "" || strings.Contains(required, "*") {
		return false
	}
	for _, grant := range grants {
		if grant == required || grant == "cdphub.*" && strings.HasPrefix(required, "cdphub.") {
			return true
		}
	}
	return false
}
