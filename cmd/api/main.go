package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/molpadia/molpastream/internal/app"
)

var (
	addr = flag.String("addr", env("ADDR", ":4443"), "web server address")
	cert = flag.String("cert", env("CERT_FILE", ""), "path of TLS certificate file")
	key  = flag.String("key", env("CERT_KEY", ""), "path of TLS private key file")
)

// Get the value of environment variables.
func env(key string, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func main() {
	flag.Parse()

	r := mux.NewRouter()
	app.SetupRoutes(r)

	srv := &http.Server{
		Handler:      r,
		Addr:         *addr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Printf("the server started on port: %s\n", *addr)

	if *cert != "" && *key != "" {
		log.Fatal(srv.ListenAndServeTLS(*cert, *key))
	} else {
		log.Fatal(srv.ListenAndServe())
	}

	defer srv.Close()
}
