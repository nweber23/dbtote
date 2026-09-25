package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotify_PostsJSONTextField(t *testing.T) {
	var received map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := New(server.URL)
	if err := n.Notify(context.Background(), "backup completed"); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if received["text"] != "backup completed" {
		t.Errorf("got %q, want %q", received["text"], "backup completed")
	}
}

func TestNotify_ErrorsOnNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	n := New(server.URL)
	if err := n.Notify(context.Background(), "x"); err == nil {
		t.Error("expected an error for a 500 response")
	}
}
