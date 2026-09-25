package i18n

import (
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const (
	// LocalizerKey stores the localiser in the Gin context.
	LocalizerKey = "localizer"
	// LanguageKey stores the canonical language code in the Gin context.
	LanguageKey = "language"
)

// AttachRequestLanguage stores the British English localiser on the request.
func AttachRequestLanguage(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(LocalizerKey, newLocalizer())
	c.Set(LanguageKey, defaultLanguage)
}

// Middleware adds the British English localiser to each request.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		AttachRequestLanguage(c)
		c.Next()
	}
}

// GetLanguageFromContext returns the language code used for the response.
func GetLanguageFromContext(c *gin.Context) string {
	if value, exists := c.Get(LanguageKey); exists {
		if language, ok := value.(string); ok && language != "" {
			return language
		}
	}
	return ResolveLanguage(c.GetHeader("Accept-Language"))
}

// GetLocalizerFromContext returns the localiser stored on the request.
func GetLocalizerFromContext(c *gin.Context) *i18n.Localizer {
	if localizer, exists := c.Get(LocalizerKey); exists {
		if typed, ok := localizer.(*i18n.Localizer); ok {
			return typed
		}
	}
	return GetLocalizer("")
}

// Message translates a request message using the request localiser.
func Message(c *gin.Context, msgID string, templateData ...map[string]any) string {
	localizer := GetLocalizerFromContext(c)
	return T(localizer, msgID, templateData...)
}
