// TallyMCP — AI-Native MCP Bridge for TallyPrime
//
// A Go-based Model Context Protocol (MCP) server that bridges AI assistants
// directly to TallyPrime's local XML API on port 9000.
//
// Usage:
//
//	tallymcp.exe                          # Runs with defaults (localhost:9000)
//	TALLY_PORT=9001 tallymcp.exe          # Custom Tally port
//	TALLYMCP_LOG_LEVEL=debug tallymcp.exe # Verbose logging
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/omesh7/tally-mcp/internal/config"
	"github.com/omesh7/tally-mcp/internal/tally"
	"github.com/omesh7/tally-mcp/internal/tools"
)

func main() {
	// All logging goes to stderr — stdout is reserved for MCP JSON-RPC
	log.SetOutput(os.Stderr)
	log.SetPrefix("[TallyMCP] ")

	// Load configuration
	cfg := config.LoadFromEnv()
	log.Printf("Starting TallyMCP Go Server v1.0.0")
	log.Printf("Tally target: %s", cfg.TallyURL())

	// Create Tally HTTP client with 15s timeout
	client := tally.NewClient(cfg.TallyHost, cfg.TallyPort, 15*time.Second)

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "tallymcp-go",
		Version: "1.0.0",
	}, nil)

	// Register all tools
	tools.RegisterAll(server, client)

	log.Printf("13 MCP tools registered. Waiting for AI client connections on stdio...")

	var transport mcp.Transport = &mcp.StdioTransport{}
	if cfg.LogLevel == "debug" {
		transport = &mcp.LoggingTransport{
			Transport: transport,
			Writer:    os.Stderr,
		}
		log.Printf("Debug logging enabled for JSON-RPC transport")
	}

	// Run MCP server over transport (blocks until client disconnects)
	if err := server.Run(context.Background(), transport); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}
