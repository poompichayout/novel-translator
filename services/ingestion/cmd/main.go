package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/poompich/novel-translator/services/ingestion/internal/config"
	"github.com/poompich/novel-translator/services/ingestion/internal/repository"
	"github.com/poompich/novel-translator/services/ingestion/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ingestion-service <command>")
		fmt.Println("Commands: serve, translate")
		os.Exit(1)
	}

	cmd := os.Args[1]

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "../../config.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("Warning: failed to load config (%v); using defaults.", err)
		cfg = &config.Config{}
		if envDB := os.Getenv("DB_URL"); envDB != "" {
			cfg.Database.URL = envDB
		} else {
			cfg.Database.URL = "postgres://translator:password123@localhost:5432/novel_translator?sslmode=disable"
		}
		cfg.Server.Addr = ":8080"
		if pwd := os.Getenv("SERVER_PASSWORD"); pwd != "" {
			cfg.Server.Password = pwd
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	repo, err := repository.NewPostgresRepo(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("DB error: %v", err)
	}
	defer repo.Close()

	switch cmd {
	case "serve":
		if cfg.Server.Password == "" {
			log.Fatal("server.password not set in config or SERVER_PASSWORD env")
		}
		h := &server.Handlers{Repo: repo}
		if err := server.Run(ctx, h, cfg.Server.Addr, cfg.Server.Password); err != nil {
			log.Fatalf("serve: %v", err)
		}
	case "translate":
		log.Println("translate: translation worker not implemented yet (Phase 1, Week 3)")
		os.Exit(2)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}
