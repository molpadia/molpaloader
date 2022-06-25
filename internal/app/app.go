package app

import (
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gorilla/mux"
	"github.com/molpadia/molpastream/internal/infrastructure/persistence"
)

type appHandler func(http.ResponseWriter, *http.Request) error

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		log.Printf("Error: %v", err)
		if e, ok := err.(*appError); ok {
			replyJSON(w, e, e.Code)
		} else {
			replyJSON(w, fmt.Sprintf("Internal server error: %v", err), http.StatusInternalServerError)
		}
	}
}

// Register API endpoints to the router.
func SetupRoutes(r *mux.Router) {
	sess := session.Must(session.NewSession())
	c := &controller{persistence.NewVideoRepository(sess), persistence.NewUploader(sess)}
	r.Methods("GET").Path("/molpastream/v1/videos/{id}").Handler(appHandler(c.getVideo))
	r.Methods("POST").Path("/molpastream/v1/videos").Handler(appHandler(c.createVideo))
	r.Methods("PUT").Path("/upload/molpastream/v1/videos/{id}").Handler(appHandler(c.uploadVideo))
}
