package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"imgserver/internal/api"
	"imgserver/internal/config"
	"imgserver/internal/storage"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	var cfg *config.Config
	if _, err := os.Stat(*cfgPath); err == nil {
		var err error
		cfg, err = config.Load(*cfgPath)
		if err != nil {
			log.Fatalf("load config: %v", err)
		}
	} else {
		cfg = config.Default()
		log.Printf("config %s not found, using defaults", *cfgPath)
	}

	if err := os.MkdirAll(cfg.StorageDir, 0o755); err != nil {
		log.Fatalf("mkdir storage: %v", err)
	}

	st := storage.New(cfg.StorageDir, cfg.BaseURL)
	log.Printf("imgserver listening on %s, storage=%s baseURL=%s", cfg.Listen, cfg.StorageDir, cfg.BaseURL)
	if err := http.ListenAndServe(cfg.Listen, api.New(cfg, st)); err != nil {
		log.Fatal(err)
	}
}
