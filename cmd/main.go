package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anomalyco/branch-pol-mcp/internal/mcp"
)

func main() {
	server := mcp.NewServer()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		os.Exit(0)
	}()

	if err := server.Run(); err != nil {
		log.Fatal(err)
	}

	select {}
}
