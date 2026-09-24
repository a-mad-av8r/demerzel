package state

import (
	"slices"
	"sort"
	"time"
)

type SchedulingMemberCheckpoint struct {
	ID                 uint               `json:"id"`
	GroupID            uint               `json:"group_id"`
	IdentityGeneration uint64             `json:"identity_generation"`
	Progress           SchedulingProgress `json:"progress"`
	LastSelected       uint64             `json:"last_selected"`
	Pending            bool               `json:"pending,omitempty"`
}

type SchedulingCheckpoint struct {
	Version      int                          `json:"version"`
	Watermark    SchedulingProgress           `json:"watermark"`
	Sequence     uint64                       `json:"sequence"`
	LastMember   uint                         `json:"last_member"`
	Consecutive  uint64                       `json:"consecutive"`
	Members      []SchedulingMemberCheckpoint `json:"members"`
	ModelCursors []ModelCursorCheckpoint      `json:"model_cursors,omitempty"`
	SerialGroups []SerialGroupCheckpoint       `json:"serial_groups,omitempty"`
}

type ModelCursorCheckpoint struct {
	GroupID       uint     `json:"group_id"`
	ExternalModel string   `json:"external_model"`
	Order         []string `json:"model_order"`
}

type SerialGroupCheckpoint struct {
	GroupID                   uint                         `json:"group_id"`
	CursorCredentialID        uint                         `json:"cursor_credential_id,omitempty"`
	CursorIdentityGeneration  uint64                       `json:"cursor_identity_generation,omitempty"`
	Accounts                  []SerialAccountCheckpoint    `json:"accounts,omitempty"`
}

type SerialAccountCheckpoint struct {
	CredentialID             uint      `json:"credential_id"`
	IdentityGeneration       uint64    `json:"identity_generation"`
	ResetAt                  time.Time `json:"reset_at,omitempty"`
	CooldownUntil            time.Time `json:"cooldown_until,omitempty"`
	NextProbeAt              time.Time `json:"next_probe_at,omitempty"`
	ProbeFailures            int       `json:"probe_failures,omitempty"`
	ProbeUnsupported         bool      `json:"probe_unsupported,omitempty"`
	SuppressedQuotaResetAt   time.Time `json:"suppressed_quota_reset_at,omitempty"`
}

func (s *SchedulingState) CaptureCheckpoint() SchedulingCheckpoint {
	checkpoint := SchedulingCheckpoint{Version: 1}
	s.WithLock(func(d *SchedulingLedger) {
		checkpoint.Watermark, checkpoint.Sequence = d.Watermark, d.Sequence
		checkpoint.LastMember, checkpoint.Consecutive = d.LastMember, d.Consecutive
		for key, order := range d.ModelCursors {
			if len(order) > 0 {
				checkpoint.ModelCursors = append(checkpoint.ModelCursors, ModelCursorCheckpoint{
					GroupID: key.GroupID, ExternalModel: key.ExternalModel, Order: slices.Clone(order),
				})
			}
		}
		checkpoint.Members = make([]SchedulingMemberCheckpoint, 0, len(d.Members))
		for groupID, serial := range d.SerialGroups {
			saved := SerialGroupCheckpoint{
				GroupID: groupID, CursorCredentialID: serial.CursorCredentialID,
				CursorIdentityGeneration: serial.CursorIdentityGeneration,
			}
			for credentialID, account := range serial.Accounts {
				saved.Accounts = append(saved.Accounts, SerialAccountCheckpoint{
					CredentialID: credentialID, IdentityGeneration: account.IdentityGeneration,
					ResetAt: account.ResetAt, CooldownUntil: account.CooldownUntil,
					NextProbeAt: account.NextProbeAt, ProbeFailures: account.ProbeFailures,
					ProbeUnsupported: account.ProbeUnsupported,
					SuppressedQuotaResetAt: account.SuppressedQuotaResetAt,
				})
			}
			sort.Slice(saved.Accounts, func(i, j int) bool {
				return saved.Accounts[i].CredentialID < saved.Accounts[j].CredentialID
			})
			checkpoint.SerialGroups = append(checkpoint.SerialGroups, saved)
		}
		for _, m := range d.Members {
			checkpoint.Members = append(checkpoint.Members, SchedulingMemberCheckpoint{
				ID: m.ID, GroupID: m.GroupID, IdentityGeneration: m.IdentityGeneration,
				Progress: m.Progress, LastSelected: m.LastSelected, Pending: m.Pending,
			})
		}
	})
	sort.Slice(checkpoint.Members, func(i, j int) bool { return checkpoint.Members[i].ID < checkpoint.Members[j].ID })
	sort.Slice(checkpoint.ModelCursors, func(i, j int) bool {
		a, b := checkpoint.ModelCursors[i], checkpoint.ModelCursors[j]
		if a.GroupID != b.GroupID {
			return a.GroupID < b.GroupID
		}
		return a.ExternalModel < b.ExternalModel
	})
	sort.Slice(checkpoint.SerialGroups, func(i, j int) bool {
		return checkpoint.SerialGroups[i].GroupID < checkpoint.SerialGroups[j].GroupID
	})
	return checkpoint
}

// RestoreCheckpoint 在加载最新配置后调用；失配成员按新成员入场，不能单独归零追补旧历史。
func (s *SchedulingState) RestoreCheckpoint(checkpoint SchedulingCheckpoint) int {
	if checkpoint.Version != 1 || checkpoint.Sequence == ^uint64(0) || checkpoint.Watermark.Whole == ^uint64(0) {
		return 0
	}
	restored := 0
	s.WithLock(func(d *SchedulingLedger) {
		d.SerialGroups = make(map[uint]*SerialGroupState)
		for _, saved := range checkpoint.ModelCursors {
			key := GroupModelKey{GroupID: saved.GroupID, ExternalModel: saved.ExternalModel}
			if configured, exists := d.ModelCursors[key]; exists {
				d.ModelCursors[key] = reconcileModelOrder(saved.Order, configured)
			}
		}
		d.Started = checkpoint.Sequence != 0
		d.Watermark, d.Sequence = checkpoint.Watermark, checkpoint.Sequence
		d.LastMember, d.Consecutive = 0, 0
		for _, member := range d.Members {
			member.Pending = true
		}
		seen := make(map[uint]struct{}, len(checkpoint.Members))
		for _, saved := range checkpoint.Members {
			member := d.Members[saved.ID]
			if member == nil || member.GroupID != saved.GroupID || member.IdentityGeneration != saved.IdentityGeneration ||
				saved.LastSelected > checkpoint.Sequence || saved.Progress.Compare(checkpoint.Watermark) > 0 {
				continue
			}
			if _, duplicate := seen[saved.ID]; duplicate {
				continue
			}
			seen[saved.ID] = struct{}{}
			member.Progress, member.LastSelected = saved.Progress, saved.LastSelected
			member.Pending = saved.Pending || member.suspended || d.GroupsKnown && !d.Groups[member.GroupID]
			if saved.ID == checkpoint.LastMember && checkpoint.Consecutive <= 99+MaxWeight*MaxWeight {
				d.LastMember, d.Consecutive = saved.ID, checkpoint.Consecutive
			}
			restored++
		}
		seenSerialGroups := make(map[uint]struct{}, len(checkpoint.SerialGroups))
		for _, saved := range checkpoint.SerialGroups {
			if saved.GroupID == 0 {
				continue
			}
			if _, duplicate := seenSerialGroups[saved.GroupID]; duplicate {
				continue
			}
			seenSerialGroups[saved.GroupID] = struct{}{}
			serial := &SerialGroupState{Accounts: make(map[uint]*SerialAccountState)}
			for _, account := range saved.Accounts {
				member := d.Members[account.CredentialID]
				if account.CredentialID == 0 || account.ProbeFailures < 0 || member == nil ||
					member.GroupID != saved.GroupID || member.IdentityGeneration != account.IdentityGeneration {
					continue
				}
				failures := account.ProbeFailures
				if failures > 32 {
					failures = 32
				}
				if _, duplicate := serial.Accounts[account.CredentialID]; duplicate {
					continue
				}
				serial.Accounts[account.CredentialID] = &SerialAccountState{
					IdentityGeneration: account.IdentityGeneration,
					ResetAt: account.ResetAt, CooldownUntil: account.CooldownUntil,
					NextProbeAt: account.NextProbeAt, ProbeFailures: failures,
					ProbeUnsupported: account.ProbeUnsupported,
					SuppressedQuotaResetAt: account.SuppressedQuotaResetAt,
				}
			}
			cursor := d.Members[saved.CursorCredentialID]
			if saved.CursorCredentialID != 0 && cursor != nil &&
				cursor.GroupID == saved.GroupID &&
				cursor.IdentityGeneration == saved.CursorIdentityGeneration {
				serial.CursorCredentialID = saved.CursorCredentialID
				serial.CursorIdentityGeneration = saved.CursorIdentityGeneration
			}
			d.SerialGroups[saved.GroupID] = serial
		}
	})
	return restored
}
