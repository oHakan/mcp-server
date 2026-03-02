package tools

import (
	"log"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Register tüm tool'ları MCP sunucusuna kaydeder.
// Yeni bir tool eklemek için buraya bir satır ekleyin.
func Register(s *mcpserver.MCPServer, logger *log.Logger) {
	registerAnalyzeTool(s, logger)
	registerPlanTool(s, logger)
}
