package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	os.MkdirAll("logs", 0755)
	logFile, err := os.OpenFile("logs/server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("open log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	cfg, err := LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	h := NewHandler(cfg)

	log.Printf("FileShare server starting on %s:%d", cfg.Address, cfg.Port)
	log.Printf("Mount points:")
	for _, mp := range cfg.MountPoints {
		abs, _ := filepath.Abs(mp.Path)
		log.Printf("  %s -> %s (max %d MB)", mp.Label, abs, mp.MaxSizeMB)
	}

	if err := http.ListenAndServe(fmt.Sprintf("%s:%d", cfg.Address, cfg.Port), h); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
