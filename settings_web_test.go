package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsRulesGETReturnsEmptyArrayForMissingConfigFile(t *testing.T) {
	store := NewRuleStore(filepath.Join(t.TempDir(), "keyword_replies.json"))
	if err := store.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	service := newSettingsWebService(func() *RuleStore { return store }, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/rules?token=test-token", nil)
	recorder := httptest.NewRecorder()

	service.routes(store, "test-token").ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != "[]" {
		t.Fatalf("body = %q, want []", body)
	}
}
