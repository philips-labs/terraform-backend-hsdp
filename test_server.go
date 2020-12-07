package main

import (
	"log"
	"net/http"

	"github.com/philips-labs/terraform-backend-http/backend"
	"github.com/philips-labs/terraform-backend-http/backend/store/s3"
)

func main() {
	// create a store
	store := s3.NewStore(&s3.Options{})

	// create a backend
	tfbackend := backend.NewBackend(store, &backend.Options{
		EncryptionKey: []byte("thisishardlysecure"),
		Logger: func(level, message string, err error) {
			if err != nil {
				log.Printf("%s: %s - %v", level, message, err)
			} else {
				log.Printf("%s: %s", level, message)
			}
		},
		GetMetadataFunc: func(state map[string]interface{}) map[string]interface{} {
			// fmt.Println(state)
			return map[string]interface{}{
				"test": "metadata",
			}
		},
	})
	if err := tfbackend.Init(); err != nil {
		log.Fatal(err)
	}

	// add handlers
	http.HandleFunc("/backend", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "LOCK":
			tfbackend.HandleLockState(w, r)
		case "UNLOCK":
			tfbackend.HandleUnlockState(w, r)
		case http.MethodGet:
			tfbackend.HandleGetState(w, r)
		case http.MethodPost:
			tfbackend.HandleUpdateState(w, r)
		case http.MethodDelete:
			tfbackend.HandleDeleteState(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	log.Println("Starting test server on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
