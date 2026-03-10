package lsp

import (
	"context"
	"sync"
	"time"
)

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

func (s *completionState) touchLocked(key string) {
	idx := -1
	for i, k := range s.compCacheOrder {
		if k == key {
			idx = i
			break
		}
	}
	if idx >= 0 {
		s.compCacheOrder = append(append([]string{}, s.compCacheOrder[:idx]...), s.compCacheOrder[idx+1:]...)
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

func (s *Server) storePendingCompletion(key string, items []CompletionItem) {
	s.completionState.storePendingCompletion(key, items)
}

func (s *Server) setCompletionsDisabled(disabled bool) bool {
	return s.completionState.setCompletionsDisabled(disabled)
}

func (s *Server) completionDisabled() bool {
	return s.completionState.completionDisabled()
}

func (s *Server) takePendingCompletion(key string) []CompletionItem {
	return s.completionState.takePendingCompletion(key)
}

func (s *Server) completionCacheGet(key string) (string, bool) {
	return s.completionState.cacheGet(key)
}

func (s *Server) completionCachePut(key, value string) {
	s.completionState.cachePut(key, value)
}

func (s *Server) waitForThrottle(ctx context.Context) bool {
	return s.completionState.waitForThrottle(ctx, s.completionThrottle())
}
