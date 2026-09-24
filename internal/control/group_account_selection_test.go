package control

import (
	"encoding/json"
	"testing"

	"gpt-load/internal/channel"
	"gpt-load/internal/platform/config"
	"gpt-load/internal/state"
	"gpt-load/internal/storage/models"
)

func TestGroupAccountSelectionPersistsAndRoundTrips(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.CreateGroup(t.Context(), GroupCreateRequest{
		Name: stringPointer("new serial group"), ChannelID: channel.OpenAI,
		ConnectionType: models.ConnectionTypeAPIKey, Params: json.RawMessage(`{}`),
		Models: optionalGroupModels{Set: true, Values: []GroupModel{{ID: "gpt-4o"}}},
		Credentials: "sk-serial-default",
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
	var group models.Group
	if err := fixture.db.Take(&group, created.GroupID).Error; err != nil {
		t.Fatal(err)
	}
	if string(group.Overrides) != `{"account_selection":"serial"}` {
		t.Fatalf("new group overrides = %s", group.Overrides)
	}
	settings, err := fixture.service.GetGroupSettings(t.Context(), created.GroupID)
	if err != nil || settings.Effective.AccountSelection != state.AccountSelectionSerial ||
		settings.Effective.SerialQuotaReservePercent != 10 || settings.Overrides[state.SettingAccountSelection] != "serial" {
		t.Fatalf("new group settings = %#v, %v", settings, err)
	}

	custom := config.Settings{
		state.SettingAccountSelection:          string(state.AccountSelectionWeightedFair),
		state.SettingSerialQuotaReservePercent: 35,
	}
	updated, err := fixture.service.UpdateGroupSettings(t.Context(), created.GroupID, GroupSettingsUpdateRequest{
		Overrides: optionalField[config.Settings]{Set: true, Value: custom},
	})
	if err != nil {
		t.Fatalf("UpdateGroupSettings() error = %v", err)
	}
	if updated.Effective.AccountSelection != state.AccountSelectionWeightedFair ||
		updated.Effective.SerialQuotaReservePercent != 35 ||
		updated.Overrides[state.SettingSerialQuotaReservePercent] != json.Number("35") {
		t.Fatalf("updated group settings = %#v", updated)
	}
	if err := fixture.db.Take(&group, created.GroupID).Error; err != nil {
		t.Fatal(err)
	}
	if string(group.Overrides) != `{"account_selection":"weighted_fair","serial_quota_reserve_percent":35}` {
		t.Fatalf("persisted group overrides = %s", group.Overrides)
	}

	if _, err := fixture.service.UpdateGroupSettings(t.Context(), created.GroupID, GroupSettingsUpdateRequest{
		Overrides: optionalField[config.Settings]{Set: true, Value: config.Settings{
			state.SettingAccountSelection: "weighted_mix",
		}},
	}); err == nil {
		t.Fatal("UpdateGroupSettings() accepted a model-route strategy as account-selection mode")
	}
}

func TestLegacyGroupAccountSelectionDefaultsToWeightedFair(t *testing.T) {
	fixture := newServiceFixture(t)
	created, err := fixture.service.CreateGroup(t.Context(), GroupCreateRequest{
		Name: stringPointer("legacy group"), ChannelID: channel.OpenAI,
		ConnectionType: models.ConnectionTypeAPIKey, Params: json.RawMessage(`{}`),
		Models: optionalGroupModels{Set: true, Values: []GroupModel{{ID: "gpt-4o"}}},
		Credentials: "sk-legacy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Model(&models.Group{}).Where("id = ?", created.GroupID).
		Update("overrides", models.JSON(`{}`)).Error; err != nil {
		t.Fatal(err)
	}
	settings, err := fixture.service.GetGroupSettings(t.Context(), created.GroupID)
	if err != nil || settings.Effective.AccountSelection != state.AccountSelectionWeightedFair {
		t.Fatalf("legacy group settings = %#v, %v", settings, err)
	}
}
