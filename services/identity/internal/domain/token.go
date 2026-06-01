package domain

import "time"

type AccessToken struct {
	Value     string
	ExpiresAt time.Time
}

// Claims is the verified content of an access token, as returned by the
// TokenParser port. It is the domain view — string newtypes, no jwt package.
type Claims struct {
	UserID UserID
	Email  Email
	Roles  []Role
}

type TokenPair struct {
	Access  AccessToken
	Refresh string
}
