package main

import (
	"fmt"
	"log"
	"net/http"
)

const webPort = "8081"

type Config struct {
	IPFSNode string
}

func main() {
	app := Config{}

	log.Printf("Starting server on port %s", webPort)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", webPort),
		Handler: app.routes(),
	}

	err := srv.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
