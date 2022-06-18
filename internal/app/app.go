package app

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type appHandler func(http.ResponseWriter, *http.Request) error

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		log.Printf("Error: %v", err)
		if e, ok := err.(*AppError); ok {
			replyJSON(w, e, e.Code)
		} else {
			http.Error(w, fmt.Sprintf("Internal server error: %v", err), http.StatusInternalServerError)
		}
	}
}

// Register API endpoints to the router.
func SetupRoutes(r *mux.Router) {
	r.Methods("POST").Path("/molpastream/v1/videos").Handler(appHandler(createVideo))
	r.Methods("PUT").Path("/upload/molpastream/v1/videos/{id}").Handler(appHandler(uploadVideo))
}
