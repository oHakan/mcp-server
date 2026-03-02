package main

import (
	"flag"
	"log"
	"os"

	"ohakan-mcp/server"
)

func main() {
	addr := flag.String("addr", "", "HTTP dinleme adresi (örn: :8080). Boş bırakılırsa stdio modu kullanılır.")
	flag.Parse()

	// Tüm loglar stderr'e gider — stdout MCP protokolüne ayrılmış.
	logger := log.New(os.Stderr, "[ohakan-mcp] ", log.LstdFlags)

	s := server.New(logger)

	var err error
	if *addr != "" {
		err = s.RunHTTP(*addr)
	} else {
		err = s.Run()
	}

	if err != nil {
		logger.Fatalf("Sunucu hatası: %v", err)
	}
}
