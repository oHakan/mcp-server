package analyzer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// GeneratedFiles üretilen MD dosyalarının içeriklerini tutar.
type GeneratedFiles struct {
	Stack        string
	Architecture string
	Context      string
	Standards    string
}

// Generate ProjectScan'den 4 MD dosyasının içeriğini üretir.
func Generate(scan *ProjectScan) *GeneratedFiles {
	return &GeneratedFiles{
		Stack:        generateStack(scan),
		Architecture: generateArchitecture(scan),
		Context:      generateContext(scan),
		Standards:    generateStandards(scan),
	}
}

// WriteToProject sadece missing listesindeki dosyaları diske yazar.
// Döndürür: gerçekten yazılan dosyaların basename listesi.
func WriteToProject(projectPath string, missing []string, scan *ProjectScan, logger *log.Logger) []string {
	docsPath := filepath.Join(projectPath, "docs", "ai")
	if err := os.MkdirAll(docsPath, 0o755); err != nil {
		if logger != nil {
			logger.Printf("analyzer: docs/ai dizini oluşturulamadı: %v", err)
		}
		return []string{}
	}

	gen := Generate(scan)

	fileContents := map[string]string{
		"stack.md":        gen.Stack,
		"architecture.md": gen.Architecture,
		"context.md":      gen.Context,
		"standards.md":    gen.Standards,
	}

	var written []string
	for _, name := range missing {
		content, ok := fileContents[name]
		if !ok {
			continue
		}
		path := filepath.Join(docsPath, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			if logger != nil {
				logger.Printf("analyzer: %s yazılamadı: %v", name, err)
			}
			continue
		}
		if logger != nil {
			logger.Printf("analyzer: %s otomatik oluşturuldu", name)
		}
		written = append(written, name)
	}

	return written
}

func generateStack(scan *ProjectScan) string {
	var b strings.Builder

	lang := scan.Lang
	if lang.Name == "Unknown" || lang.Name == "" {
		b.WriteString("Unknown\n\n")
		b.WriteString("<!-- auto-generated: no manifest file detected -->\n")
		return b.String()
	}

	// İlk satır: dil adı ve versiyon
	if lang.Version != "" {
		fmt.Fprintf(&b, "%s (%s)\n\n", lang.Name, lang.Version)
	} else {
		fmt.Fprintf(&b, "%s\n\n", lang.Name)
	}

	b.WriteString("## Runtime\n\n")
	if lang.Framework != "" {
		fmt.Fprintf(&b, "- Language: %s\n", lang.Name)
		fmt.Fprintf(&b, "- Framework: %s\n", lang.Framework)
	} else {
		fmt.Fprintf(&b, "- Language: %s\n", lang.Name)
	}
	if lang.Version != "" {
		fmt.Fprintf(&b, "- Version: %s\n", lang.Version)
	}
	if lang.ModuleName != "" {
		fmt.Fprintf(&b, "- Module: %s\n", lang.ModuleName)
	}
	b.WriteString("\n")

	if len(lang.KeyDeps) > 0 {
		b.WriteString("## Key Dependencies\n\n")
		for _, dep := range lang.KeyDeps {
			fmt.Fprintf(&b, "- %s\n", dep)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Build\n\n")
	if lang.BuildCmd != "" {
		fmt.Fprintf(&b, "```\n%s\n```\n\n", lang.BuildCmd)
	} else {
		b.WriteString("<!-- no build command detected -->\n\n")
	}

	b.WriteString("## Package Manager\n\n")
	if lang.PkgManager != "" {
		fmt.Fprintf(&b, "%s\n", lang.PkgManager)
	} else {
		b.WriteString("<!-- no package manager detected -->\n")
	}

	return b.String()
}

func generateArchitecture(scan *ProjectScan) string {
	var b strings.Builder

	arch := scan.Arch
	pattern := arch.Pattern
	if pattern == "" {
		pattern = "Layered package structure"
	}

	// İlk satır: pattern adı
	fmt.Fprintf(&b, "%s\n\n", pattern)

	b.WriteString("## Pattern\n\n")
	fmt.Fprintf(&b, "%s\n\n", pattern)

	if len(arch.Packages) > 0 {
		b.WriteString("## Packages\n\n")
		for _, pkg := range arch.Packages {
			fmt.Fprintf(&b, "- %s/\n", pkg)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Communication\n\n")
	if arch.CommType != "" {
		fmt.Fprintf(&b, "%s\n", arch.CommType)
	} else {
		b.WriteString("<!-- no specific communication type detected -->\n")
	}

	return b.String()
}

func generateContext(scan *ProjectScan) string {
	var b strings.Builder

	ctx := scan.Context
	desc := ctx.Description
	if desc == "" {
		desc = ctx.ModuleName
	}
	if desc == "" {
		desc = "<!-- auto-generated: no description found -->"
	}

	b.WriteString(desc)
	b.WriteString("\n\n")

	if ctx.ModuleName != "" && ctx.ModuleName != desc {
		fmt.Fprintf(&b, "## Module\n\n%s\n", ctx.ModuleName)
	}

	return b.String()
}

func generateStandards(scan *ProjectScan) string {
	var b strings.Builder

	s := scan.Standards
	hasAny := s.Linter != "" || s.Formatter != "" || s.HasCI || s.HasDocker || s.HasMake

	if !hasAny {
		b.WriteString("<!-- auto-generated: no standards config detected -->\n\n")
		if s.TestCmd != "" {
			b.WriteString("## Testing\n\n")
			fmt.Fprintf(&b, "```\n%s\n```\n", s.TestCmd)
		}
		return b.String()
	}

	b.WriteString("## Linting\n\n")
	if s.Linter != "" {
		fmt.Fprintf(&b, "- Linter: %s\n", s.Linter)
	} else {
		b.WriteString("- Linter: none detected\n")
	}
	b.WriteString("\n")

	b.WriteString("## Formatting\n\n")
	if s.Formatter != "" {
		fmt.Fprintf(&b, "- Formatter: %s\n", s.Formatter)
	} else {
		b.WriteString("- Formatter: none detected\n")
	}
	b.WriteString("\n")

	b.WriteString("## CI/CD\n\n")
	if s.HasCI {
		b.WriteString("- CI: enabled\n")
	} else {
		b.WriteString("- CI: none detected\n")
	}
	if s.HasDocker {
		b.WriteString("- Docker: enabled\n")
	}
	if s.HasMake {
		b.WriteString("- Make: enabled\n")
	}
	b.WriteString("\n")

	if s.TestCmd != "" {
		b.WriteString("## Testing\n\n")
		fmt.Fprintf(&b, "```\n%s\n```\n", s.TestCmd)
	}

	return b.String()
}
