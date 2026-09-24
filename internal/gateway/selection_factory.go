package gateway

import (
	"errors"
	"time"

	"gpt-load/internal/scheduler"
	"gpt-load/internal/state"
)

// SelectionFactory creates a selector for one request. It receives only the
// compiled snapshot and credential metadata; it must not access key material.
type SelectionFactory func(*state.ConfigSnapshot, scheduler.CredentialSource, scheduler.Query) scheduler.SelectionIterator

// ConfigureSelectionFactory replaces the built-in scheduler before the handler
// starts serving requests. Callers must configure it during assembly, not at runtime.
func (handler *Handler) ConfigureSelectionFactory(factory SelectionFactory) error {
	if handler == nil || factory == nil {
		return errors.New("selection factory is required")
	}
	handler.selectionFactory = factory
	return nil
}

func (handler *Handler) newSelectionIterator(snapshot *state.ConfigSnapshot, source scheduler.CredentialSource, query scheduler.Query) scheduler.SelectionIterator {
	if handler.selectionFactory == nil {
		return scheduler.New(snapshot, source, query)
	}
	iterator := handler.selectionFactory(snapshot, source, query)
	if iterator == nil {
		return exhaustedSelectionIterator{}
	}
	return iterator
}

type exhaustedSelectionIterator struct{}

func (exhaustedSelectionIterator) Next() (scheduler.Selection, error) {
	return scheduler.Selection{}, scheduler.ErrExhausted
}
func (exhaustedSelectionIterator) SkipGroup(uint) {}
func (exhaustedSelectionIterator) ChargeReplay(scheduler.Selection, state.CredentialRef) bool {
	return false
}
func (exhaustedSelectionIterator) StaticReason() scheduler.ReasonCode { return "" }
func (exhaustedSelectionIterator) CooldownUntil() (time.Time, bool)   { return time.Time{}, false }

var _ scheduler.SelectionIterator = exhaustedSelectionIterator{}
