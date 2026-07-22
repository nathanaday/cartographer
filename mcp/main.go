package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/cartographer/internal/mcptools"
	"github.com/nathanaday/cartographer/internal/store"
)

func main() {
	cfg, err := store.Resolve()
	if err != nil {
		log.Fatal(err)
	}
	st, err := store.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "cartographer", Version: "v0.1.0"}, nil)
	mcptools.Register(server, st)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
