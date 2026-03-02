package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// PlanStatus planın genel durumunu gösterir.
type PlanStatus string

const (
	PlanInProgress PlanStatus = "in_progress"
	PlanCompleted  PlanStatus = "completed"
	PlanFailed     PlanStatus = "failed"
)

// TaskStatus bir task'ın durumunu gösterir.
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskDone       TaskStatus = "done"
	TaskFailed     TaskStatus = "failed"
)

// TaskState tek bir task'ın yürütme sonucunu tutar.
type TaskState struct {
	Status       TaskStatus `json:"status"`
	Summary      string     `json:"summary,omitempty"`
	ChangedFiles []string   `json:"changed_files,omitempty"`
	CompletedAt  string     `json:"completed_at,omitempty"`
}

// PlanState planın tüm yürütme durumunu tutar.
// State dosyası: docs/ai/plans/<name>.state.json
type PlanState struct {
	PlanPath    string                `json:"plan_path"`
	PlanTitle   string                `json:"plan_title"`
	ProjectPath string                `json:"project_path"`
	StartedAt   string                `json:"started_at"`
	CompletedAt string                `json:"completed_at,omitempty"`
	Status      PlanStatus            `json:"status"`
	TotalTasks  int                   `json:"total_tasks"`
	Tasks       map[string]*TaskState `json:"tasks"`
	// Batch bilgisi — hangi task'lar hangi batch'te
	BatchMap map[string]int `json:"batch_map"`
}

// StatePathFor plan MD yolundan state JSON yolunu türetir.
// "docs/ai/plans/2026-03-02-foo.md" → "docs/ai/plans/2026-03-02-foo.state.json"
func StatePathFor(planPath string) string {
	base := strings.TrimSuffix(planPath, ".md")
	return base + ".state.json"
}

// ReportPathFor plan MD yolundan report MD yolunu türetir.
func ReportPathFor(planPath string) string {
	base := strings.TrimSuffix(planPath, ".md")
	return base + ".report.md"
}

// InitState execute_plan çağrısında state dosyasını ilklendirir.
// Sadece dosya yoksa yazar; varsa mevcut state korunur.
func InitState(projectPath, planPath, planTitle string, batches []ExecutionBatch) (*PlanState, error) {
	statePath := resolveStatePath(projectPath, planPath)

	// Varsa yükle
	if existing, err := loadStateFromPath(statePath); err == nil {
		return existing, nil
	}

	// Task map'i oluştur — tüm task'lar pending
	tasks := make(map[string]*TaskState)
	batchMap := make(map[string]int)
	total := 0
	for _, batch := range batches {
		for _, t := range batch.Tasks {
			tasks[t.ID] = &TaskState{Status: TaskPending}
			batchMap[t.ID] = batch.BatchIndex
			total++
		}
	}

	state := &PlanState{
		PlanPath:    planPath,
		PlanTitle:   planTitle,
		ProjectPath: projectPath,
		StartedAt:   time.Now().Format(time.RFC3339),
		Status:      PlanInProgress,
		TotalTasks:  total,
		Tasks:       tasks,
		BatchMap:    batchMap,
	}

	if err := saveStateToPath(statePath, state); err != nil {
		return nil, fmt.Errorf("state dosyası oluşturulamadı: %w", err)
	}

	return state, nil
}

// LoadState plan yolundan state'i yükler.
func LoadState(projectPath, planPath string) (*PlanState, error) {
	statePath := resolveStatePath(projectPath, planPath)
	return loadStateFromPath(statePath)
}

// SaveState state'i diske yazar.
func SaveState(projectPath, planPath string, state *PlanState) error {
	statePath := resolveStatePath(projectPath, planPath)
	return saveStateToPath(statePath, state)
}

// UpdateTask state'teki bir task'ı günceller.
// Tüm task'lar tamamlandıysa plan status'unu da günceller.
// Döner: güncellenmiş state, planın tamamlanıp tamamlanmadığı
func UpdateTask(projectPath, planPath, taskID string, status TaskStatus, summary string, changedFiles []string) (*PlanState, bool, error) {
	state, err := LoadState(projectPath, planPath)
	if err != nil {
		return nil, false, fmt.Errorf("state yüklenemedi: %w", err)
	}

	task, ok := state.Tasks[taskID]
	if !ok {
		return nil, false, fmt.Errorf("bilinmeyen task ID: %q", taskID)
	}

	task.Status = status
	task.Summary = summary
	task.ChangedFiles = changedFiles
	task.CompletedAt = time.Now().Format(time.RFC3339)

	// Plan tamamlanma kontrolü
	planDone := false
	allDone := true
	anyFailed := false
	for _, t := range state.Tasks {
		switch t.Status {
		case TaskFailed:
			anyFailed = true
		case TaskDone:
			// ok
		default:
			allDone = false
		}
	}

	if allDone || (anyFailed && !hasAnyPending(state.Tasks)) {
		if anyFailed {
			state.Status = PlanFailed
		} else {
			state.Status = PlanCompleted
		}
		state.CompletedAt = time.Now().Format(time.RFC3339)
		planDone = true
	}

	if err := SaveState(projectPath, planPath, state); err != nil {
		return nil, false, err
	}

	return state, planDone, nil
}

// DoneCount tamamlanan (done + failed) task sayısını döner.
func (s *PlanState) DoneCount() int {
	n := 0
	for _, t := range s.Tasks {
		if t.Status == TaskDone || t.Status == TaskFailed {
			n++
		}
	}
	return n
}

// AllChangedFiles tüm task'lardan değişen dosyaları birleşik döner (tekrarsız).
func (s *PlanState) AllChangedFiles() []string {
	seen := map[string]bool{}
	var result []string
	for _, t := range s.Tasks {
		for _, f := range t.ChangedFiles {
			if !seen[f] {
				seen[f] = true
				result = append(result, f)
			}
		}
	}
	return result
}

// ---- iç yardımcılar ----

func resolveStatePath(projectPath, planPath string) string {
	sp := StatePathFor(planPath)
	if projectPath != "" && !isAbsolute(sp) {
		return projectPath + "/" + sp
	}
	return sp
}

func isAbsolute(p string) bool {
	return len(p) > 0 && p[0] == '/'
}

func hasAnyPending(tasks map[string]*TaskState) bool {
	for _, t := range tasks {
		if t.Status == TaskPending {
			return true
		}
	}
	return false
}

func loadStateFromPath(statePath string) (*PlanState, error) {
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}
	var state PlanState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("state JSON parse hatası: %w", err)
	}
	return &state, nil
}

func saveStateToPath(statePath string, state *PlanState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("state JSON üretilemedi: %w", err)
	}
	return os.WriteFile(statePath, data, 0o644)
}
