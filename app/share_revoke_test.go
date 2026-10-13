package app

import (
	"fmt"
	chi "github.com/go-chi/chi/v5"
	"net/http"
	"testing"
)

func TestRevokeShareDeletesRelatedRows(t *testing.T) {
	a, admin, user := setup(t)
	var ids []int64
	for i := 0; i < 2; i++ {
		result, err := a.DB.Exec("INSERT INTO shares(user_id,path,token) VALUES(?,?,?)", user.ID, "file.txt", fmt.Sprintf("revoke-token-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		id, _ := result.LastInsertId()
		ids = append(ids, id)
		for _, statement := range []string{
			"INSERT INTO share_sessions(share_id,token,expires_at) VALUES(?,hex(randomblob(16)),datetime('now','+1 day'))",
			"INSERT INTO share_passwords(share_id,password_hash) VALUES(?,'hash')",
			"INSERT INTO share_stats(share_id,visits,downloads) VALUES(?,2,3)",
			"INSERT INTO share_items(share_id,path) VALUES(?,'file.txt')",
			"INSERT INTO share_storage(share_id,storage) VALUES(?,'connection')",
		} {
			if _, err := a.DB.Exec(statement, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	router := chi.NewRouter()
	router.Delete("/api/shares/{id}", a.DeleteShare)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { router.ServeHTTP(w, r) })
	url := fmt.Sprintf("/api/shares/%d", ids[0])
	expect(t, request(a, admin, "DELETE", url, nil, handler), 404)
	expect(t, request(a, user, "DELETE", url, nil, handler), 200)
	for _, table := range []string{"shares", "share_sessions", "share_passwords", "share_stats", "share_items", "share_storage"} {
		column := "share_id"
		if table == "shares" {
			column = "id"
		}
		for i, id := range ids {
			var count int
			if err := a.DB.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE "+column+"=?", id).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != i {
				t.Fatalf("%s: share %d has %d related rows", table, id, count)
			}
		}
	}
}
