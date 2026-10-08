package app

import (
	"fmt"
	"sync"
	"time"

	"ai-dev-manager-v2/internal/projectanalysis"
)

// ProjectIndexJob is a safe, bounded snapshot of a long-running index refresh.
// It intentionally omits the full index artifacts, which may contain source.
type ProjectIndexJob struct {
	ID            string     `json:"job_id"`
	EnvironmentID string     `json:"environment_id"`
	State         string     `json:"state"`
	Phase         string     `json:"phase"`
	FilesScanned  int        `json:"files_scanned"`
	TotalFiles    int        `json:"total_files"`
	FilesIndexed  int        `json:"files_indexed,omitempty"`
	Symbols       int        `json:"symbols,omitempty"`
	Truncated     bool       `json:"truncated,omitempty"`
	Error         string     `json:"error,omitempty"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}
type projectIndexJob struct {
	mu       sync.Mutex
	snapshot ProjectIndexJob
}

func (j *projectIndexJob) get() ProjectIndexJob { j.mu.Lock(); defer j.mu.Unlock(); return j.snapshot }
func (j *projectIndexJob) set(fn func(*ProjectIndexJob)) {
	j.mu.Lock()
	defer j.mu.Unlock()
	fn(&j.snapshot)
}

// StartProjectIndexJob immediately returns a task ID. The task belongs to
// the ADM service, not the Desktop modal or one HTTP/MCP request.
func (s *Service) StartProjectIndexJob(id string) (ProjectIndexJob, error) {
	if _, err := s.Environments.Get(id); err != nil {
		return ProjectIndexJob{}, err
	}
	s.indexJobsMu.Lock()
	if s.indexJobs == nil {
		s.indexJobs = map[string]*projectIndexJob{}
	}
	if j := s.indexJobs[id]; j != nil {
		state := j.get()
		if state.State == "running" || state.State == "queued" {
			s.indexJobsMu.Unlock()
			return state, nil
		}
	}
	started := time.Now().UTC()
	job := &projectIndexJob{snapshot: ProjectIndexJob{
		ID: fmt.Sprintf("index-%d", started.UnixNano()), EnvironmentID: id,
		State: "queued", Phase: "排队等待索引锁", StartedAt: started,
	}}
	s.indexJobs[id] = job
	s.indexJobsMu.Unlock()
	go s.runProjectIndexJob(job, id)
	return job.get(), nil
}

func (s *Service) runProjectIndexJob(job *projectIndexJob, id string) {
	defer func() {
		if value := recover(); value != nil {
			now := time.Now().UTC()
			job.set(func(v *ProjectIndexJob) {
				v.State = "failed"
				v.Phase = "分析意外中断"
				v.Error = "索引任务意外失败"
				v.FinishedAt = &now
			})
		}
	}()
	job.set(func(v *ProjectIndexJob) { v.State = "running"; v.Phase = "准备扫描" })
	result, err := s.AnalyzeProjectWithProgress(id, "", 0, 0, func(p projectanalysis.Progress) {
		job.set(func(v *ProjectIndexJob) {
			v.State = "running"
			v.Phase = "扫描源码"
			v.FilesScanned = p.FilesScanned
			v.TotalFiles = p.TotalFiles
			if p.TotalFiles > 0 && p.FilesScanned >= p.TotalFiles {
				v.Phase = "生成并发布索引"
			}
		})
	})
	completed := time.Now().UTC()
	job.set(func(v *ProjectIndexJob) {
		v.FinishedAt = &completed
		if err != nil {
			v.State = "failed"
			v.Phase = "任务失败"
			v.Error = err.Error()
			return
		}
		v.State = "succeeded"
		v.Phase = "索引更新完成"
		v.FilesIndexed = result.FilesIndexed
		v.Symbols = result.Symbols
		v.Truncated = result.Truncated
		v.FilesScanned = result.FilesScanned
	})
}

// ProjectIndexJobStatus returns the latest task for an Environment after
// reconnecting/reopening its detail dialog. Never waits for completion.
func (s *Service) ProjectIndexJobStatus(id string) (ProjectIndexJob, error) {
	if _, err := s.Environments.Get(id); err != nil {
		return ProjectIndexJob{}, err
	}
	s.indexJobsMu.Lock()
	job := s.indexJobs[id]
	s.indexJobsMu.Unlock()
	if job == nil {
		return ProjectIndexJob{EnvironmentID: id, State: "idle", Phase: "尚未启动后台索引任务"}, nil
	}
	return job.get(), nil
}
