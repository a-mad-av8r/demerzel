package accountscli

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxControlResponseBytes = 8 << 20

type apiClient struct {
	baseURL string
	authKey string
	http    *http.Client
}

type group struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	ChannelID      string `json:"channel_id"`
	ChannelName    string `json:"channel_name"`
	ConnectionType string `json:"connection_type"`
}

type credentialItem struct {
	CredentialID            uint            `json:"credential_id"`
	Label                   string          `json:"label"`
	Mask                    string          `json:"mask"`
	Account                 accountIdentity `json:"account"`
	AuthState               string          `json:"auth_state"`
	AuthErrorCode           string          `json:"auth_error_code"`
	ConfiguredStatus        string          `json:"configured_status"`
	EffectiveStatus         string          `json:"effective_status"`
	RecentSuccessCount      uint64          `json:"recent_success_count"`
	RecentFailureCount      uint64          `json:"recent_failure_count"`
	ConsecutiveFailureCount uint64          `json:"consecutive_failure_count"`
	LastFailureCategory     string          `json:"last_failure_category"`
	LastStatusCode          *int            `json:"last_status_code"`
	CooldownUntilMS         *int64          `json:"cooldown_until_ms"`
	ModelCooldowns          []modelCooldown `json:"model_cooldowns"`
	Observation             *observation    `json:"observation"`
}

type accountIdentity struct {
	Email     string `json:"email"`
	EmailMask string `json:"email_mask"`
}

type modelCooldown struct {
	Model           string `json:"model"`
	CooldownUntilMS int64  `json:"cooldown_until_ms"`
}

type observation struct {
	State         string               `json:"state"`
	LastErrorCode string               `json:"last_error_code"`
	Snapshot      *observationSnapshot `json:"snapshot"`
}

type observationSnapshot struct {
	Account      *observationAccount `json:"account_summary"`
	QuotaWindows []quotaWindow       `json:"quota_windows"`
}

type observationAccount struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type quotaWindow struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Unit      string   `json:"unit"`
	Remaining *float64 `json:"remaining"`
	Limit     *float64 `json:"limit"`
	State     string   `json:"state"`
	ResetAtMS *int64   `json:"reset_at_ms"`
}

type credentialPage struct {
	Items      []credentialItem `json:"items"`
	Pagination struct {
		TotalPages int `json:"total_pages"`
	} `json:"pagination"`
}

func newAPIClient(baseURL, authKey string) *apiClient {
	return &apiClient{
		baseURL: baseURL,
		authKey: authKey,
		http: &http.Client{
			Timeout: 20 * time.Second,
			Transport: &http.Transport{
				Proxy: nil,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func validateControlURL(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed == nil || parsed.Host == "" {
		return "", errors.New("control API URL must be a loopback HTTP(S) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		parsed.Opaque != "" || parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("control API URL must be a loopback HTTP(S) origin")
	}
	host := strings.TrimSuffix(parsed.Hostname(), ".")
	if !strings.EqualFold(host, "localhost") {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return "", errors.New("management credentials may only be sent to a loopback control API")
		}
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (c *apiClient) listGroups() ([]group, error) {
	var result struct {
		Items []group `json:"items"`
	}
	if err := c.request(http.MethodGet, "/api/modern/groups", nil, "", &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (c *apiClient) listGroupCredentials(groupID uint) ([]credentialItem, error) {
	const pageSize = 100
	items := make([]credentialItem, 0)
	for page := 1; page <= 10000; page++ {
		query := url.Values{
			"page":      {strconv.Itoa(page)},
			"page_size": {strconv.Itoa(pageSize)},
		}
		var result credentialPage
		path := fmt.Sprintf("/api/modern/groups/%d/credentials?%s", groupID, query.Encode())
		if err := c.request(http.MethodGet, path, nil, "", &result); err != nil {
			return nil, err
		}
		items = append(items, result.Items...)
		if len(result.Items) == 0 || result.Pagination.TotalPages == 0 || page >= result.Pagination.TotalPages {
			return items, nil
		}
	}
	return nil, errors.New("control API returned too many credential pages")
}

func (c *apiClient) importCredential(groupID uint, secret, idempotencyKey string) (struct {
	CredentialsAdded      int `json:"credentials_added"`
	CredentialsDuplicated int `json:"credentials_duplicated"`
}, error) {
	var result struct {
		CredentialsAdded      int `json:"credentials_added"`
		CredentialsDuplicated int `json:"credentials_duplicated"`
	}
	path := fmt.Sprintf("/api/groups/%d/credentials/import", groupID)
	body := struct {
		Credentials string `json:"credentials"`
	}{Credentials: secret}
	if err := c.request(http.MethodPost, path, body, idempotencyKey, &result); err != nil {
		return result, err
	}
	return result, nil
}

func (c *apiClient) updateLabel(groupID, credentialID uint, label string) (credentialItem, error) {
	var result credentialItem
	path := fmt.Sprintf("/api/groups/%d/credentials/%d", groupID, credentialID)
	body := struct {
		Label string `json:"label"`
	}{Label: label}
	if err := c.request(http.MethodPut, path, body, "", &result); err != nil {
		return credentialItem{}, err
	}
	return result, nil
}

func (c *apiClient) setEnabled(groupID, credentialID uint, enabled bool) error {
	action := "disable"
	if enabled {
		action = "enable"
	}
	body := struct {
		Action        string `json:"action"`
		CredentialIDs []uint `json:"credential_ids"`
	}{Action: action, CredentialIDs: []uint{credentialID}}
	var result struct {
		AffectedCredentialIDs []uint `json:"affected_credential_ids"`
	}
	path := fmt.Sprintf("/api/groups/%d/credentials/batch", groupID)
	if err := c.request(http.MethodPost, path, body, "", &result); err != nil {
		return err
	}
	for _, affectedID := range result.AffectedCredentialIDs {
		if affectedID == credentialID {
			return nil
		}
	}
	return errors.New("control API did not confirm the credential status change")
}

func (c *apiClient) removeCredential(groupID, credentialID uint) error {
	path := fmt.Sprintf("/api/groups/%d/credentials/%d", groupID, credentialID)
	return c.request(http.MethodDelete, path, struct{}{}, "", nil)
}

func (c *apiClient) request(method, path string, body any, idempotencyKey string, result any) error {
	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return errors.New("could not encode management API request")
		}
		defer clear(encoded)
		requestBody = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(context.Background(), method, c.baseURL+path, requestBody)
	if err != nil {
		return errors.New("could not create management API request")
	}
	request.Header.Set("Authorization", "Bearer "+c.authKey)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("control API request failed")
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxControlResponseBytes+1))
	if err != nil || len(payload) > maxControlResponseBytes {
		return errors.New("control API returned an unreadable response")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("control API returned HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Code json.RawMessage `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil || !isSuccessCode(envelope.Code) {
		return errors.New("control API returned an unsuccessful response")
	}
	if result == nil {
		return nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return errors.New("control API response did not include result data")
	}
	if err := json.Unmarshal(envelope.Data, result); err != nil {
		return errors.New("control API returned an invalid result")
	}
	return nil
}

func isSuccessCode(code json.RawMessage) bool {
	return string(code) == `"0"` || string(code) == "0"
}

func newIdempotencyKey() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value[:])
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		encoded[:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:],
	), nil
}
