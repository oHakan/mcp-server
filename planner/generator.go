package planner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ohakan-mcp/aicontext"
)

// PlanInput plan üretimi için gereken tüm girdileri tutar.
type PlanInput struct {
	Prompt      string
	Answers     string
	Context     *aicontext.ProjectContext
	ProjectPath string
}

// PlanOutput Generate() tarafından dönen sonucu tutar.
type PlanOutput struct {
	PlanMarkdown string
	DAGJSON      string // JSON string
	SavedPath    string // docs/ai/plans/... göreli yol; kayıt başarısızsa ""
}

// DAGTask bağımlılık grafiğindeki bir görevi temsil eder.
type DAGTask struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	DependsOn []string `json:"depends_on"`
	Action    string   `json:"action"`
	Files     []string `json:"files"`
}

// DAG yürütme planının bağımlılık grafiğini tutar.
type DAG struct {
	Tasks []DAGTask `json:"tasks"`
}

// Generate prompt, cevaplar ve proje bağlamını kullanarak plan üretir ve kaydeder.
func Generate(input PlanInput) (*PlanOutput, error) {
	dag := buildDAG(input.Prompt, input.Answers)

	dagJSON, err := json.MarshalIndent(dag, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("DAG JSON üretilemedi: %w", err)
	}

	planMD := buildMarkdown(input, string(dagJSON))

	savedPath, _ := savePlan(input.ProjectPath, input.Prompt, planMD)
	// Kayıt hatası kritik değil; savedPath "" olarak kalır.

	return &PlanOutput{
		PlanMarkdown: planMD,
		DAGJSON:      string(dagJSON),
		SavedPath:    savedPath,
	}, nil
}

// buildDAG prompt ve cevaplara göre keyword tabanlı basit bir DAG üretir.
func buildDAG(prompt, answers string) *DAG {
	lower := strings.ToLower(prompt + " " + answers)

	tasks := []DAGTask{
		{
			ID: "t1", Name: "Gereksinim Analizi", DependsOn: []string{},
			Action: "Prompt ve cevapları analiz et, kabul kriterlerini belirle",
			Files:  []string{},
		},
		{
			ID: "t2", Name: "Tasarım", DependsOn: []string{"t1"},
			Action: "Bileşen/modül tasarımını yap ve arayüzleri tanımla",
			Files:  []string{},
		},
	}

	nextID := idGenerator(3)

	if containsAny(lower, "db", "database", "sql", "migration") {
		tasks = append(tasks, DAGTask{
			ID: nextID(), Name: "DB Migration", DependsOn: []string{"t2"},
			Action: "Schema değişikliklerini yaz ve migration dosyalarını oluştur",
			Files:  []string{"migrations/"},
		})
	}

	if containsAny(lower, "auth", "login", "jwt", "oauth") {
		tasks = append(tasks, DAGTask{
			ID: nextID(), Name: "Auth Implementasyonu", DependsOn: []string{"t2"},
			Action: "Kimlik doğrulama modülünü implement et",
			Files:  []string{},
		})
	}

	if containsAny(lower, "api", "endpoint", "route") {
		tasks = append(tasks, DAGTask{
			ID: nextID(), Name: "API Endpoint'leri", DependsOn: []string{"t2"},
			Action: "HTTP handler ve route tanımlarını yaz",
			Files:  []string{},
		})
	}

	if containsAny(lower, "test", "spec") {
		// Test görevi şu ana kadar eklenen domain görevlerine bağlı.
		domainDeps := taskIDs(tasks[2:])
		if len(domainDeps) == 0 {
			domainDeps = []string{"t2"}
		}
		tasks = append(tasks, DAGTask{
			ID: nextID(), Name: "Testler", DependsOn: domainDeps,
			Action: "Unit ve entegrasyon testlerini yaz",
			Files:  []string{},
		})
	}

	// Son görev: her şeyin tamamlanmasını bekler.
	lastDeps := taskIDs(tasks[2:])
	if len(lastDeps) == 0 {
		lastDeps = []string{"t2"}
	}
	tasks = append(tasks, DAGTask{
		ID: nextID(), Name: "Entegrasyon & Doğrulama", DependsOn: lastDeps,
		Action: "Tüm bileşenleri entegre et ve kabul kriterlerini doğrula",
		Files:  []string{},
	})

	return &DAG{Tasks: tasks}
}

// idGenerator t3, t4, ... şeklinde ID'ler üreten closure döner.
func idGenerator(start int) func() string {
	n := start
	return func() string {
		id := fmt.Sprintf("t%d", n)
		n++
		return id
	}
}

// taskIDs bir dilim içindeki görevlerin ID'lerini döner.
func taskIDs(tasks []DAGTask) []string {
	ids := make([]string, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	return ids
}

// containsAny s içinde verilen keyword'lerden herhangi birinin geçip geçmediğini kontrol eder.
func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// buildMarkdown plan içeriğini Markdown formatında oluşturur.
func buildMarkdown(input PlanInput, dagJSON string) string {
	ctx := input.Context
	date := time.Now().Format("2006-01-02")

	standardsSummary := ctx.Standards
	if standardsSummary == "" {
		standardsSummary = "standards.md bulunamadı."
	} else {
		lines := strings.Split(standardsSummary, "\n")
		if len(lines) > 10 {
			lines = lines[:10]
		}
		standardsSummary = strings.Join(lines, "\n")
	}

	return fmt.Sprintf(
		"# Plan: %s\n"+
			"**Tarih:** %s  **Durum:** generated\n\n"+
			"## Bağlam\n%s\n\n"+
			"## Görev Tanımı\n%s\n\n"+
			"## Açıklamalar\n%s\n\n"+
			"## Yürütme Planı (DAG)\n"+
			"```json\n%s\n```\n\n"+
			"## Standartlar & Kısıtlar\n%s\n",
		input.Prompt,
		date,
		ctx.Summary(),
		input.Prompt,
		input.Answers,
		dagJSON,
		standardsSummary,
	)
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// savePlan planı docs/ai/plans/YYYY-MM-DD-<slug>.md dosyasına kaydeder.
func savePlan(projectPath, prompt, content string) (string, error) {
	plansDir := filepath.Join(projectPath, "docs", "ai", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		return "", fmt.Errorf("plans dizini oluşturulamadı: %w", err)
	}

	date := time.Now().Format("2006-01-02")
	slug := slugify(prompt)
	filename := fmt.Sprintf("%s-%s.md", date, slug)
	fullPath := filepath.Join(plansDir, filename)

	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("plan dosyası yazılamadı: %w", err)
	}

	return filepath.Join("docs", "ai", "plans", filename), nil
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}
