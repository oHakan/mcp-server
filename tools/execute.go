package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"ohakan-mcp/aicontext"
	"ohakan-mcp/executor"
	"ohakan-mcp/planner"
)

type executeResponse struct {
	PlanTitle   string                    `json:"plan_title"`
	PlanPath    string                    `json:"plan_path,omitempty"`
	TotalTasks  int                       `json:"total_tasks"`
	TotalBatch  int                       `json:"total_batches"`
	Batches     []executor.ExecutionBatch `json:"batches"`
	Instruction string                    `json:"instruction"`
}

func registerExecuteTool(s *mcpserver.MCPServer, logger *log.Logger) {
	tool := mcp.NewTool("execute_plan",
		mcp.WithDescription("Kaydedilmiş bir planı okur, DAG'ı paralel yürütme batch'lerine böler ve her görev için hazır sub-agent prompt'ları üretir."),
		mcp.WithString("project_path",
			mcp.Required(),
			mcp.Description("Projenin kök dizin yolu"),
		),
		mcp.WithString("plan_path",
			mcp.Description("docs/ai/plans/ altındaki plan dosyasının göreli yolu (plan_path veya dag_json gerekli)"),
		),
		mcp.WithString("dag_json",
			mcp.Description("create_plan'dan dönen DAG JSON'ı (plan_path yoksa kullanılır)"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectPath, err := req.RequireString("project_path")
		if err != nil {
			return nil, err
		}

		planPath := req.GetString("plan_path", "")
		dagJSON := req.GetString("dag_json", "")

		if planPath == "" && dagJSON == "" {
			return nil, fmt.Errorf("plan_path veya dag_json parametrelerinden biri gerekli")
		}

		logger.Printf("execute_plan çağrıldı: project=%q plan_path=%q", projectPath, planPath)

		// DAG'ı al
		dag, planTitle, err := loadDAG(projectPath, planPath, dagJSON)
		if err != nil {
			return nil, err
		}

		// Batch'lere böl
		batches, err := executor.BatchDAG(dag)
		if err != nil {
			return nil, fmt.Errorf("DAG batch'lere bölünemedi: %w", err)
		}

		// Proje context'ini yükle (sub-agent prompt'ları için)
		projCtx := aicontext.Load(projectPath, logger)

		// State dosyasını başlat (plan_path varsa)
		if planPath != "" {
			if _, serr := executor.InitState(projectPath, planPath, planTitle, batches); serr != nil {
				logger.Printf("execute_plan: state başlatılamadı: %v", serr)
			}
		}

		// Her task için sub-agent prompt üret (plan_path rapor için gerekli)
		executor.GeneratePrompts(batches, projectPath, planPath, projCtx)

		// Talimat metni üret
		instruction := executor.BuildInstruction(batches, planTitle)

		// Toplam görev sayısı
		total := 0
		for _, b := range batches {
			total += len(b.Tasks)
		}

		resp := executeResponse{
			PlanTitle:   planTitle,
			PlanPath:    planPath,
			TotalTasks:  total,
			TotalBatch:  len(batches),
			Batches:     batches,
			Instruction: instruction,
		}

		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return nil, err
		}

		return mcp.NewToolResultText(string(data)), nil
	})
}

// loadDAG plan_path veya dag_json'dan bir DAG yükler.
func loadDAG(projectPath, planPath, dagJSON string) (*planner.DAG, string, error) {
	if planPath != "" {
		return loadDAGFromFile(projectPath, planPath)
	}
	return loadDAGFromJSON(dagJSON)
}

// loadDAGFromFile plan MD dosyasından DAG JSON bloğunu çıkarır ve parse eder.
func loadDAGFromFile(projectPath, planPath string) (*planner.DAG, string, error) {
	fullPath := filepath.Join(projectPath, planPath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		// Mutlak yol olarak dene
		data, err = os.ReadFile(planPath)
		if err != nil {
			return nil, "", fmt.Errorf("plan dosyası okunamadı: %w", err)
		}
	}

	content := string(data)

	// Plan başlığını çek
	planTitle := extractPlanTitle(content)

	// ```json ... ``` bloğunu çek
	jsonStr := extractJSONBlock(content)
	if jsonStr == "" {
		return nil, planTitle, fmt.Errorf("plan dosyasında DAG JSON bloğu bulunamadı")
	}

	dag, err := parseDAGJSON(jsonStr)
	if err != nil {
		return nil, planTitle, err
	}

	return dag, planTitle, nil
}

// loadDAGFromJSON doğrudan JSON string'inden DAG parse eder.
func loadDAGFromJSON(dagJSON string) (*planner.DAG, string, error) {
	dag, err := parseDAGJSON(dagJSON)
	if err != nil {
		return nil, "", err
	}
	return dag, "Plan", nil
}

func parseDAGJSON(jsonStr string) (*planner.DAG, error) {
	var dag planner.DAG
	if err := json.Unmarshal([]byte(jsonStr), &dag); err != nil {
		return nil, fmt.Errorf("DAG JSON parse hatası: %w", err)
	}
	return &dag, nil
}

// extractPlanTitle MD dosyasından ilk # başlığını alır.
func extractPlanTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return "Plan"
}

var jsonBlockRe = regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```")

// extractJSONBlock MD içindeki ilk ```json ... ``` bloğunu döner.
func extractJSONBlock(content string) string {
	m := jsonBlockRe.FindStringSubmatch(content)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
