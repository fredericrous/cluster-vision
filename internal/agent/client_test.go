package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello world"}}]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", "test-model")
	got, err := client.Complete(context.Background(), "system", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello world" {
		t.Errorf("Complete() = %q, want %q", got, "hello world")
	}
}

func TestCompleteHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`internal server error`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", "test-model")
	_, err := client.Complete(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if want := "500"; !containsStr(err.Error(), want) {
		t.Errorf("error = %q, want to contain %q", err.Error(), want)
	}
}

func TestCompleteNoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", "test-model")
	_, err := client.Complete(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if want := "no choices"; !containsStr(err.Error(), want) {
		t.Errorf("error = %q, want to contain %q", err.Error(), want)
	}
}

func TestCompleteAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[],"error":{"message":"rate limited"}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", "test-model")
	_, err := client.Complete(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if want := "rate limited"; !containsStr(err.Error(), want) {
		t.Errorf("error = %q, want to contain %q", err.Error(), want)
	}
}

func TestCompleteJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"name\":\"test\",\"value\":42}"}}]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", "test-model")
	var dest struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	err := client.CompleteJSON(context.Background(), "system", "user", &dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dest.Name != "test" || dest.Value != 42 {
		t.Errorf("CompleteJSON() = {%q, %d}, want {\"test\", 42}", dest.Name, dest.Value)
	}
}

func TestCompleteJSONWithCodeFences(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// LLM wraps JSON in code fences — use escaped newlines in JSON string
		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "```json\n{\"name\":\"fenced\"}\n```"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", "test-model")
	var dest struct {
		Name string `json:"name"`
	}
	err := client.CompleteJSON(context.Background(), "system", "user", &dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dest.Name != "fenced" {
		t.Errorf("name = %q, want %q", dest.Name, "fenced")
	}
}

func TestStripCodeFences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"bare JSON object", `{"key":"value"}`, `{"key":"value"}`},
		{"json code fence", "```json\n{\"key\":\"value\"}\n```", `{"key":"value"}`},
		{"code fence no lang", "```\n{\"key\":\"value\"}\n```", `{"key":"value"}`},
		{"extra text before", "Here is the result:\n{\"key\":\"value\"}", `{"key":"value"}`},
		{"extra text after", "{\"key\":\"value\"}\nDone!", `{"key":"value"}`},
		{"nested braces", `{"a":{"b":"c"}}`, `{"a":{"b":"c"}}`},
		{"array input", `[{"a":1},{"b":2}]`, `[{"a":1},{"b":2}]`},
		{"array with fences", "```json\n[1,2,3]\n```", `[1,2,3]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripCodeFences(tt.input)
			if got != tt.want {
				t.Errorf("stripCodeFences(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
