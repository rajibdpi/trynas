package core

// Home returns only an explicitly assigned, valid non-root storage path.
// Permissions must never supply a fallback home or widen this boundary.
func (s *State) Home(uid int) string {
	var home string
	if s.DB.QueryRow("SELECT h.path FROM user_homes h WHERE h.user_id=? AND NOT EXISTS(SELECT 1 FROM user_homes other WHERE other.path=h.path AND other.user_id<>h.user_id)", uid).Scan(&home) != nil {
		return ""
	}
	clean, err := CleanPath(home)
	if err != nil || clean == "" || clean != home {
		return ""
	}
	return home
}
