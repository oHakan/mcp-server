package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GenerateReport tamamlanmış (veya başarısız) bir plan için
// rapor MD içeriğini üretir ve diske yazar.
// Döner: rapor dosyasının yolu, hata
func GenerateReport(projectPath string, state *PlanState, batches []ExecutionBatch) (string, error) {
	content := buildReportContent(state, batches)

	reportPath := resolveReportPath(projectPath, state.PlanPath)
	if err := os.WriteFile(reportPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("rapor yazılamadı: %w", err)
	}

	return reportPath, nil
}

// MarkPlanCompleted orijinal plan MD dosyasındaki status satırını günceller.
// "**Durum:** generated" → "**Durum:** completed" (veya failed)
func MarkPlanCompleted(projectPath, planPath string, status PlanStatus) error {
	fullPath := resolvePlanPath(projectPath, planPath)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("plan dosyası okunamadı: %w", err)
	}

	content := string(data)
	statusStr := "completed ✓"
	if status == PlanFailed {
		statusStr = "failed ✗"
	}

	updated := replaceStatusLine(content, statusStr)
	return os.WriteFile(fullPath, []byte(updated), 0o644)
}

// ---- iç yardımcılar ----

func buildReportContent(state *PlanState, batches []ExecutionBatch) string {
	var b strings.Builder

	statusEmoji := "✓"
	if state.Status == PlanFailed {
		statusEmoji = "✗"
	}

	fmt.Fprintf(&b, "# Yürütme Raporu: %s\n\n", state.PlanTitle)

	// Özet başlık
	b.WriteString("## Özet\n\n")
	fmt.Fprintf(&b, "| Alan | Değer |\n")
	fmt.Fprintf(&b, "|------|-------|\n")
	fmt.Fprintf(&b, "| Durum | %s %s |\n", strings.Title(string(state.Status)), statusEmoji)
	fmt.Fprintf(&b, "| Proje | `%s` |\n", state.ProjectPath)
	fmt.Fprintf(&b, "| Başlangıç | %s |\n", formatTime(state.StartedAt))
	fmt.Fprintf(&b, "| Bitiş | %s |\n", formatTime(state.CompletedAt))
	if state.TotalTasks > 0 {
		fmt.Fprintf(&b, "| Tamamlanan | %d / %d görev |\n", state.DoneCount(), state.TotalTasks)
	}
	b.WriteString("\n")

	// Etkilenen dosyalar
	allFiles := state.AllChangedFiles()
	if len(allFiles) > 0 {
		sort.Strings(allFiles)
		b.WriteString("## Etkilenen Dosyalar\n\n")
		for _, f := range allFiles {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
		b.WriteString("\n")
	}

	// Task detayları — batch sırasına göre
	b.WriteString("## Görev Detayları\n\n")

	// batch'leri varsa kullan, yoksa tasks map'ini sırala
	if len(batches) > 0 {
		for _, batch := range batches {
			if len(batch.Tasks) > 1 {
				fmt.Fprintf(&b, "### Batch %d (Paralel)\n\n", batch.BatchIndex)
			} else {
				fmt.Fprintf(&b, "### Batch %d\n\n", batch.BatchIndex)
			}
			for _, bt := range batch.Tasks {
				writeTaskDetail(&b, bt.ID, bt.Name, state.Tasks[bt.ID])
			}
		}
	} else {
		// Batch bilgisi yoksa task ID sırası ile yaz
		ids := make([]string, 0, len(state.Tasks))
		for id := range state.Tasks {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			writeTaskDetail(&b, id, id, state.Tasks[id])
		}
	}

	// Alt not
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "*Rapor otomatik oluşturuldu: %s*\n", time.Now().Format("2006-01-02 15:04:05"))

	return b.String()
}

func writeTaskDetail(b *strings.Builder, id, name string, ts *TaskState) {
	if ts == nil {
		fmt.Fprintf(b, "#### %s — %s\n\n*Durum bilgisi yok*\n\n", id, name)
		return
	}

	statusLabel := taskStatusLabel(ts.Status)
	fmt.Fprintf(b, "#### %s — %s %s\n\n", id, name, statusLabel)

	if ts.Summary != "" {
		fmt.Fprintf(b, "%s\n\n", ts.Summary)
	} else {
		b.WriteString("*Özet girilmedi.*\n\n")
	}

	if len(ts.ChangedFiles) > 0 {
		b.WriteString("**Değiştirilen dosyalar:**\n")
		for _, f := range ts.ChangedFiles {
			fmt.Fprintf(b, "- `%s`\n", f)
		}
		b.WriteString("\n")
	}

	if ts.CompletedAt != "" {
		fmt.Fprintf(b, "*Tamamlandı: %s*\n\n", formatTime(ts.CompletedAt))
	}
}

func taskStatusLabel(s TaskStatus) string {
	switch s {
	case TaskDone:
		return "✓"
	case TaskFailed:
		return "✗"
	case TaskPending:
		return "⏳"
	default:
		return ""
	}
}

func formatTime(t string) string {
	if t == "" {
		return "—"
	}
	parsed, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return t
	}
	return parsed.Format("2006-01-02 15:04:05")
}

func replaceStatusLine(content, newStatus string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "**Durum:**") {
			// Satırdaki mevcut durumu değiştir
			// Örn: "**Tarih:** 2026-03-02  **Durum:** generated"
			idx := strings.Index(line, "**Durum:**")
			lines[i] = line[:idx] + "**Durum:** " + newStatus
			break
		}
	}
	return strings.Join(lines, "\n")
}

func resolveReportPath(projectPath, planPath string) string {
	rp := ReportPathFor(planPath)
	if projectPath != "" && !isAbsolute(rp) {
		return filepath.Join(projectPath, rp)
	}
	return rp
}

func resolvePlanPath(projectPath, planPath string) string {
	if projectPath != "" && !isAbsolute(planPath) {
		return filepath.Join(projectPath, planPath)
	}
	return planPath
}
