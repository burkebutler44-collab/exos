package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"relay/client-backend/internal/domain"
	"relay/client-backend/internal/services"
	"relay/client-backend/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type platformPermissionRepo struct {
	store.Repository
	allowed bool
	userID  uuid.UUID
}

func (r *platformPermissionRepo) UpsertUser(ctx context.Context, identity store.AuthIdentity) (domain.User, error) {
	if r.userID == uuid.Nil {
		r.userID = uuid.New()
	}
	return domain.User{
		ID:       r.userID,
		Auth0Sub: identity.Auth0Sub,
		Email:    identity.Email,
		Name:     identity.Name,
	}, nil
}

func (r *platformPermissionRepo) HasPlatformPermission(ctx context.Context, userID uuid.UUID, permission string) (bool, error) {
	return r.allowed && userID == r.userID && permission == domain.PlatformUsersView, nil
}

func TestRequirePlatformPermissionRequiresAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/test", RequirePlatformPermission(services.New(&platformPermissionRepo{}), domain.PlatformUsersView), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequirePlatformPermissionDeniesMissingPermission(t *testing.T) {
	rec := performPlatformPermissionRequest(t, false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequirePlatformPermissionAllowsExpectedPermission(t *testing.T) {
	rec := performPlatformPermissionRequest(t, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func performPlatformPermissionRequest(t *testing.T, allowed bool) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := &platformPermissionRepo{allowed: allowed, userID: uuid.New()}
	svc := services.New(repo)
	router := gin.New()
	router.Use(RequireAuth(svc))
	router.GET("/admin/test", RequirePlatformPermission(svc, domain.PlatformUsersView), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	req.Header.Set("X-Auth0-Sub", "auth0|admin")
	req.Header.Set("X-User-Email", "ops@example.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
