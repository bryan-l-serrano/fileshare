package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
)

func main() {
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
