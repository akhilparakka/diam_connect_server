package main

import (
	"fmt"
	"log"
	"net/http"
)

const webPort = "9080"

type Config struct {
	IPFSNode string
}

func main() {
	if false {
		WriteCIDToFile("QmWBAMX8egSrym7vWueBhGSdxD2VPWs2BrqacbimMwf17W")
	} else {
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

}
