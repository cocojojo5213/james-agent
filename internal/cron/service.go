package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	rcron "github.com/robfig/cron/v3"

	"github.com/cocojojo5213/james-agent/internal/shared"
)

type Service struct {
	storePath string
	mu        sync.Mutex
	jobs      []CronJob
	OnJob     func(job CronJob) (string, error)
	cron      *rcron.Cron
	entryMap  map[string]rcron.EntryID // job ID -> cron entry ID
}

func NewService(storePath string) *Service {
	return &Service{
		storePath: storePath,
		entryMap:  make(map[string]rcron.EntryID),
	}
}

func (s *Service) Start(ctx context.Context) error {
	if err := s.load(); err != nil {
		slog.Warn("failed to load cron jobs", "error", err)
	}

	s.cron = rcron.New(rcron.WithSeconds())

	s.mu.Lock()
	for i := range s.jobs {
		if s.jobs[i].Enabled && s.jobs[i].Schedule.Kind == "cron" {
			s.registerJob(&s.jobs[i])
		}
	}
	s.mu.Unlock()

	s.cron.Start()
	slog.Info("cron started", "job_count", len(s.jobs))

	// Handle "every" and "at" jobs in a separate goroutine
	go s.tickLoop(ctx)

	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

func (s *Service) registerJob(job *CronJob) {
	jobCopy := *job
	id, err := s.cron.AddFunc(job.Schedule.Expr, func() {
		s.executeJob(jobCopy)
	})
	if err != nil {
		slog.Error("failed to register cron job", "name", job.Name, "expr", job.Schedule.Expr, "error", err)
		return
	}
	s.entryMap[job.ID] = id
}

func (s *Service) executeJob(job CronJob) {
	slog.Info("executing cron job", "name", job.Name, "id", job.ID)

	if s.OnJob == nil {
		slog.Warn("no OnJob handler set")
		return
	}

	result, err := s.OnJob(job)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if s.jobs[i].ID == job.ID {
			s.jobs[i].State.LastRunAtMs = time.Now().UnixMilli()
			if err != nil {
				s.jobs[i].State.LastStatus = "error"
				s.jobs[i].State.LastError = err.Error()
				slog.Error("cron job failed", "name", job.Name, "error", err)
			} else {
				s.jobs[i].State.LastStatus = "ok"
				s.jobs[i].State.LastError = ""
				slog.Info("cron job completed", "name", job.Name, "result", shared.Truncate(result, 100))
			}

			if s.jobs[i].DeleteAfterRun {
				s.jobs = append(s.jobs[:i], s.jobs[i+1:]...)
			}
			break
		}
	}

	_ = s.save()
}

func (s *Service) tickLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now().UnixMilli()
			s.mu.Lock()
			for i := range s.jobs {
				job := &s.jobs[i]
				if !job.Enabled {
					continue
				}
				switch job.Schedule.Kind {
				case "every":
					if job.Schedule.EveryMs > 0 {
						nextRun := job.State.LastRunAtMs + job.Schedule.EveryMs
						if now >= nextRun {
							jobCopy := *job
							s.mu.Unlock()
							s.executeJob(jobCopy)
							s.mu.Lock()
						}
					}
				case "at":
					if job.Schedule.AtMs > 0 && now >= job.Schedule.AtMs {
						jobCopy := *job
						job.Enabled = false
						s.mu.Unlock()
						s.executeJob(jobCopy)
						s.mu.Lock()
					}
				}
			}
			s.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
	slog.Info("cron stopped")
}

func (s *Service) AddJob(name string, schedule Schedule, payload Payload) (*CronJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job := NewCronJob(name, schedule, payload)
	s.jobs = append(s.jobs, job)

	if job.Schedule.Kind == "cron" && s.cron != nil {
		s.registerJob(&s.jobs[len(s.jobs)-1])
	}

	if err := s.save(); err != nil {
		return nil, fmt.Errorf("save jobs: %w", err)
	}

	return &job, nil
}

func (s *Service) RemoveJob(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, job := range s.jobs {
		if job.ID == id {
			if entryID, ok := s.entryMap[id]; ok {
				s.cron.Remove(entryID)
				delete(s.entryMap, id)
			}
			s.jobs = append(s.jobs[:i], s.jobs[i+1:]...)
			_ = s.save()
			return true
		}
	}
	return false
}

func (s *Service) ListJobs() []CronJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]CronJob, len(s.jobs))
	copy(result, s.jobs)
	return result
}

func (s *Service) EnableJob(id string, enabled bool) (*CronJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if s.jobs[i].ID == id {
			s.jobs[i].Enabled = enabled
			_ = s.save()
			job := s.jobs[i]
			return &job, nil
		}
	}
	return nil, fmt.Errorf("job %s not found", id)
}

func (s *Service) load() error {
	data, err := os.ReadFile(s.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &s.jobs)
}

func (s *Service) save() error {
	dir := filepath.Dir(s.storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.jobs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.storePath, data, 0644)
}

