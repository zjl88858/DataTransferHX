package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"filetransferhx/config"
	"filetransferhx/core"
)

func main() {
	configPath := flag.String("config", "config.toml", "Path to config file")
	historyPath := flag.String("history", "history.json", "Path to history file")
	flag.Parse()

	// 1. Load Config
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Init History
	hm := core.NewHistoryManager(*historyPath)
	if err := hm.Load(); err != nil {
		log.Printf("Warning: Failed to load history: %v", err)
	}

	// 3. Init Transfer Manager
	tm := core.NewTransferManager(hm)

	// 4. Init Runner
	runner := core.NewRunner(cfg, tm)
	runner.Start()

	log.Println("FileTransferHX started...")

	// 5. Wait for signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	runner.Stop()
	hm.Save()
}
