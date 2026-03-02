package analyzer

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// LangInfo dil ve bağımlılık bilgisini tutar.
type LangInfo struct {
	Name       string
	Version    string
	ModuleName string
	Framework  string
	KeyDeps    []string // max 10
	BuildCmd   string
	PkgManager string
}

// ArchInfo mimari bilgisini tutar.
type ArchInfo struct {
	Pattern  string   // "Go standard layout", "MVC", "Clean Architecture", "Layered", vb.
	Packages []string // üst-düzey dizin adları (max 20)
	CommType string   // "stdio + HTTP", "HTTP", "gRPC", ""
}

// StandardsInfo kodlama standartları bilgisini tutar.
type StandardsInfo struct {
	Linter    string
	Formatter string
	HasCI     bool
	HasDocker bool
	HasMake   bool
	TestCmd   string
}

// ContextInfo proje bağlamı bilgisini tutar.
type ContextInfo struct {
	Description string
	Purpose     string
	ModuleName  string
}

// ProjectScan statik analiz sonucunu tutar.
type ProjectScan struct {
	Lang      LangInfo
	Arch      ArchInfo
	Standards StandardsInfo
	Context   ContextInfo
}

// ignoredDirs tarama sırasında atlanan dizinler.
var ignoredDirs = map[string]bool{
	"vendor": true, "node_modules": true, ".git": true,
	"dist": true, "build": true,
}

// Scan projeyi statik olarak tarar ve ProjectScan döndürür.
func Scan(projectPath string, logger *log.Logger) *ProjectScan {
	scan := &ProjectScan{}

	entries, err := os.ReadDir(projectPath)
	if err != nil {
		if logger != nil {
			logger.Printf("analyzer: ReadDir hata: %v", err)
		}
		scan.Lang.Name = "Unknown"
		scan.Arch.Pattern = "Layered package structure"
		return scan
	}

	// Üst-düzey dizin ve dosya adlarını topla
	var topDirs []string
	fileSet := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() && !ignoredDirs[name] {
			topDirs = append(topDirs, name)
		} else if !e.IsDir() {
			fileSet[strings.ToLower(name)] = true
		}
	}

	scan.Lang = detectLang(projectPath, fileSet, logger)
	scan.Arch = detectArch(topDirs, fileSet, projectPath)
	scan.Standards = detectStandards(projectPath, fileSet, topDirs, scan.Lang)
	scan.Context = detectContext(projectPath, scan.Lang.ModuleName, logger)

	return scan
}

// detectLang go.mod → package.json → Cargo.toml → requirements.txt/pyproject.toml →
// pom.xml/build.gradle → Gemfile önceliğiyle dil tespiti yapar.
func detectLang(projectPath string, fileSet map[string]bool, logger *log.Logger) LangInfo {
	info := LangInfo{Name: "Unknown"}

	switch {
	case fileSet["go.mod"]:
		info = parseGoMod(filepath.Join(projectPath, "go.mod"), logger)
	case fileSet["package.json"]:
		info = parsePackageJSON(filepath.Join(projectPath, "package.json"), logger)
	case fileSet["cargo.toml"]:
		info = parseCargoToml(filepath.Join(projectPath, "cargo.toml"), logger)
	case fileSet["requirements.txt"] || fileSet["pyproject.toml"]:
		info = parsePython(projectPath, fileSet)
	case fileSet["pom.xml"] || fileSet["build.gradle"]:
		info = parseJVM(fileSet)
	case fileSet["gemfile"]:
		info = LangInfo{Name: "Ruby", PkgManager: "bundler", BuildCmd: "bundle exec ruby"}
	}

	return info
}

func parseGoMod(path string, logger *log.Logger) LangInfo {
	info := LangInfo{
		Name:       "Go",
		PkgManager: "go modules",
		BuildCmd:   "go build ./...",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if logger != nil {
			logger.Printf("analyzer: go.mod okunamadı: %v", err)
		}
		return info
	}

	lines := strings.Split(string(data), "\n")
	inRequire := false
	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			info.ModuleName = strings.TrimSpace(strings.TrimPrefix(line, "module "))
		} else if strings.HasPrefix(line, "go ") {
			info.Version = strings.TrimSpace(strings.TrimPrefix(line, "go "))
		} else if line == "require (" {
			inRequire = true
		} else if line == ")" {
			inRequire = false
		} else if inRequire && line != "" && !strings.HasPrefix(line, "//") {
			parts := strings.Fields(line)
			if len(parts) >= 1 && count < 10 {
				dep := parts[0]
				// indirect bağımlılıkları atla
				if !strings.Contains(line, "// indirect") {
					info.KeyDeps = append(info.KeyDeps, dep)
					count++
				}
			}
		} else if strings.HasPrefix(line, "require ") && !strings.Contains(line, "(") {
			// tek satır require
			parts := strings.Fields(line)
			if len(parts) >= 2 && count < 10 {
				info.KeyDeps = append(info.KeyDeps, parts[1])
				count++
			}
		}
	}

	// Framework tespiti
	for _, dep := range info.KeyDeps {
		switch {
		case strings.Contains(dep, "gin-gonic"):
			info.Framework = "Gin"
		case strings.Contains(dep, "fiber"):
			info.Framework = "Fiber"
		case strings.Contains(dep, "echo"):
			info.Framework = "Echo"
		case strings.Contains(dep, "chi"):
			info.Framework = "Chi"
		}
	}

	return info
}

func parsePackageJSON(path string, logger *log.Logger) LangInfo {
	info := LangInfo{
		Name:       "Node.js",
		PkgManager: "npm",
		BuildCmd:   "npm run build",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if logger != nil {
			logger.Printf("analyzer: package.json okunamadı: %v", err)
		}
		return info
	}

	content := string(data)

	// Yarn / pnpm tespiti
	if _, yerr := os.Stat(filepath.Dir(path) + "/yarn.lock"); yerr == nil {
		info.PkgManager = "yarn"
		info.BuildCmd = "yarn build"
	} else if _, perr := os.Stat(filepath.Dir(path) + "/pnpm-lock.yaml"); perr == nil {
		info.PkgManager = "pnpm"
		info.BuildCmd = "pnpm build"
	}

	// Framework tespiti (basit string search)
	frameworks := map[string]string{
		`"next"`:    "Next.js",
		`"react"`:   "React",
		`"vue"`:     "Vue",
		`"svelte"`:  "Svelte",
		`"express"`: "Express",
		`"fastify"`: "Fastify",
		`"nestjs"`:  "NestJS",
	}
	for key, name := range frameworks {
		if strings.Contains(content, key) {
			info.Framework = name
			break
		}
	}

	// Bağımlılıkları satır satır çek (dependencies bloğu)
	count := 0
	inDeps := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, `"dependencies"`) || strings.Contains(trimmed, `"devDependencies"`) {
			inDeps = true
			continue
		}
		if inDeps && trimmed == "}" {
			inDeps = false
			continue
		}
		if inDeps && count < 10 {
			// "package-name": "version" satırı
			if idx := strings.Index(trimmed, `"`); idx == 0 {
				end := strings.Index(trimmed[1:], `"`)
				if end > 0 {
					dep := trimmed[1 : end+1]
					if dep != "" && dep != "dependencies" && dep != "devDependencies" {
						info.KeyDeps = append(info.KeyDeps, dep)
						count++
					}
				}
			}
		}
	}

	return info
}

func parseCargoToml(path string, logger *log.Logger) LangInfo {
	info := LangInfo{
		Name:       "Rust",
		PkgManager: "cargo",
		BuildCmd:   "cargo build",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if logger != nil {
			logger.Printf("analyzer: Cargo.toml okunamadı: %v", err)
		}
		return info
	}

	inDeps := false
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name = ") {
			val := strings.Trim(strings.TrimPrefix(line, "name = "), `"`)
			info.ModuleName = val
		} else if line == "[dependencies]" {
			inDeps = true
		} else if strings.HasPrefix(line, "[") && line != "[dependencies]" {
			inDeps = false
		} else if inDeps && line != "" && count < 10 {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) >= 1 {
				info.KeyDeps = append(info.KeyDeps, strings.TrimSpace(parts[0]))
				count++
			}
		}
	}

	return info
}

func parsePython(projectPath string, fileSet map[string]bool) LangInfo {
	info := LangInfo{
		Name:     "Python",
		BuildCmd: "python -m build",
	}

	if fileSet["pyproject.toml"] {
		info.PkgManager = "pip/poetry"
	} else {
		info.PkgManager = "pip"
	}

	// requirements.txt'ten bağımlılıkları oku
	if fileSet["requirements.txt"] {
		data, err := os.ReadFile(filepath.Join(projectPath, "requirements.txt"))
		if err == nil {
			count := 0
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				// Paket adını versiyon kısıtından ayır
				parts := strings.FieldsFunc(line, func(r rune) bool {
					return r == '>' || r == '<' || r == '=' || r == '!'
				})
				if len(parts) > 0 && count < 10 {
					dep := strings.TrimSpace(parts[0])
					if dep != "" {
						info.KeyDeps = append(info.KeyDeps, dep)
						count++
					}
				}
			}
		}
	}

	// Framework tespiti
	for _, dep := range info.KeyDeps {
		switch strings.ToLower(dep) {
		case "django":
			info.Framework = "Django"
		case "flask":
			info.Framework = "Flask"
		case "fastapi":
			info.Framework = "FastAPI"
		}
	}

	return info
}

func parseJVM(fileSet map[string]bool) LangInfo {
	if fileSet["pom.xml"] {
		return LangInfo{
			Name:       "Java",
			PkgManager: "maven",
			BuildCmd:   "mvn package",
		}
	}
	return LangInfo{
		Name:       "Java/Kotlin",
		PkgManager: "gradle",
		BuildCmd:   "gradle build",
	}
}

// detectArch üst-düzey dizin adlarından mimari pattern'i belirler.
func detectArch(topDirs []string, fileSet map[string]bool, projectPath string) ArchInfo {
	dirSet := map[string]bool{}
	for _, d := range topDirs {
		dirSet[strings.ToLower(d)] = true
	}

	var pattern string
	switch {
	case dirSet["cmd"] && (dirSet["internal"] || dirSet["pkg"]):
		pattern = "Go standard layout"
	case dirSet["controllers"] && dirSet["models"] && dirSet["views"]:
		pattern = "MVC"
	case dirSet["domain"] && (dirSet["usecase"] || dirSet["usecases"]) && (dirSet["repository"] || dirSet["repositories"]):
		pattern = "Clean Architecture"
	case dirSet["components"] && dirSet["pages"]:
		pattern = "Frontend SPA"
	default:
		pattern = "Layered package structure"
	}

	// İletişim tipi tespiti
	commType := ""
	hasHTTP := dirSet["api"] || dirSet["http"] || dirSet["rest"] || dirSet["web"] || dirSet["handler"] || dirSet["handlers"]
	hasGRPC := dirSet["grpc"] || dirSet["proto"] || dirSet["protobuf"]
	hasStdio := fileSet["main.go"] // go projelerinde stdio/mcp ihtimali

	switch {
	case hasGRPC && hasHTTP:
		commType = "gRPC + HTTP"
	case hasGRPC:
		commType = "gRPC"
	case hasHTTP:
		commType = "HTTP"
	case hasStdio && !hasHTTP:
		// stdio olasılığını kontrol et
		commType = detectGoCommType(projectPath)
	}

	// Üst-düzey paket adlarını max 20 ile sınırla
	packages := topDirs
	if len(packages) > 20 {
		packages = packages[:20]
	}

	return ArchInfo{
		Pattern:  pattern,
		Packages: packages,
		CommType: commType,
	}
}

// detectGoCommType go dosyalarında stdio/http kullanımını tespit eder.
func detectGoCommType(projectPath string) string {
	mainPath := filepath.Join(projectPath, "main.go")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		return ""
	}
	content := string(data)
	hasHTTP := strings.Contains(content, "http.Listen") || strings.Contains(content, "net/http")
	hasStdio := strings.Contains(content, "os.Stdin") || strings.Contains(content, "stdio") ||
		strings.Contains(content, "StdioTransport") || strings.Contains(content, "NewStdioServer")
	switch {
	case hasHTTP && hasStdio:
		return "stdio + HTTP"
	case hasHTTP:
		return "HTTP"
	case hasStdio:
		return "stdio"
	}
	return ""
}

// detectStandards linter/formatter/CI/Docker/Makefile varlığını tespit eder.
func detectStandards(projectPath string, fileSet map[string]bool, topDirs []string, lang LangInfo) StandardsInfo {
	info := StandardsInfo{}

	// Linter
	switch {
	case fileSet[".golangci.yml"] || fileSet[".golangci.yaml"] || fileSet[".golangci.toml"]:
		info.Linter = "golangci-lint"
	case fileSet[".eslintrc.js"] || fileSet[".eslintrc.json"] || fileSet[".eslintrc.yml"] ||
		fileSet[".eslintrc.yaml"] || fileSet[".eslintrc"]:
		info.Linter = "ESLint"
	case fileSet[".flake8"] || fileSet["setup.cfg"] || fileSet[".pylintrc"]:
		info.Linter = "flake8/pylint"
	case fileSet["clippy.toml"] || fileSet[".clippy.toml"]:
		info.Linter = "Clippy"
	}

	// Formatter
	switch {
	case fileSet[".prettierrc"] || fileSet[".prettierrc.js"] || fileSet[".prettierrc.json"] ||
		fileSet[".prettierrc.yml"] || fileSet[".prettierrc.yaml"]:
		info.Formatter = "Prettier"
	case lang.Name == "Go":
		info.Formatter = "gofmt"
	case fileSet[".editorconfig"]:
		info.Formatter = "EditorConfig"
	}

	// CI
	for _, d := range topDirs {
		if d == ".github" {
			// .github/workflows/ kontrol et
			wfPath := filepath.Join(projectPath, ".github", "workflows")
			entries, err := os.ReadDir(wfPath)
			if err == nil && len(entries) > 0 {
				info.HasCI = true
			}
			break
		}
	}
	if fileSet[".travis.yml"] || fileSet["circle.yml"] || fileSet[".circleci"] {
		info.HasCI = true
	}

	// Docker
	info.HasDocker = fileSet["dockerfile"] || fileSet["docker-compose.yml"] || fileSet["docker-compose.yaml"]

	// Makefile
	info.HasMake = fileSet["makefile"]

	// Test komutu
	switch lang.Name {
	case "Go":
		info.TestCmd = "go test ./..."
	case "Node.js":
		info.TestCmd = "npm test"
	case "Python":
		info.TestCmd = "pytest"
	case "Rust":
		info.TestCmd = "cargo test"
	case "Java", "Java/Kotlin":
		if lang.PkgManager == "maven" {
			info.TestCmd = "mvn test"
		} else {
			info.TestCmd = "gradle test"
		}
	case "Ruby":
		info.TestCmd = "bundle exec rspec"
	}

	return info
}

// detectContext README'den ilk paragrafı çeker.
func detectContext(projectPath, moduleName string, logger *log.Logger) ContextInfo {
	info := ContextInfo{ModuleName: moduleName}

	// README dosyası bul
	candidates := []string{"README.md", "readme.md", "Readme.md", "README.txt", "README"}
	var readmePath string
	for _, name := range candidates {
		p := filepath.Join(projectPath, name)
		if _, err := os.Stat(p); err == nil {
			readmePath = p
			break
		}
	}

	if readmePath == "" {
		info.Description = moduleName
		return info
	}

	f, err := os.Open(readmePath)
	if err != nil {
		if logger != nil {
			logger.Printf("analyzer: README açılamadı: %v", err)
		}
		info.Description = moduleName
		return info
	}
	defer f.Close()

	// İlk boş olmayan paragrafı oku, max 500 karakter
	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)
	started := false
	for scanner.Scan() {
		line := scanner.Text()
		// Markdown başlıklarını atla
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.TrimSpace(line) == "" {
			if started {
				break // ilk paragraf bitti
			}
			continue
		}
		started = true
		if sb.Len() > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(strings.TrimSpace(line))
		if sb.Len() >= 500 {
			break
		}
	}

	desc := sb.String()
	if len(desc) > 500 {
		desc = desc[:500]
	}
	if desc == "" {
		desc = moduleName
	}
	info.Description = desc

	return info
}
