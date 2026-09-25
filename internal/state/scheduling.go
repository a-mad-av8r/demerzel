package state

import (
	"cmp"
	"math/bits"
	"sync"
	"time"
)

// SchedulingProgress uses fixed 128-bit coordinates to avoid small floating-point increments failing over long runtimes.
// The whole part increases by at most one per allocation; even one million allocations a second requires over half a million years to exhaust it.
type SchedulingProgress struct {
	Whole    uint64 `json:"whole"`
	Fraction uint64 `json:"fraction"`
}

func (p SchedulingProgress) Compare(other SchedulingProgress) int {
	if order := cmp.Compare(p.Whole, other.Whole); order != 0 {
		return order
	}
	return cmp.Compare(p.Fraction, other.Fraction)
}

func (p SchedulingProgress) Advance(weight uint64) (SchedulingProgress, bool) {
	if weight == 0 {
		return p, false
	}
	var carry uint64
	if weight == 1 {
		carry = 1
	} else {
		step, _ := bits.Div64(1, 0, weight)
		p.Fraction, carry = bits.Add64(p.Fraction, step, 0)
	}
	p.Whole, carry = bits.Add64(p.Whole, carry, 0)
	return p, carry == 0
}

type SchedulingMember struct {
	ID                 uint
	GroupID            uint
	IdentityGeneration uint64
	Progress           SchedulingProgress
	LastSelected       uint64
	Pending            bool
	suspended          bool
	cooldownUntil      time.Time
}

func (m *SchedulingMember) Admit(baseline SchedulingProgress) {
	if m.Progress.Compare(baseline) < 0 {
		m.Progress = baseline
	}
	m.Pending, m.suspended = false, false
}

type SerialAccountState struct {
	IdentityGeneration     uint64
	ResetAt                time.Time
	CooldownUntil          time.Time
	NextProbeAt            time.Time
	ProbeFailures          int
	ProbeInFlight          bool
	ProbeVerified          bool
	ProbeUnsupported       bool
	SuppressedQuotaResetAt time.Time
}

type SerialGroupState struct {
	CursorCredentialID       uint
	CursorIdentityGeneration uint64
	Accounts                 map[uint]*SerialAccountState
}

// SchedulingLedger is accessed only within a SchedulingState.WithLock callback.
// Credential facts belong to Registry; this table independently retains allocation history and does not roll back with credential configuration replacement.
type SchedulingLedger struct {
	Members       map[uint]*SchedulingMember
	ModelCursors  map[GroupModelKey][]string
	SerialGroups  map[uint]*SerialGroupState
	Groups        map[uint]bool
	GroupRevision uint64
	GroupsKnown   bool
	Started       bool
	Watermark     SchedulingProgress
	Sequence      uint64
	LastMember    uint
	Consecutive   uint64
}

type SchedulingState struct {
	mu     sync.Mutex
	ledger SchedulingLedger
}

func NewSchedulingState() *SchedulingState {
	return &SchedulingState{ledger: SchedulingLedger{
		Members: make(map[uint]*SchedulingMember), Groups: make(map[uint]bool),
		ModelCursors: make(map[GroupModelKey][]string),
		SerialGroups: make(map[uint]*SerialGroupState),
	}}
}

func (s *SchedulingState) WithLock(fn func(*SchedulingLedger)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.ledger)
}

func (s *SchedulingState) syncCredentialLocked(view CredentialRuntimeView) {
	d := &s.ledger
	m := d.Members[view.ID]
	if m == nil || m.GroupID != view.GroupID || m.IdentityGeneration != view.IdentityGeneration {
		if groups := d.SerialGroups[view.GroupID]; groups != nil {
			delete(groups.Accounts, view.ID)
			if groups.CursorCredentialID == view.ID {
				groups.CursorCredentialID, groups.CursorIdentityGeneration = 0, 0
			}
		}
		// Startup publishes Groups before loading credentials; retain the recovery boundary for disabled Groups even before their first allocation.
		m = &SchedulingMember{ID: view.ID, GroupID: view.GroupID,
			IdentityGeneration: view.IdentityGeneration,
			Pending:            d.Started || d.GroupsKnown && !d.Groups[view.GroupID]}
		d.Members[view.ID] = m
		if d.LastMember == view.ID {
			d.LastMember, d.Consecutive = 0, 0
		}
	}
	authUnavailable := view.AuthState != CredentialAuthStateReady &&
		view.AuthState != "" && view.AuthState != CredentialAuthStateRefreshing
	suspended := view.Status != CredentialStatusActive || view.Blacklisted || authUnavailable ||
		view.WeightManual != nil && *view.WeightManual <= 0 || view.CooldownUntil.After(time.Now())
	// Record the cooldown event itself so the recovery boundary is not lost between expiry and the next selection without a management update.
	if suspended || m.suspended || !view.CooldownUntil.IsZero() && !view.CooldownUntil.Equal(m.cooldownUntil) {
		m.Pending = true
	}
	m.suspended, m.cooldownUntil = suspended, view.CooldownUntil
}

func (s *SchedulingState) SyncCredential(view CredentialRuntimeView) {
	s.WithLock(func(*SchedulingLedger) { s.syncCredentialLocked(view) })
}

// SyncCredentials aligns the member set; groupID=0 means a complete replacement, while other values reconcile only that Group.
func (s *SchedulingState) SyncCredentials(groupID uint, views []CredentialRuntimeView) {
	s.WithLock(func(d *SchedulingLedger) {
		seen := make(map[uint]struct{}, len(views))
		for _, view := range views {
			seen[view.ID] = struct{}{}
			s.syncCredentialLocked(view)
		}
		for id, member := range d.Members {
			if groupID != 0 && member.GroupID != groupID {
				continue
			}
			if _, exists := seen[id]; !exists {
				s.removeLocked(id)
			}
		}
	})
}

func (s *SchedulingState) Remove(id uint) {
	s.WithLock(func(*SchedulingLedger) { s.removeLocked(id) })
}

func (s *SchedulingState) removeLocked(id uint) {
	member := s.ledger.Members[id]
	if member != nil {
		if serial := s.ledger.SerialGroups[member.GroupID]; serial != nil {
			delete(serial.Accounts, id)
			if serial.CursorCredentialID == id {
				serial.CursorCredentialID, serial.CursorIdentityGeneration = 0, 0
			}
		}
	}
	delete(s.ledger.Members, id)
	if s.ledger.LastMember == id {
		s.ledger.LastMember, s.ledger.Consecutive = 0, 0
	}
}

// SyncGroups is called by the configuration-publication path to avoid missing calibration when a Group is disabled and re-enabled between requests.
func (s *SchedulingState) SyncGroups(snapshot *ConfigSnapshot) {
	if snapshot == nil {
		return
	}
	s.WithLock(func(d *SchedulingLedger) {
		if d.GroupsKnown && snapshot.Revision <= d.GroupRevision {
			return
		}
		groups := make(map[uint]bool, len(snapshot.GroupCatalog))
		for id, group := range snapshot.GroupCatalog {
			// Consistent with routing compilation: Groups without models are entirely paused, while normal candidate changes retain history.
			groups[id] = group.Enabled && len(snapshot.Groups[id].Models) > 0 &&
				(group.WeightManual == nil || *group.WeightManual > 0)
		}
		for _, member := range d.Members {
			if !groups[member.GroupID] {
				member.Pending = true
			}
		}
		d.Groups, d.GroupRevision, d.GroupsKnown = groups, snapshot.Revision, true
		for groupID, serial := range d.SerialGroups {
			if _, exists := snapshot.GroupCatalog[groupID]; !exists {
				delete(d.SerialGroups, groupID)
				continue
			}
			for _, account := range serial.Accounts {
				account.ProbeUnsupported = false
			}
		}
		s.syncModelCursorsLocked(snapshot)
	})
}

func (r *CredentialRegistry) SchedulingState() *SchedulingState { return r.scheduling }

func (r *CredentialRegistry) syncSchedulingGroupLocked(groupID uint) {
	views := make([]CredentialRuntimeView, 0, len(r.buckets[groupID]))
	for _, entry := range r.buckets[groupID] {
		views = append(views, runtimeView(entry))
	}
	r.scheduling.SyncCredentials(groupID, views)
}

// WithCredentialCandidates fixes identity and weight for this credential set; the callback permits in-memory scheduling only.
// Lock order is Registry read lock -> SchedulingState; never acquire the Registry lock in reverse order.
func (r *CredentialRegistry) WithCredentialCandidates(groupIDs []uint, excluded func(uint) bool,
	now time.Time, fn func([]CredentialMeta),
) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fn(r.collectCredentialCandidatesLocked(groupIDs, excluded, now))
}
