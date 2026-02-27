package eam

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"hello": "world"}

	writeJSON(w, data)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var got map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["hello"] != "world" {
		t.Errorf("body[hello] = %q, want %q", got["hello"], "world")
	}
}

func TestJsonErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"simple message", errors.New("not found"), `{"error":"not found"}`},
		{"with quotes", errors.New(`bad "input"`), `{"error":"bad \"input\""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonErr(tt.err)
			if got != tt.want {
				t.Errorf("jsonErr(%v) = %s, want %s", tt.err, got, tt.want)
			}
		})
	}
}
