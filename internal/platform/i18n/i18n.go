package i18n

import (
	"gpt-load/internal/platform/i18n/locales"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

const defaultLanguage = "en-GB"

var bundle *i18n.Bundle

// Init loads the sole supported message catalogue.
func Init() error {
	tag := language.MustParse(defaultLanguage)
	bundle = i18n.NewBundle(tag)
	for id, message := range locales.MessagesEnGB {
		bundle.AddMessages(tag, &i18n.Message{
			ID:    id,
			Other: message,
		})
	}
	return nil
}

// GetLocalizer returns the British English localiser. Accept-Language is
// intentionally ignored because the application ships one language.
func GetLocalizer(_ string) *i18n.Localizer {
	return newLocalizer()
}

func newLocalizer() *i18n.Localizer {
	return i18n.NewLocalizer(bundle, defaultLanguage)
}

// ResolveLanguage always returns the sole supported language.
func ResolveLanguage(_ string) string {
	return defaultLanguage
}

// T translates a message, returning its identifier if it is unavailable.
func T(localizer *i18n.Localizer, msgID string, data ...map[string]any) string {
	config := &i18n.LocalizeConfig{
		MessageID: msgID,
	}
	if len(data) > 0 {
		config.TemplateData = data[0]
	}
	msg, err := localizer.Localize(config)
	if err != nil {
		return msgID
	}
	return msg
}
