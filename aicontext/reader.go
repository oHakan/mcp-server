package aicontext

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectContext docs/ai/ dizininden okunan proje bağlamını tutar.
type ProjectContext struct {
	ProjectPath  string
	Standards    string   // standards.md içeriği, yoksa ""
	Architecture string   // architecture.md içeriği, yoksa ""
	Stack        string   // stack.md içeriği, yoksa ""
	Context      string   // context.md içeriği, yoksa ""
	PlanCount    int      // docs/ai/plans/ içindeki MD dosya sayısı
	MissingFiles []string // bulunamayan dosyaların listesi
}

// Load docs/ai/*.md dosyalarını okur. Eksik dosyalar hata yerine MissingFiles'a yazılır.
func Load(projectPath string) (*ProjectContext, error) {
	ctx := &ProjectContext{
		ProjectPath:  projectPath,
		MissingFiles: []string{},
	}

	docsPath := filepath.Join(projectPath, "docs", "ai")

	files := []struct {
		name string
		dest *string
	}{
		{"standards.md", &ctx.Standards},
		{"architecture.md", &ctx.Architecture},
		{"stack.md", &ctx.Stack},
		{"context.md", &ctx.Context},
	}

	for _, f := range files {
		path := filepath.Join(docsPath, f.name)
		data, err := os.ReadFile(path)
		if err != nil {
			ctx.MissingFiles = append(ctx.MissingFiles, f.name)
			continue
		}
		*f.dest = string(data)
	}

	// plans/ dizinindeki MD dosyalarını say (içerik yüklenmez)
	plansPath := filepath.Join(docsPath, "plans")
	entries, err := os.ReadDir(plansPath)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
				ctx.PlanCount++
			}
		}
	}

	return ctx, nil
}

// Summary proje bağlamının kısa özetini döner.
func (c *ProjectContext) Summary() string {
	parts := []string{}

	if c.Stack != "" {
		firstLine := strings.SplitN(c.Stack, "\n", 2)[0]
		parts = append(parts, "Stack: "+strings.TrimSpace(firstLine))
	} else {
		parts = append(parts, "Stack: bilinmiyor")
	}

	if c.Architecture != "" {
		firstLine := strings.SplitN(c.Architecture, "\n", 2)[0]
		parts = append(parts, "Mimari: "+strings.TrimSpace(firstLine))
	} else {
		parts = append(parts, "Mimari: bilinmiyor")
	}

	if c.Standards != "" {
		parts = append(parts, "Standartlar: mevcut")
	} else {
		parts = append(parts, "Standartlar: eksik")
	}

	if c.PlanCount > 0 {
		parts = append(parts, fmt.Sprintf("Önceki planlar: %d", c.PlanCount))
	}

	return strings.Join(parts, ". ") + "."
}
