package share

import (
	"slices"
	"sort"
)

func (a *Module) existingShare(owner int, storage string, paths []string) (map[string]any, error) {
	rows, err := a.DB.Query(`SELECT s.id,s.path,s.token,COALESCE(strftime('%Y-%m-%dT%H:%M:%fZ',s.expires_at),''),EXISTS(SELECT 1 FROM share_passwords p WHERE p.share_id=s.id) FROM shares s WHERE s.user_id=? AND COALESCE((SELECT storage FROM share_storage WHERE share_id=s.id),'')=? AND (s.expires_at IS NULL OR julianday(s.expires_at)>julianday('now')) ORDER BY s.id DESC`, owner, storage)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		id                  int64
		path, token, expiry string
		protected           bool
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err = rows.Scan(&c.id, &c.path, &c.token, &c.expiry, &c.protected); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	wanted := slices.Clone(paths)
	sort.Strings(wanted)
	for _, c := range candidates {
		existing, err := a.SharePaths(c.id, c.path)
		if err != nil {
			return nil, err
		}
		sort.Strings(existing)
		if slices.Equal(wanted, existing) {
			return map[string]any{"id": c.id, "path": c.path, "paths": existing, "token": c.token, "expiresAt": c.expiry, "passwordProtected": c.protected, "storage": storage, "reused": true}, nil
		}
	}
	return nil, nil
}
