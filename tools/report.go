package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"ohakan-mcp/executor"
)

type reportTaskResponse struct {
	TaskID         string   `json:"task_id"`
	TaskStatus     string   `json:"task_status"`
	PlanStatus     string   `json:"plan_status"`
	CompletedTasks int      `json:"completed_tasks"`
	TotalTasks     int      `json:"total_tasks"`
	ReportPath     string   `json:"report_path,omitempty"`
	Message        string   `json:"message"`
}

func registerReportTool(s *mcpserver.MCPServer, logger *log.Logger) {
	tool := mcp.NewTool("report_task",
		mcp.WithDescription("Sub-agent bir görevi tamamladığında çağırır. Task durumunu kaydeder; tüm görevler bitince planı tamamlandı olarak işaretler ve rapor oluşturur."),
		mcp.WithString("project_path",
			mcp.Required(),
			mcp.Description("Projenin kök dizin yolu"),
		),
		mcp.WithString("plan_path",
			mcp.Required(),
			mcp.Description("Plan dosyasının göreli yolu (örn: docs/ai/plans/2026-03-02-foo.md)"),
		),
		mcp.WithString("task_id",
			mcp.Required(),
			mcp.Description("Tamamlanan task'ın ID'si (örn: t3)"),
		),
		mcp.WithString("status",
			mcp.Required(),
			mcp.Description("Task sonucu: 'done' veya 'failed'"),
		),
		mcp.WithString("summary",
			mcp.Required(),
			mcp.Description("Task sırasında yapılanların kısa özeti"),
		),
		mcp.WithString("changed_files",
			mcp.Description("Değiştirilen/oluşturulan dosyalar, virgülle ayrılmış (boş bırakılabilir)"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectPath, err := req.RequireString("project_path")
		if err != nil {
			return nil, err
		}
		planPath, err := req.RequireString("plan_path")
		if err != nil {
			return nil, err
		}
		taskID, err := req.RequireString("task_id")
		if err != nil {
			return nil, err
		}
		statusStr, err := req.RequireString("status")
		if err != nil {
			return nil, err
		}
		summary, err := req.RequireString("summary")
		if err != nil {
			return nil, err
		}
		changedFilesStr := req.GetString("changed_files", "")

		logger.Printf("report_task çağrıldı: task=%q status=%q plan=%q", taskID, statusStr, planPath)

		// Status parse
		taskStatus, err := parseTaskStatus(statusStr)
		if err != nil {
			return nil, err
		}

		// changed_files parse (virgülle ayrılmış)
		var changedFiles []string
		for _, f := range strings.Split(changedFilesStr, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				changedFiles = append(changedFiles, f)
			}
		}

		// State'i güncelle
		state, planDone, err := executor.UpdateTask(projectPath, planPath, taskID, taskStatus, summary, changedFiles)
		if err != nil {
			return nil, fmt.Errorf("task güncellenemedi: %w", err)
		}

		resp := reportTaskResponse{
			TaskID:         taskID,
			TaskStatus:     string(taskStatus),
			PlanStatus:     string(state.Status),
			CompletedTasks: state.DoneCount(),
			TotalTasks:     state.TotalTasks,
		}

		if planDone {
			// Plan MD'deki status satırını güncelle
			if merr := executor.MarkPlanCompleted(projectPath, planPath, state.Status); merr != nil {
				logger.Printf("plan MD güncellenemedi: %v", merr)
			}

			// Rapor oluştur
			// Batch bilgisi state'te yok; reporter nil batches ile çalışır
			reportPath, rerr := executor.GenerateReport(projectPath, state, nil)
			if rerr != nil {
				logger.Printf("rapor oluşturulamadı: %v", rerr)
			} else {
				resp.ReportPath = reportPath
				logger.Printf("plan tamamlandı, rapor: %s", reportPath)
			}

			if state.Status == executor.PlanCompleted {
				resp.Message = fmt.Sprintf("Plan tamamlandı ✓ (%d/%d görev). Rapor: %s",
					state.DoneCount(), state.TotalTasks, resp.ReportPath)
			} else {
				resp.Message = fmt.Sprintf("Plan başarısız ✗ (%d/%d görev tamamlandı). Rapor: %s",
					state.DoneCount(), state.TotalTasks, resp.ReportPath)
			}
		} else {
			resp.Message = fmt.Sprintf("Task %s kaydedildi (%d/%d tamamlandı)",
				taskID, state.DoneCount(), state.TotalTasks)
		}

		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return nil, err
		}

		return mcp.NewToolResultText(string(data)), nil
	})
}

func parseTaskStatus(s string) (executor.TaskStatus, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "done":
		return executor.TaskDone, nil
	case "failed":
		return executor.TaskFailed, nil
	default:
		return "", fmt.Errorf("geçersiz status %q — 'done' veya 'failed' olmalı", s)
	}
}
