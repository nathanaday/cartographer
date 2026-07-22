package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/nathanaday/cartographer/internal/store"
)

func main() {
	addr := flag.String("addr", ":8099", "listen address")
	flag.Parse()

	cfg, err := store.Resolve()
	if err != nil {
		log.Fatal(err)
	}
	srv := newServer(cfg)
	log.Printf("cartographer web: store=%s listening on %s", cfg.Dir, *addr)
	if err := http.ListenAndServe(*addr, srv.routes()); err != nil {
		log.Fatal(err)
	}
}
