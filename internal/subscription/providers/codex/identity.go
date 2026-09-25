package codex

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	subscriptionruntime "gpt-load/internal/subscription/runtime"
)

func (*codexDriver) MatchesRefreshIdentity(current, refreshed subscriptionruntime.Credential) bool {
	before, err := ParseCredentialJSON(current.Canonical())
	if err != nil {
		return false
	}
	after, err := ParseCredentialJSON(refreshed.Canonical())
	if err != nil || before.AccountID != after.AccountID {
		return false
	}
	beforeUser, afterUser := credentialUserID(before), credentialUserID(after)
	// An old credential may complete identity, but a confirmed user cannot be downgraded to unknown.
	return beforeUser == "" || beforeUser == afterUser
}

// Derive identity only from existing tokens without changing canonical JSON used to calculate the persisted-content fingerprint.
func credentialIdentity(value Credential) string {
	accountID := strings.TrimSpace(value.AccountID)
	if userID := credentialUserID(value); userID != "" {
		return accountID + "/" + userID
	}
	return accountID
}

func credentialUserID(value Credential) string {
	for _, token := range []string{value.IDToken, value.AccessToken} {
		if userID := tokenUserID(token); userID != "" {
			return userID
		}
	}
	return ""
}

// An old ID token must not obscure the user belonging to the new access token actually used for requests.
func validateCredentialIdentity(value Credential) error {
	idUser, accessUser := tokenUserID(value.IDToken), tokenUserID(value.AccessToken)
	if idUser != "" && accessUser != "" && idUser != accessUser {
		return ErrCredentialIdentityChanged
	}
	return nil
}

// JWT is used only to read identity metadata from a held credential, never for signature or login authentication.
func tokenUserID(token string) string {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	defer clear(payload)
	var claims struct {
		Auth struct {
			ChatGPTUserID string `json:"chatgpt_user_id"`
			UserID        string `json:"user_id"`
		} `json:"https://api.openai.com/auth"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	if userID := strings.TrimSpace(claims.Auth.ChatGPTUserID); userID != "" {
		return userID
	}
	return strings.TrimSpace(claims.Auth.UserID)
}
