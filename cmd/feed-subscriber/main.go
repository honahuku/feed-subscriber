package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/honahuku/feed-subscriber/internal/app"
)

//go:embed VERSION
var version string

func main() {
	showVersion := flag.Bool("version", false, "print version")
	configPath := flag.String("config", "config.yaml", "path to the feed configuration")
	secretsPath := flag.String("secrets", "secrets.yaml", "path to secret values")
	statePath := flag.String("state", "state.json", "path to persistent feed state")
	flag.Parse()

	if *showVersion {
		fmt.Printf("feed-subscriber %s\n", strings.TrimSpace(version))
		return
	}

	config, err := app.LoadConfig(*configPath, *secretsPath)
	if err != nil {
		log.Fatal(err)
	}
	state, err := app.LoadState(*statePath)
	if err != nil {
		log.Fatal(err)
	}
	if config.MigrateState(state) {
		if err := state.Save(*statePath); err != nil {
			log.Fatal(err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.New(config, state, *statePath).Run(ctx); err != nil {
		log.Fatal(err)
	}
}
