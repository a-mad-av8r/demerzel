package control

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"gpt-load/internal/platform/config"
)

func TestDonorReleaseCheckEndpointIsNotExposed(t *testing.T) {
	initControlI18n(t)
	fixture := newServiceFixture(t)
	server := NewServer(&config.Config{AuthKey: "route-test-auth"}, fixture.service)
	engine := gin.New()
	server.RegisterRoutes(engine)

	request := httptest.NewRequest(http.MethodGet, "/api/system/update", nil)
	request.Header.Set("Authorization", "Bearer route-test-auth")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("GET /api/system/update status = %d, want removed route", response.Code)
	}
}
