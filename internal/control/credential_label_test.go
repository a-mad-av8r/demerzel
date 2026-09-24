package control

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"gpt-load/internal/platform/config"
	"gpt-load/internal/storage/models"
)

func TestCredentialLabelPersistsWithoutChangingSecretIdentity(t *testing.T) {
	initControlI18n(t)
	fixture, groupID, credentialID := newSubscriptionCredentialFixture(t)
	var before models.Credential
	if err := fixture.db.Take(&before, credentialID).Error; err != nil {
		t.Fatal(err)
	}
	server := NewServer(&config.Config{AuthKey: "label-test-auth"}, fixture.service)
	engine := gin.New()
	server.RegisterRoutes(engine)
	path := fmt.Sprintf("/api/groups/%d/credentials/%d", groupID, credentialID)

	update := serveCredentialRequest(t, engine, http.MethodPut, path,
		`{"label":"Personal account"}`, "label-test-auth", "00000000-0000-4000-8000-000000000501")
	if update.Code != http.StatusOK {
		t.Fatalf("label update = %d %s", update.Code, update.Body.String())
	}
	var result struct {
		Data struct {
			CredentialID uint   `json:"credential_id"`
			Label        string `json:"label"`
		} `json:"data"`
	}
	if err := json.Unmarshal(update.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.CredentialID != credentialID || result.Data.Label != "Personal account" {
		t.Fatalf("updated credential = %#v", result.Data)
	}

	setStatus := serveCredentialRequest(t, engine, http.MethodPut, path,
		`{"status":"disabled"}`, "label-test-auth", "00000000-0000-4000-8000-000000000502")
	if setStatus.Code != http.StatusOK {
		t.Fatalf("status update = %d %s", setStatus.Code, setStatus.Body.String())
	}
	get := serveCredentialRequest(t, engine, http.MethodGet, path, "", "label-test-auth", "")
	if get.Code != http.StatusOK || json.Unmarshal(get.Body.Bytes(), &result) != nil ||
		result.Data.Label != "Personal account" {
		t.Fatalf("persisted label missing after other update: %d %s", get.Code, get.Body.String())
	}

	invalid := serveCredentialRequest(t, engine, http.MethodPut, path,
		`{"label":null}`, "label-test-auth", "00000000-0000-4000-8000-000000000503")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("null label status = %d, want validation refusal", invalid.Code)
	}
	var after models.Credential
	if err := fixture.db.Take(&after, credentialID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Label != "Personal account" || after.Data != before.Data ||
		after.Fingerprint != before.Fingerprint || after.SecretVersion != before.SecretVersion {
		t.Fatal("label mutation changed encrypted credential identity")
	}
}
