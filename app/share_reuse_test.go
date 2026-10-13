package app

import (
	"encoding/json"
	"testing"
)

func TestShareReusesActiveSelection(t *testing.T) {
	a, admin, user := setup(t)
	write(t, a, "folder/a.txt", "a")
	write(t, a, "folder/b.txt", "b")
	grant(t, a, user, "folder", "read")
	create := func(paths []string) (string, bool) {
		response := jsonRequest(a, user, "POST", "/api/shares", map[string]any{"paths": paths, "never": true}, a.CreateShare)
		expect(t, response, 200)
		var result struct {
			Token  string
			Reused bool
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Token, result.Reused
	}
	token, reused := create([]string{"folder/a.txt", "folder/b.txt"})
	if reused {
		t.Fatal("first share reused")
	}
	same, reused := create([]string{"folder/b.txt", "folder/a.txt"})
	if !reused || same != token {
		t.Fatal("same selection not reused")
	}
	file, reused := create([]string{"folder/a.txt"})
	if reused || file == token {
		t.Fatal("individual item reused collection")
	}
	folder, _ := create([]string{"folder"})
	same, reused = create([]string{"folder", "folder/a.txt"})
	if !reused || same != folder {
		t.Fatal("folder descendants not normalized")
	}
	other := jsonRequest(a, admin, "POST", "/api/shares", map[string]any{"path": "folder/a.txt", "never": true}, a.CreateShare)
	expect(t, other, 200)
	var otherResult struct{ Token string }
	_ = json.Unmarshal(other.Body.Bytes(), &otherResult)
	if otherResult.Token == file {
		t.Fatal("other owner reused link")
	}
	_, _ = a.DB.Exec("UPDATE shares SET expires_at=datetime('now','-1 day') WHERE token=?", file)
	fresh, reused := create([]string{"folder/a.txt"})
	if reused || fresh == file {
		t.Fatal("expired link reused")
	}
	var count int
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM shares WHERE user_id=?", user.ID).Scan(&count)
	if count != 4 {
		t.Fatalf("unexpected share count %d", count)
	}
}
