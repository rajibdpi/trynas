package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUploadOutlastsServerReadAndWriteDeadlines(t *testing.T) {
	server := httptest.NewUnstartedServer(unlimitedUpload(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		time.Sleep(250 * time.Millisecond)
		_, _ = w.Write(data)
	})))
	server.Config.ReadTimeout = 100 * time.Millisecond
	server.Config.WriteTimeout = 100 * time.Millisecond
	server.Start()
	defer server.Close()
	reader, writer := io.Pipe()
	defer reader.Close()
	go func() {
		defer writer.Close()
		_, _ = writer.Write([]byte("first"))
		time.Sleep(250 * time.Millisecond)
		_, _ = writer.Write([]byte("last"))
	}()
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Post(server.URL, "application/octet-stream", reader)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != 200 || string(data) != "firstlast" {
		t.Fatalf("upload was interrupted: status=%d body=%q err=%v", response.StatusCode, data, err)
	}
}
