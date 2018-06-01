package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"

	"github.com/ericyan/doh"
)

var (
	bind     = flag.String("bind", "127.0.0.1", "interface to bind")
	port     = flag.Int("port", 8053, "port to run on")
	upstream = flag.String("upstream", "8.8.8.8:53", "upstream to use")
)

func main() {
	flag.Parse()

	srv := &http.Server{
		Addr:    *bind + ":" + strconv.Itoa(*port),
		Handler: &doh.Handler{*upstream},
	}

	log.Printf("Listening on %s:%d...\n", *bind, *port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start HTTP server: %s\n", err.Error())
	}
}
