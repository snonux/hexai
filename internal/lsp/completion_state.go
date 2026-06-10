package lsp

import (
	"context"
	"sync"
	"time"
)

// completionState manages the LRU completion cache, pending completions, and
// throttle timing. Its stateMu is independent of Server.mu — the two locks
// are never held simultaneously, so there is no ordering constraint.
type completionState struct {
	stateMu             sync.RWMutex
	compCache           map[string]string
	compCacheOrder      []string
	pendingCompletions  map[string][]CompletionItem
	lastLLMCall         time.Time
	completionsDisabled bool
}

func newCompletionState() completionState {
	return completionState{
		compCache:          make(map[string]string),
		pendingCompletions: make(map[string][]CompletionItem),
	}
}

func (s *completionState) storePendingCompletion(key string, items []CompletionItem) {
	if len(items) == 0 {
		return
	}
	cpy := make([]CompletionItem, len(items))
	copy(cpy, items)
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.pendingCompletions == nil {
		s.pendingCompletions = make(map[string][]CompletionItem)
	}
	s.pendingCompletions[key] = cpy
}

func (s *completionState) setCompletionsDisabled(disabled bool) bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	prev := s.completionsDisabled
	s.completionsDisabled = disabled
	return prev
}

func (s *completionState) completionDisabled() bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.completionsDisabled
}

func (s *completionState) takePendingCompletion(key string) []CompletionItem {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if len(s.pendingCompletions) == 0 {
		return nil
	}
	items, ok := s.pendingCompletions[key]
	if !ok {
		return nil
	}
	delete(s.pendingCompletions, key)
	cpy := make([]CompletionItem, len(items))
	copy(cpy, items)
	return cpy
}

// cacheGet returns the cached value for key. Uses a single write lock to
// avoid a TOCTOU race between the lookup and the LRU touch — the key could
// be evicted between an RUnlock and a subsequent Lock promotion.
func (s *completionState) cacheGet(key string) (string, bool) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	v, ok := s.compCache[key]
	if !ok {
		return "", false
	}
	s.touchLocked(key)
	return v, true
}

func (s *completionState) cachePut(key, value string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.compCache == nil {
		s.compCache = make(map[string]string)
	}
	if _, exists := s.compCache[key]; !exists {
		s.compCacheOrder = append(s.compCacheOrder, key)
		s.compCache[key] = value
		if len(s.compCacheOrder) > 10 {
			old := s.compCacheOrder[0]
			s.compCacheOrder = s.compCacheOrder[1:]
			delete(s.compCache, old)
		}
		return
	}
	s.compCache[key] = value
	s.touchLocked(key)
}

// touchLocked moves key to the end of the LRU order list.
// Uses delete-and-append: remove the existing entry in-place, then append.
func (s *completionState) touchLocked(key string) {
	for i, k := range s.compCacheOrder {
		if k == key {
			s.compCacheOrder = append(s.compCacheOrder[:i], s.compCacheOrder[i+1:]...)
			break
		}
	}
	s.compCacheOrder = append(s.compCacheOrder, key)
}

func (s *completionState) waitForThrottle(ctx context.Context, interval time.Duration) bool {
	if interval <= 0 {
		return true
	}
	var wait time.Duration
	for {
		s.stateMu.Lock()
		next := s.lastLLMCall.Add(interval)
		now := time.Now()
		if now.Before(next) {
			wait = next.Sub(now)
			s.stateMu.Unlock()
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false
			case <-timer.C:
				continue
			}
		}
		s.lastLLMCall = now
		s.stateMu.Unlock()
		return true
	}
}
