package server

import (
	"fmt"
	"log"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"ohakan-mcp/tools"
)

// Server MCP sunucusunu ve bağımlılıklarını tutar.
type Server struct {
	logger *log.Logger
	mcp    *mcpserver.MCPServer
}

// New sunucuyu oluşturur ve tüm tool'ları kaydeder.
func New(logger *log.Logger) *Server {
	s := &Server{
		logger: logger,
		mcp:    mcpserver.NewMCPServer("HakoOrchestrator", "1.0.0"),
	}
	tools.Register(s.mcp, logger)
	return s
}

// Run sunucuyu stdio üzerinden başlatır (Claude Code / Cursor bağlantısı için).
func (s *Server) Run() error {
	s.logger.Println("HakoOrchestrator MCP Sunucusu stdio üzerinden başlatılıyor...")
	return mcpserver.ServeStdio(s.mcp)
}

// RunHTTP sunucuyu Streamable HTTP üzerinden başlatır.
// addr örneği: ":8080"  — endpoint: POST/GET http://localhost:8080/mcp
func (s *Server) RunHTTP(addr string) error {
	s.logger.Printf("HakoOrchestrator MCP Sunucusu HTTP üzerinden başlatılıyor: http://localhost%s/mcp", addr)
	httpServer := mcpserver.NewStreamableHTTPServer(s.mcp)
	fmt.Printf("MCP endpoint: http://localhost%s/mcp\n", addr)
	return httpServer.Start(addr)
}
