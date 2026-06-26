package storage

import (
	"context"
	"sync"
)

type MemoryStore struct {
	mu        sync.Mutex
	jobs      map[string]LocalJob
	processed map[string]ProcessedMessage
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs:      map[string]LocalJob{},
		processed: map[string]ProcessedMessage{},
	}
}

func (s *MemoryStore) CreateOrGetJob(ctx context.Context, job LocalJob) (LocalJob, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.jobs[job.CentralJobID]; ok {
		return existing, false, nil
	}
	s.jobs[job.CentralJobID] = job
	return job, true, nil
}

func (s *MemoryStore) UpdateJobStep(ctx context.Context, centralJobID, status, step string, failureReason *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobs[centralJobID]
	job.Status = status
	job.LastStep = step
	job.FailureReason = failureReason
	s.jobs[centralJobID] = job
	return nil
}

func (s *MemoryStore) GetJobByCentralID(ctx context.Context, centralJobID string) (LocalJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobs[centralJobID], nil
}

func (s *MemoryStore) ActiveJobsCount(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int
	for _, job := range s.jobs {
		if job.Status == "running" {
			count++
		}
	}
	return count, nil
}

func (s *MemoryStore) AlreadyProcessed(ctx context.Context, messageID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.processed[messageID]
	return ok, nil
}

func (s *MemoryStore) MarkProcessed(ctx context.Context, message ProcessedMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processed[message.MessageID] = message
	return nil
}
