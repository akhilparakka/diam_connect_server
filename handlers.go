package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
)

type UploadRequest struct {
	Name        string    `json:"name"`
	Desc        string    `json:"desc"`
	UserAddress string    `json:"user_address"`
	Time        time.Time `json:"time"`
	LikeCount   int64     `json:"like_count"`
}

func (app *Config) Check(w http.ResponseWriter, r *http.Request) {
	payload := jsonResponse{
		Error:   false,
		Message: "Hit the service",
	}

	app.writeJSON(w, http.StatusAccepted, payload)
}

func (app *Config) Upload(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		app.errorJSON(w, errors.New("file size should be less than 32mb"))
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		app.errorJSON(w, errors.New("error processing file"))
		return
	}
	defer file.Close()

	var jsonData UploadRequest

	jsonData.Name = r.FormValue("name")
	jsonData.Desc = r.FormValue("desc")
	jsonData.UserAddress = r.FormValue("user_address")
	timeStr := r.FormValue("time")
	jsonData.Time, err = time.Parse("Mon, 02 Jan 2006 15:04:05 GMT", timeStr)
	if err != nil {
		app.errorJSON(w, errors.New("error parsing time"))
		return
	}
	likeCountStr := r.FormValue("like_count")
	jsonData.LikeCount, err = strconv.ParseInt(likeCountStr, 10, 64)
	if err != nil {
		app.errorJSON(w, errors.New("error parsing like_count"))
		return
	}

	sh := shell.NewShell("http://54.219.7.190:4000")
	buf := new(bytes.Buffer)
	buf.ReadFrom(file)

	imageHash, err := sh.Add(buf)
	if err != nil {
		log.Println(err)
		app.errorJSON(w, errors.New("error adding image to IPFS"))
		return
	}

	metadata := map[string]interface{}{
		"name":         jsonData.Name,
		"description":  jsonData.Desc,
		"user_address": jsonData.UserAddress,
		"time":         jsonData.Time.Format(time.RFC3339),
		"like_count":   jsonData.LikeCount,
		"image_hash":   imageHash,
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		app.errorJSON(w, errors.New("error encoding metadata"))
		return
	}

	metadataReader := bytes.NewReader(metadataBytes)

	metadataHash, err := sh.Add(metadataReader)
	if err != nil {
		app.errorJSON(w, errors.New("error adding metadata to IPFS"))
		return
	}

	err = sh.Pin(metadataHash)
	if err != nil {
		app.errorJSON(w, errors.New("error pinning metadata"))
		return
	}

	app.writeJSON(w, http.StatusOK, map[string]interface{}{
		"metadata_hash": metadataHash,
	})

}
