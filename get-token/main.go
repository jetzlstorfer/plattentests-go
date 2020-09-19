package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/browser"
	"github.com/spotify"
)

const redirectURI = "http://localhost:8888/callback"
const autologin = false

var (
	auth   = spotify.NewAuthenticator(redirectURI, spotify.ScopePlaylistModifyPrivate)
	ch     = make(chan *spotify.Client)
	state  = "myCrazyState"
	s3     *s3manager.Uploader
	config struct {
		Bucket    string `required:"true"`
		TokenFile string `envconfig:"TOKEN_FILE" required:"true"`
		Region    string `required:"true"`
	}
)

func main() {
	err := envconfig.Process("", &config)
	if err != nil {
		log.Fatal("could not process env variables: " + err.Error())
	}

	// will read credentials automatically from ENV variables
	log.Println("Region: ", config.Region)
	s3 = s3manager.NewUploader(session.Must(session.NewSession(&aws.Config{Region: aws.String(config.Region)})))

	// start an HTTP server
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})
	go http.ListenAndServe(":8888", nil)

	url := auth.AuthURL(state)
	log.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	if autologin {
		err := browser.OpenURL(url)
		if err != nil {
			log.Fatalln(err.Error())
		}
	}

	// wait for auth to complete
	client := <-ch

	// use the client to make calls that require authorization
	user, err := client.CurrentUser()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("You are logged in as: %s (%s)", user.DisplayName, user.ID)
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	log.Println("token retrieved")
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}

	btys, err := json.Marshal(tok)
	if err != nil {
		log.Fatalf("could not marshal token: %v", err)
	}

	err = ioutil.WriteFile(config.TokenFile, btys, 0644)
	if err != nil {
		log.Fatalf("could not write file: %v", err)
	}

	// Save the token to S3 for later use.
	if _, err := s3.Upload(&s3manager.UploadInput{
		Bucket: aws.String(config.Bucket),
		Key:    aws.String(config.TokenFile),
		Body:   bytes.NewReader(btys),
	}); err != nil {
		log.Fatalf("could not write token to s3: %v", err)
	}

	// use the token to get an authenticated client
	client := auth.NewClient(tok)
	fmt.Fprintf(w, "Login Completed!")
	ch <- &client
}
