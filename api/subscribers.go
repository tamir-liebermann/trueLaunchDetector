package api

import "sync"

type Subscriber struct {
	Phone    string
	Location string // city name (Hebrew or English) for alert prioritization
	Language string // "he" or "en"
}

type SubscriberStore struct {
	mu   sync.RWMutex
	subs map[string]*Subscriber
}

func NewSubscriberStore() *SubscriberStore {
	return &SubscriberStore{subs: make(map[string]*Subscriber)}
}

func (s *SubscriberStore) Add(phone string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.subs[phone]; !exists {
		s.subs[phone] = &Subscriber{Phone: phone, Language: "he"}
	}
}

func (s *SubscriberStore) Remove(phone string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, phone)
}

func (s *SubscriberStore) SetLocation(phone, location string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sub, ok := s.subs[phone]; ok {
		sub.Location = location
	}
}

func (s *SubscriberStore) SetLanguage(phone, language string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sub, ok := s.subs[phone]; ok {
		sub.Language = language
	}
}

func (s *SubscriberStore) Get(phone string) (*Subscriber, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.subs[phone]
	return sub, ok
}

func (s *SubscriberStore) IsSubscribed(phone string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.subs[phone]
	return ok
}

func (s *SubscriberStore) All() []*Subscriber {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Subscriber, 0, len(s.subs))
	for _, sub := range s.subs {
		result = append(result, sub)
	}
	return result
}
