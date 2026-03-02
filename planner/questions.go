package planner

import (
	"strings"

	"ohakan-mcp/aicontext"
)

// Question analiz sırasında çağıran AI'ın kullanıcıya sorması gereken bir soruyu temsil eder.
type Question struct {
	ID       string // "q_db_schema"
	Category string // "Database", "Auth", "API", ...
	Text     string // Sorunun metni
	Why      string // Bu sorunun planı nasıl etkilediği
	Required bool   // Context'te cevap yoksa zorunlu mu?
}

// keywordRule bir keyword kümesi ile eşleşen soruları tanımlar.
type keywordRule struct {
	keywords  []string
	questions []Question
}

var keywordRules = []keywordRule{
	{
		keywords: []string{"db", "database", "sql", "schema", "migration"},
		questions: []Question{
			{
				ID: "q_db_type", Category: "Database",
				Text:     "Hangi veritabanı kullanılıyor? (PostgreSQL, MySQL, SQLite, vb.)",
				Why:      "Plan doğru DB migration adımlarını içersin",
				Required: true,
			},
			{
				ID: "q_db_schema", Category: "Database",
				Text:     "Mevcut bir şema var mı? Varsa hangi tablolar etkilenecek?",
				Why:      "Migration sırası ve bağımlılıkları belirlenir",
				Required: false,
			},
		},
	},
	{
		keywords: []string{"auth", "login", "user", "session", "jwt", "oauth"},
		questions: []Question{
			{
				ID: "q_auth_method", Category: "Auth",
				Text:     "Kimlik doğrulama yöntemi nedir? (JWT, session, OAuth, vb.)",
				Why:      "Doğru auth akışı planlanabilsin",
				Required: true,
			},
			{
				ID: "q_auth_roles", Category: "Auth",
				Text:     "Kullanıcı rolleri/yetkileri var mı? Varsa listeleyin.",
				Why:      "Yetkilendirme katmanı planlanabilsin",
				Required: false,
			},
		},
	},
	{
		keywords: []string{"api", "rest", "endpoint", "route", "http"},
		questions: []Question{
			{
				ID: "q_api_style", Category: "API",
				Text:     "API stili nedir? (REST, GraphQL, gRPC, vb.)",
				Why:      "Endpoint yapısı ve middleware planlanabilsin",
				Required: true,
			},
			{
				ID: "q_api_versioning", Category: "API",
				Text:     "API versiyonlaması gerekiyor mu?",
				Why:      "URL yapısı ve backward-compatibility planlanabilsin",
				Required: false,
			},
		},
	},
	{
		keywords: []string{"ui", "frontend", "page", "component", "form"},
		questions: []Question{
			{
				ID: "q_fe_framework", Category: "Frontend",
				Text:     "Frontend framework nedir? (React, Vue, Next.js, vb.)",
				Why:      "Bileşen yapısı ve state yönetimi planlanabilsin",
				Required: true,
			},
			{
				ID: "q_fe_styling", Category: "Frontend",
				Text:     "Stil sistemi nedir? (Tailwind, CSS Modules, styled-components, vb.)",
				Why:      "UI bileşenleri tutarlı yazılsın",
				Required: false,
			},
		},
	},
	{
		keywords: []string{"test", "spec", "coverage"},
		questions: []Question{
			{
				ID: "q_test_framework", Category: "Testing",
				Text:     "Test framework nedir? (Jest, Vitest, Go test, pytest, vb.)",
				Why:      "Test dosyaları doğru formatta yazılsın",
				Required: true,
			},
			{
				ID: "q_test_coverage", Category: "Testing",
				Text:     "Hedef test coverage yüzdesi nedir?",
				Why:      "Hangi modüllerin test gerektirdiği belirlenir",
				Required: false,
			},
		},
	},
	{
		keywords: []string{"performance", "cache", "scale", "load"},
		questions: []Question{
			{
				ID: "q_perf_bottleneck", Category: "Performance",
				Text:     "Mevcut performans darboğazı nerede tespit edildi?",
				Why:      "Doğru optimizasyon stratejisi seçilsin",
				Required: true,
			},
			{
				ID: "q_perf_cache", Category: "Performance",
				Text:     "Cache katmanı var mı? (Redis, in-memory, CDN)",
				Why:      "Cache invalidation stratejisi planlanabilsin",
				Required: false,
			},
		},
	},
	{
		keywords: []string{"security", "encrypt", "permission", "role"},
		questions: []Question{
			{
				ID: "q_sec_threat", Category: "Security",
				Text:     "Hangi güvenlik tehditleri ele alınacak? (XSS, CSRF, SQLi, vb.)",
				Why:      "Doğru güvenlik önlemleri planlanabilsin",
				Required: true,
			},
			{
				ID: "q_sec_audit", Category: "Security",
				Text:     "Güvenlik audit/log gereksinimi var mı?",
				Why:      "Audit trail planlanabilsin",
				Required: false,
			},
		},
	},
}

// universalQuestions her prompt için her zaman döner.
var universalQuestions = []Question{
	{
		ID: "q_scope", Category: "Evrensel",
		Text:     "Bu görevin kapsamı nedir? Hangi modüller/dosyalar etkilenecek?",
		Why:      "Görev sınırları netleşsin, kapsam kayması önlensin",
		Required: true,
	},
	{
		ID: "q_success", Category: "Evrensel",
		Text:     "Başarı kriterleri nedir? Görev ne zaman tamamlandı sayılır?",
		Why:      "Plan doğrulanabilir çıktılar içersin",
		Required: true,
	},
}

// GenerateQuestions prompt keyword analizi ve eksik context dosyalarına göre soru listesi üretir.
func GenerateQuestions(prompt string, ctx *aicontext.ProjectContext) []Question {
	questions := make([]Question, 0)
	questions = append(questions, universalQuestions...)

	lower := strings.ToLower(prompt)

	seen := map[string]bool{}
	for _, rule := range keywordRules {
		matched := false
		for _, kw := range rule.keywords {
			if strings.Contains(lower, kw) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		for _, q := range rule.questions {
			if !seen[q.ID] {
				seen[q.ID] = true
				questions = append(questions, q)
			}
		}
	}

	// Eksik context dosyaları için otomatik sorular
	for _, missing := range ctx.MissingFiles {
		switch missing {
		case "architecture.md":
			questions = append(questions, Question{
				ID:       "q_missing_arch",
				Category: "Context",
				Text:     "architecture.md bulunamadı — projenin mimarisi nedir? (monolith, microservices, vb.)",
				Why:      "Mimari kararlar plana yansıtılabilsin",
				Required: true,
			})
		case "stack.md":
			questions = append(questions, Question{
				ID:       "q_missing_stack",
				Category: "Context",
				Text:     "stack.md bulunamadı — hangi teknoloji yığını kullanılıyor?",
				Why:      "Teknolojiye uygun implementasyon planlanabilsin",
				Required: true,
			})
		case "standards.md":
			questions = append(questions, Question{
				ID:       "q_missing_standards",
				Category: "Context",
				Text:     "standards.md bulunamadı — kodlama standartları ve kurallar nedir?",
				Why:      "Plan standartlara uygun adımlar içersin",
				Required: false,
			})
		case "context.md":
			questions = append(questions, Question{
				ID:       "q_missing_context",
				Category: "Context",
				Text:     "context.md bulunamadı — projenin genel bağlamı ve amacı nedir?",
				Why:      "Görev bağlamı doğru anlaşılabilsin",
				Required: true,
			})
		}
	}

	return questions
}
