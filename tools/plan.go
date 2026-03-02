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

type planResponse struct {
	Status    string          `json:"status"`
	SavedPath string          `json:"saved_path"`
	Plan      string          `json:"plan"`
	DAG       json.RawMessage `json:"dag"`
}

func registerPlanTool(s *mcpserver.MCPServer, logger *log.Logger) {
	tool := mcp.NewTool("create_plan",
		mcp.WithDescription("Görev açıklaması ve kullanıcı cevapları alarak DAG tabanlı yürütme planı oluşturur ve docs/ai/plans/ altına kaydeder."),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("Orijinal görev açıklaması"),
		),
		mcp.WithString("answers",
			mcp.Required(),
			mcp.Description("Kullanıcının analyze_task sorularına verdiği cevaplar"),
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
		answers, err := req.RequireString("answers")
		if err != nil {
			return nil, err
		}
		projectPath, err := req.RequireString("project_path")
		if err != nil {
			return nil, err
		}

		logger.Printf("create_plan çağrıldı: prompt=%q project=%q", prompt, projectPath)

		projCtx, _ := aicontext.Load(projectPath)
		output, err := planner.Generate(planner.PlanInput{
			Prompt:      prompt,
			Answers:     answers,
			Context:     projCtx,
			ProjectPath: projectPath,
		})
		if err != nil {
			return nil, err
		}

		resp := planResponse{
			Status:    "success",
			SavedPath: output.SavedPath,
			Plan:      output.PlanMarkdown,
			DAG:       json.RawMessage(output.DAGJSON),
		}

		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return nil, err
		}

		return mcp.NewToolResultText(string(data)), nil
	})
}
