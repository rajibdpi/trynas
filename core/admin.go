package core

func ValidAdminPassword(password string) bool {
	return len(password) >= 8 && len(password) <= 72 && password != "change-this-now" && password != "replace-with-a-strong-random-password"
}
