package tools

import (
	"context"
	"encoding/json"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"ohakan-mcp/aicontext"
	"ohakan-mcp/planner"
)

type analyzeResponse struct {
	ContextSummary string             `json:"context_summary"`
	MissingContext []string           `json:"missing_context"`
	Questions      []questionResponse `json:"questions"`
	Instruction    string             `json:"instruction"`
}

type questionResponse struct {
	Category string `json:"category"`
	Text     string `json:"text"`
	Why      string `json:"why"`
}

func registerAnalyzeTool(s *mcpserver.MCPServer, logger *log.Logger) {
	tool := mcp.NewTool("analyze_task",
		mcp.WithDescription("Görevi analiz eder, proje bağlamını okur ve çağıran AI'ın kullanıcıya sorması gereken soruları döner."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Yapılacak görevin açıklaması"),
		),
		mcp.WithString("project_path",
			mcp.Required(),
			mcp.Description("Projenin kök dizin yolu"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		prompt, err := req.RequireString("prompt")
		if err != nil {
			return nil, err
		}
		projectPath, err := req.RequireString("project_path")
		if err != nil {
			return nil, err
		}

		logger.Printf("analyze_task çağrıldı: prompt=%q project=%q", prompt, projectPath)

		projCtx, _ := aicontext.Load(projectPath)
		questions := planner.GenerateQuestions(prompt, projCtx)

		qResponses := make([]questionResponse, 0, len(questions))
		for _, q := range questions {
			qResponses = append(qResponses, questionResponse{
				Category: q.Category,
				Text:     q.Text,
				Why:      q.Why,
			})
		}

		resp := analyzeResponse{
			ContextSummary: projCtx.Summary(),
			MissingContext: projCtx.MissingFiles,
			Questions:      qResponses,
			Instruction:    "Lütfen bu soruları kullanıcıya sorun, ardından create_plan'ı çağırın.",
		}

		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return nil, err
		}

		return mcp.NewToolResultText(string(data)), nil
	})
}
