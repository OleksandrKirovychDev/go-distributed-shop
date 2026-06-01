package graphql

import (
	"net/http"
	"strings"
)

// Header contract with the router: after edge JWT validation it injects the
// verified principal. Same keys the gRPC server reads from metadata.
const (
	headerUserID    = "x-user-id"
	headerUserRoles = "x-user-roles"
)

// AuthMiddleware lifts the principal the router forwards into the request
// context, where resolvers read it. It never rejects: a missing x-user-id is an
// anonymous caller, which keeps registerUser/loginUser public while letting `me`
// enforce authentication itself.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get(headerUserID)
		if userID == "" {
			next.ServeHTTP(w, r)
			return
		}

		c := caller{userID: userID, roles: splitRoles(r.Header.Get(headerUserRoles))}
		next.ServeHTTP(w, r.WithContext(withCaller(r.Context(), c)))
	})
}

func splitRoles(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	roles := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			roles = append(roles, p)
		}
	}
	return roles
}
