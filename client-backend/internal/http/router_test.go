package httpapi

import (
	"testing"

	"relay/client-backend/internal/services"
)

func TestRouterBuilds(t *testing.T) {
	if router := NewRouter(&services.Services{}); router == nil {
		t.Fatal("NewRouter returned nil")
	}
}
