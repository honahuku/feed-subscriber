package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/honahuku/feed-subscriber/internal/app"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to the feed configuration")
	secretsPath := flag.String("secrets", "secrets.yaml", "path to webhook URL mappings")
	statePath := flag.String("state", "state.json", "path to persistent feed state")
	flag.Parse()

	config, err := app.LoadConfig(*configPath, *secretsPath)
	if err != nil {
		log.Fatal(err)
	}
	state, err := app.LoadState(*statePath)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.New(config, state, *statePath).Run(ctx); err != nil {
		log.Fatal(err)
	}
}
