package scheduler

import (
	"github.com/robfig/cron/v3"
	"github.com/cortexhub/cortex-gateway/internal/brain"
)

// Scheduler manages cron jobs for CortexBrain operations
type Scheduler struct {
	cron        *cron.Cron
	brainClient *brain.Client
}

// NewScheduler creates a new scheduler with brain client
func NewScheduler(brainClient *brain.Client) *Scheduler {
	s := &Scheduler{
		cron:        cron.New(),
		brainClient: brainClient,
	}
	s.scheduleSleepCycle()
	return s
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
}

// scheduleSleepCycle schedules the nightly sleep cycle at 3 AM
func (s *Scheduler) scheduleSleepCycle() {
	_, err := s.cron.AddFunc("0 3 * * *", func() {
		if err := s.brainClient.TriggerSleepCycle(false); err != nil {
			// Log error if logging is available
		}
	})
	if err != nil {
		// Handle error if needed
	}
}
