package executor

import (
	"fmt"
	"strings"

	"ohakan-mcp/aicontext"
)

// GeneratePrompts her batch'teki her task için SubAgentPrompt alanını doldurur.
// planPath boşsa report_task talimatı prompt'a eklenmez.
func GeneratePrompts(batches []ExecutionBatch, projectPath, planPath string, ctx *aicontext.ProjectContext) {
	// Tamamlanan task'ları takip et (bağımlılık bilgisi için)
	completed := make(map[string]string) // id → name

	for i := range batches {
		for j := range batches[i].Tasks {
			task := &batches[i].Tasks[j]
			task.SubAgentPrompt = buildPrompt(task, projectPath, planPath, ctx, completed)
		}
		// Bu batch'teki task'ları tamamlandı olarak işaretle
		for _, t := range batches[i].Tasks {
			completed[t.ID] = t.Name
		}
	}
}

func buildPrompt(task *ExecTask, projectPath, planPath string, ctx *aicontext.ProjectContext, completedTasks map[string]string) string {
	var b strings.Builder

	b.WriteString("Sen bir yazılım geliştirme sub-agent'ısın.\n")
	b.WriteString("Sana verilen TEK görevi tamamla, kapsam dışına çıkma.\n\n")

	fmt.Fprintf(&b, "**Proje:** `%s`\n", projectPath)

	// Proje bağlamı — sadece dolu alanlar
	if ctx.Stack != "" {
		firstLine := strings.SplitN(ctx.Stack, "\n", 2)[0]
		fmt.Fprintf(&b, "**Stack:** %s\n", strings.TrimSpace(firstLine))
	}
	if ctx.Architecture != "" {
		firstLine := strings.SplitN(ctx.Architecture, "\n", 2)[0]
		fmt.Fprintf(&b, "**Mimari:** %s\n", strings.TrimSpace(firstLine))
	}
	if ctx.Standards != "" {
		fmt.Fprintf(&b, "**Standartlar:** mevcut (`docs/ai/standards.md`)\n")
	}

	b.WriteString("\n---\n\n")

	fmt.Fprintf(&b, "## Görevin: %s\n\n", task.Name)
	fmt.Fprintf(&b, "%s\n\n", task.Action)

	if len(task.Files) > 0 {
		b.WriteString("**İlgili dosya/dizinler:**\n")
		for _, f := range task.Files {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
		b.WriteString("\n")
	}

	// Tamamlanan bağımlılıkları listele
	if len(task.DependsOn) > 0 {
		b.WriteString("**Bu görev tamamlandıktan sonra başlar (bağımlılıklar):**\n")
		for _, dep := range task.DependsOn {
			if name, ok := completedTasks[dep]; ok {
				fmt.Fprintf(&b, "- %s: %s ✓\n", dep, name)
			} else {
				fmt.Fprintf(&b, "- %s ✓\n", dep)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n\n")
	b.WriteString("## Tamamlama — ZORUNLU ADIM\n\n")
	b.WriteString("Görevi bitirince **mutlaka** `report_task` MCP tool'unu çağır:\n\n")
	b.WriteString("```\n")
	b.WriteString("tool: report_task\n")
	fmt.Fprintf(&b, "project_path: \"%s\"\n", projectPath)
	if planPath != "" {
		fmt.Fprintf(&b, "plan_path: \"%s\"\n", planPath)
	}
	fmt.Fprintf(&b, "task_id: \"%s\"\n", task.ID)
	b.WriteString("status: \"done\"  # veya \"failed\"\n")
	b.WriteString("summary: \"<ne yaptığının 2-3 cümlelik özeti>\"\n")
	b.WriteString("changed_files: \"<değiştirilen dosyalar virgülle: src/foo.go, src/bar.go>\"\n")
	b.WriteString("```\n\n")
	b.WriteString("> `report_task` çağrısını **atlama** — plan durumu ancak bu sayede güncellenir.\n")
	b.WriteString("> Tüm task'lar tamamlandığında sistem otomatik olarak rapor oluşturacak.\n")

	return b.String()
}

// BuildInstruction tüm batch'leri özetleyen ve çağıran AI'a
// paralel yürütmeyi nasıl uygulayacağını anlatan talimat üretir.
func BuildInstruction(batches []ExecutionBatch, planTitle string) string {
	var b strings.Builder

	totalTasks := 0
	for _, batch := range batches {
		totalTasks += len(batch.Tasks)
	}

	fmt.Fprintf(&b, "## Yürütme Talimatı: %s\n\n", planTitle)
	fmt.Fprintf(&b, "Plan **%d batch**, toplam **%d görev** içeriyor.\n\n", len(batches), totalTasks)

	b.WriteString("### Uygulama Adımları\n\n")
	b.WriteString("Her batch için:\n")
	b.WriteString("1. Batch içindeki **tüm görevleri tek mesajda** paralel Task/Agent çağrısı olarak gönder\n")
	b.WriteString("2. Tüm sub-agent'lar tamamlanana kadar bekle\n")
	b.WriteString("3. Sonraki batch'e geç\n\n")

	b.WriteString("### Batch Özeti\n\n")
	for _, batch := range batches {
		if len(batch.Tasks) == 1 {
			fmt.Fprintf(&b, "**Batch %d** (sıralı): %s\n",
				batch.BatchIndex, batch.Tasks[0].Name)
		} else {
			names := make([]string, len(batch.Tasks))
			for i, t := range batch.Tasks {
				names[i] = t.Name
			}
			fmt.Fprintf(&b, "**Batch %d** (%d paralel görev): %s\n",
				batch.BatchIndex, len(batch.Tasks), strings.Join(names, " + "))
		}
	}

	b.WriteString("\n> Her görevin `sub_agent_prompt` alanı hazır prompt içeriyor.\n")
	b.WriteString("> Sub-agent'a o prompt'u doğrudan ver.\n")
	b.WriteString("> Her sub-agent görevi bitince `report_task` çağırır — tüm görevler tamamlandığında rapor otomatik oluşur.\n")

	return b.String()
}
