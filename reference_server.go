package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/philips-software/go-hsdp-api/console"

	"github.com/cloudfoundry-community/gautocloud"
	"github.com/dgrijalva/jwt-go"
	"github.com/philips-software/gautocloud-connectors/hsdp"

	"github.com/philips-labs/terraform-backend-http/backend"
	"github.com/philips-labs/terraform-backend-http/backend/store/s3"
)

func main() {
	// S3 bucket
	var svc *hsdp.S3MinioClient
	err := gautocloud.Inject(&svc)
	if err != nil {
		log.Printf("gautocloud: %v\n", err)
		return
	}

	// create a store
	store := s3.NewStore(&s3.Options{
		Client: svc.Client,
		Bucket: svc.Bucket,
	})

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
		GetRefFunc: refFunc(os.Getenv("REGION")),
	})
	if err := tfbackend.Init(); err != nil {
		log.Fatal(err)
	}

	// add handlers
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

	log.Println("Starting test server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func refFunc(region string) func(*http.Request) (string, error) {
	client, err := console.NewClient(nil, &console.Config{
		Region: region,
	})
	errFunc := func(r *http.Request) (string, error) {
		return "", err
	}
	if err != nil {
		return errFunc
	}
	return func(r *http.Request) (string, error) {
		// Authenticate
		username, password, ok := r.BasicAuth()
		if !ok {
			return "", fmt.Errorf("missing authentication")
		}
		c, err := client.WithLogin(username, password)
		if err != nil {
			return "", err
		}
		token, _ := jwt.Parse(c.IDToken(), func(token *jwt.Token) (interface{}, error) {
			return nil, nil
		})
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || claims["sub"] == "" {
			return "", fmt.Errorf("missing or invalid claims in IDToken")
		}
		userUUID := claims["sub"].(string)

		return filepath.Join(userUUID, r.URL.Path), nil
	}
}