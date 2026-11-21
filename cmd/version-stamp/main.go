package main

import (
	"log"
	"time"

	"github.com/honahuku/feed-subscriber/internal/buildversion"
)

func main() {
	if err := buildversion.Stamp("cmd/feed-subscriber/VERSION", time.Now()); err != nil {
		log.Fatal(err)
	}
}
