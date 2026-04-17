package airunner

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// JobStatus represents the current state of a job.
type JobStatus string

const (
	JobStatusRunning   JobStatus = "running"
	JobStatusDone      JobStatus = "done"
	JobStatusError     JobStatus = "error"
	JobStatusCancelled JobStatus = "cancelled"
)

// JobEvent is a single event in a job's event stream, with an index for
// incremental polling via ?after=<index>.
type JobEvent struct {
	Index int    `json:"index"`
	Type  string `json:"type"`

	// Fields mirror airunner.Event.
	Provider   string  `json:"provider,omitempty"`
	SessionID  string  `json:"session_id,omitempty"`
	Model      string  `json:"model,omitempty"`
	Name       string  `json:"name,omitempty"`
	Content    string  `json:"content,omitempty"`
	Error      string  `json:"error,omitempty"`
	DurationMS int     `json:"duration_ms,omitempty"`
	CostUSD    float64 `json:"cost_usd,omitempty"`
}

// Job tracks a running or completed AI job.
type Job struct {
	ID        string    `json:"job_id"`
	Status    JobStatus `json:"status"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model,omitempty"`
	SessionID string    `json:"session_id,omitempty"` // linked session ID
	UserID    string    `json:"user_id,omitempty"`

	mu          sync.RWMutex
	events      []JobEvent
	result      *RunResult // set when done
	cancel      context.CancelFunc
	doneAt      time.Time
	subscribers []chan Event // real-time event subscribers
}

// Events returns events after the given index.
func (j *Job) Events(after int) []JobEvent {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if after < 0 {
		after = -1
	}
	var out []JobEvent
	for _, ev := range j.events {
		if ev.Index > after {
			out = append(out, ev)
		}
	}
	return out
}

// Subscribe returns a channel that receives events in real-time.
// The channel is closed when the job finishes. Call Unsubscribe to stop early.
func (j *Job) Subscribe() chan Event {
	j.mu.Lock()
	defer j.mu.Unlock()
	ch := make(chan Event, 64)
	j.subscribers = append(j.subscribers, ch)
	return ch
}

// Unsubscribe removes and closes a subscriber channel.
func (j *Job) Unsubscribe(ch chan Event) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for i, sub := range j.subscribers {
		if sub == ch {
			j.subscribers = append(j.subscribers[:i], j.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// appendEvent adds an event to the job's buffer and notifies subscribers.
func (j *Job) appendEvent(ev Event) {
	j.mu.Lock()
	defer j.mu.Unlock()
	je := JobEvent{
		Index:      len(j.events),
		Type:       ev.Type,
		Provider:   ev.Provider,
		SessionID:  ev.SessionID,
		Model:      ev.Model,
		Name:       ev.Name,
		Content:    ev.Content,
		Error:      ev.Error,
		DurationMS: ev.DurationMS,
		CostUSD:    ev.CostUSD,
	}
	j.events = append(j.events, je)

	// Notify real-time subscribers (non-blocking).
	for _, ch := range j.subscribers {
		select {
		case ch <- ev:
		default:
			// subscriber too slow, skip
		}
	}
}

// finish marks the job as done with a result and closes all subscriber channels.
func (j *Job) finish(status JobStatus, result *RunResult) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = status
	j.result = result
	j.doneAt = time.Now()
	for _, ch := range j.subscribers {
		close(ch)
	}
	j.subscribers = nil
}

// Result returns the run result (nil if still running).
func (j *Job) Result() *RunResult {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.result
}

// JobView is the JSON-serializable view of a job for the poll endpoint.
type JobView struct {
	ID       string     `json:"job_id"`
	Status   JobStatus  `json:"status"`
	Provider string     `json:"provider"`
	Model    string     `json:"model,omitempty"`
	Events   []JobEvent `json:"events"`
	Result   string     `json:"result,omitempty"` // full text, only set when done
}

// JobStore manages in-memory jobs with a cleanup goroutine.
type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewJobStore creates a job store and starts a background cleanup goroutine
// that removes completed jobs older than 10 minutes.
func NewJobStore() *JobStore {
	s := &JobStore{jobs: make(map[string]*Job)}
	go s.cleanup()
	return s
}

// Submit creates a new job and starts it in a background goroutine.
// Returns the job immediately.
func (s *JobStore) Submit(
	jobID string,
	provider string,
	model string,
	runner Runner,
	cfg RunConfig,
	sessionID string,
	userID string,
) *Job {
	ctx, cancel := context.WithCancel(context.Background())

	j := &Job{
		ID:        jobID,
		Status:    JobStatusRunning,
		Provider:  provider,
		Model:     model,
		SessionID: sessionID,
		UserID:    userID,
		cancel:    cancel,
	}

	s.mu.Lock()
	s.jobs[jobID] = j
	s.mu.Unlock()

	go func() {
		result := runner.Run(ctx, cfg, sessionID, func(ev Event) {
			j.appendEvent(ev)
		})
		status := JobStatusDone
		if ctx.Err() != nil {
			status = JobStatusCancelled
		}
		j.finish(status, &result)
	}()

	return j
}

// Get returns a job by ID.
func (s *JobStore) Get(jobID string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[jobID]
	return j, ok
}

// List returns all jobs, optionally filtered by user ID.
func (s *JobStore) List(userID string) []JobView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []JobView
	for _, j := range s.jobs {
		if userID != "" && j.UserID != userID {
			continue
		}
		j.mu.RLock()
		view := JobView{
			ID:       j.ID,
			Status:   j.Status,
			Provider: j.Provider,
			Model:    j.Model,
		}
		if j.result != nil {
			view.Result = j.result.Text
		}
		j.mu.RUnlock()
		out = append(out, view)
	}
	return out
}

// Cancel cancels a running job.
func (s *JobStore) Cancel(jobID string) error {
	s.mu.RLock()
	j, ok := s.jobs[jobID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}
	j.mu.RLock()
	status := j.Status
	j.mu.RUnlock()
	if status != JobStatusRunning {
		return fmt.Errorf("job %s is not running (status: %s)", jobID, status)
	}
	j.cancel()
	return nil
}

// cleanup periodically removes completed jobs older than 10 minutes.
func (s *JobStore) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, j := range s.jobs {
			j.mu.RLock()
			done := j.Status != JobStatusRunning
			age := now.Sub(j.doneAt)
			j.mu.RUnlock()
			if done && age > 10*time.Minute {
				delete(s.jobs, id)
			}
		}
		s.mu.Unlock()
	}
}
