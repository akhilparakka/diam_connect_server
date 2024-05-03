package main

import (
	"fmt"
	"log"
	"net/http"
)

func likesHandler() {
	for {

	}
}

func main() {
	if false {
		uploadToIPFS(`[{
			"description": "mangneot the powerful mutant",
			"image_hash": "http://54.219.7.190:8080/ipfs/QmVPZAuM43hecJDvXHaLKMiZmirN9PHTjW6xrPtjkpwMrq",
			"like_count": 0,
			"name": "JOSH",
			"time": "2024-04-18T10:59:31.79669848+05:30",
			"user_address": "GCPO4W2CW3BJZS3UPZLOVBMMXKNRPNXK74J7LJOMTKF46FREH5GQFL4Y",
			"id":"iwbvuddwibvd"
			
		  }]`)
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
