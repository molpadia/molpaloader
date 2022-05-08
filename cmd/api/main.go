package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("failed to load env flie")
	}
}

func env(key string, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return def
}

func main() {
	var (
		addr = flag.String("addr", env("ADDR", ":4443"), "web server address")
		cert = flag.String("cert", env("CERT_FILE", ""), "path of TLS certificate file")
		key  = flag.String("key", env("KEY_FILE", ""), "path of TLS private key file")
	)
	flag.Parse()

	router := mux.NewRouter()
	registerEndpoints(router)

	srv := &http.Server{
		Handler:      router,
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

// Register REST API endpionts to the router.
func registerEndpoints(router *mux.Router) {
	router.HandleFunc("/files", uploadFile).Methods("POST")
}
