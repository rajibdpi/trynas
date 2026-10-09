package app

import (
	"strings"
	"testing"
)

func TestActivityIncludesIP(t *testing.T) {
	a, admin, _ := setup(t)
	a.Log(admin.ID, "test", "file.txt", "192.168.0.25")
	response := request(a, admin, "GET", "/api/activity", nil, a.Activity)
	expect(t, response, 200)
	if !strings.Contains(response.Body.String(), `"ip":"192.168.0.25"`) {
		t.Fatal("activity did not include IP")
	}
}
