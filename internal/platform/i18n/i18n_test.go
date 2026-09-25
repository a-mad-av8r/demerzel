package i18n

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddlewareAlwaysUsesBritishEnglish(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	tests := []struct {
		name           string
		acceptLanguage string
	}{
		{name: "empty header"},
		{name: "British English", acceptLanguage: "en-GB"},
		{name: "wildcard", acceptLanguage: "*"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			context, _ := gin.CreateTestContext(nil)
			context.Request = httptest.NewRequest("GET", "/", nil)
			context.Request.Header.Set("Accept-Language", test.acceptLanguage)
			Middleware()(context)

			if got := GetLanguageFromContext(context); got != "en-GB" {
				t.Fatalf("GetLanguageFromContext() = %q, want %q", got, "en-GB")
			}
			if got := Message(context, "bad_request"); got != "Bad request" {
				t.Fatalf("Message() = %q, want %q", got, "Bad request")
			}
		})
	}
}

func TestResolveLanguageAlwaysReturnsBritishEnglish(t *testing.T) {
	for _, header := range []string{"", "*", "en-GB"} {
		if got := ResolveLanguage(header); got != "en-GB" {
			t.Errorf("ResolveLanguage(%q) = %q, want %q", header, got, "en-GB")
		}
	}
}

func TestBritishEnglishControlMessages(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	tests := []struct {
		id      string
		message string
	}{
		{id: "request_too_large", message: "Request body is too large"},
		{id: "group.channel_target_conflict", message: "An existing group already uses this channel target"},
		{id: "group.in_use", message: "The group is still referenced by access keys"},
	}
	for _, test := range tests {
		if got := T(GetLocalizer(""), test.id); got != test.message {
			t.Errorf("T(%q) = %q, want %q", test.id, got, test.message)
		}
	}
}

func TestMessageWithoutLocalizerUsesBritishEnglish(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(nil)

	if got := Message(context, "bad_request"); got != "Bad request" {
		t.Fatalf("Message() = %q, want %q", got, "Bad request")
	}
}
