package core

// BuildAdminPasswordHash is populated by the release builder from .env.
// It initializes an empty database only when no runtime password is provided.
var BuildAdminPasswordHash string

// BuildAdminPasswordEncoded holds a recoverable copy for the first-start message.
var BuildAdminPasswordEncoded string

func ValidAdminPassword(password string) bool {
	return len(password) >= 8 && len(password) <= 72 && password != "change-this-now" && password != "replace-with-a-strong-random-password"
}
