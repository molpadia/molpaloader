package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type appHandler func(http.ResponseWriter, *http.Request) error

type appError struct {
	Code    int
	Message string
}

func (e *appError) Error() string {
	return fmt.Sprintf("Internal app error: %s", e.Message)
}

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		log.Printf("Server error: %s", err)

		if e, ok := err.(*appError); ok {
			http.Error(w, e.Message, e.Code)
		} else {
			http.Error(w, fmt.Sprintf("Internal app error: %s", err), http.StatusInternalServerError)
		}
	}
}

var (
	addr = flag.String("addr", env("ADDR", ":4443"), "web server address")
	cert = flag.String("cert", env("CERT_FILE", ""), "path of TLS certificate file")
	key  = flag.String("key", env("CERT_KEY", ""), "path of TLS private key file")
)

func main() {
	flag.Parse()

	r := mux.NewRouter()
	registerEndpoints(r)

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

// Register API endpionts to the router.
func registerEndpoints(r *mux.Router) {
	r.Methods("POST").Path("/molpastream/v1/videos").Handler(appHandler(createVideo))
	r.Methods("PUT").Path("/upload/molpastream/v1/videos/{id}").Handler(appHandler(uploadVideo))
}
