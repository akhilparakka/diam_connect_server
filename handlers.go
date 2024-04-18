package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
)

var (
	mutex sync.Mutex
)

type MainCID struct {
	CID string `json:"CID"`
}

type UploadRequest struct {
	Name        string    `json:"name"`
	Desc        string    `json:"desc"`
	UserAddress string    `json:"user_address"`
	Time        time.Time `json:"time"`
	LikeCount   int64     `json:"like_count"`
}

type MetadataResponse struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UserAddress string    `json:"user_address"`
	Time        time.Time `json:"time"`
	LikeCount   int64     `json:"like_count"`
	ImageHash   string    `json:"image_hash"`
}

type CIDData struct {
	CID string `json:"CID"`
}

type RequestPayload struct {
	UserAddress string `json:"user_address"`
}

type IPFSData struct {
	Description string    `json:"description"`
	IH          string    `json:"image_hash"`
	Likes       int       `json:"like_count"`
	Name        string    `json:"name"`
	Time        time.Time `json:"time"`
	UA          string    `json:"user_address"`
}

type HashRequest struct {
	Hash string `json:"hash"`
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

	if r.FormValue("user_address") == "" {
		app.errorJSON(w, errors.New("Invalid user address"))
		return

	}

	file, _, err := r.FormFile("image")
	if file == nil && r.FormValue("desc") == "" {
		app.errorJSON(w, errors.New("Request must contain either media or text"))
		return
	}

	if err != nil && err.Error() != "http: no such file" {
		app.errorJSON(w, errors.New("error processing file"))
		return
	}
	var imageHash string = ""

	if file != nil {
		defer file.Close()

		log.Println("start")

		sh := shell.NewShell("http://54.219.7.190:4000")
		buf := new(bytes.Buffer)
		buf.ReadFrom(file)
		log.Println("check")

		imageHash, err = sh.Add(buf)
		if err != nil {
			log.Println(err)
			app.errorJSON(w, errors.New("error adding image to IPFS"))

		}
		log.Println("done")

		imageHash = "http://54.219.7.190:8080/ipfs/" + imageHash
	}

	metadata := IPFSData{
		Name:        "",
		Description: r.FormValue("desc"),
		UA:          r.FormValue("user_address"),
		Time:        time.Now(),
		Likes:       0,
		IH:          imageHash,
	}
	log.Println("check 22")

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		app.errorJSON(w, errors.New("error encoding metadata"))
		return
	}

	cid, err := ReadCIDFromFile()
	if err != nil {
		app.errorJSON(w, errors.New(err.Error()))
		return
	}
	log.Println("check 6666")

	// Retrieve existing JSON data from IPFS
	existingJSON, err := fetchFromIPFS(cid) //QmZrXqJR7zuzxwhZyo1Sr92kwY1HmyeYyKqT96rxuPcFcq
	if err != nil {
		fmt.Println("Error fetching existing JSON data from IPFS:", err)
		app.errorJSON(w, errors.New(err.Error()))
		return
	}

	log.Println("check 0000000009")

	updatedJSON := appendJSON(existingJSON, string(metadataBytes))

	// Upload the updated JSON to IPFS
	hash, err := uploadToIPFS(updatedJSON)
	if err != nil {
		fmt.Println("Error uploading updated JSON to IPFS:", err)
		app.errorJSON(w, errors.New(err.Error()))
		return
	}
	fmt.Println("Uploaded updated JSON to IPFS with hash:", hash)

	WriteCIDToFile(hash)
	app.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":        true,
		"metadata_hash": hash,
	})

}
func (app *Config) getMetaData(w http.ResponseWriter, r *http.Request) {

	var payload RequestPayload

	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	cid, err := app.getCidFromFile()
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	url := fmt.Sprintf("http://54.219.7.190:8080/ipfs/%s", cid)
	fmt.Printf("URL: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	var data []MetadataResponse
	err = json.Unmarshal([]byte(body), &data)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	filteredData := make([]MetadataResponse, 0)
	for _, item := range data {
		if item.UserAddress == payload.UserAddress {
			filteredData = append(filteredData, item)
		}
	}

	app.writeJSON(w, http.StatusOK, filteredData)
}

func ReadCIDFromFile() (string, error) {
	mutex.Lock()
	defer mutex.Unlock()

	// Open the file for reading
	file, err := os.Open("mainCID.json")
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Decode JSON data into CIDData struct
	var cidData CIDData
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&cidData)
	if err != nil {
		return "", err
	}

	return cidData.CID, nil
}

func fetchFromIPFS(cid string) (string, error) {
	// Send HTTP GET request to IPFS API
	resp, err := http.Get("http://54.219.7.190:8080/ipfs/" + cid)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read response body
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Convert response body to string
	return string(data), nil
}

func appendJSON(existingJSON, newJSON string) string {
	// Unmarshal existing JSON array
	var existingArray []IPFSData

	if err := json.Unmarshal([]byte(existingJSON), &existingArray); err != nil {
		fmt.Println("Error unmarshaling existing JSON:", err)
		return existingJSON
	}

	// Unmarshal new JSON object
	var newObj IPFSData
	if err := json.Unmarshal([]byte(newJSON), &newObj); err != nil {
		fmt.Println("Error unmarshaling new JSON:", err)
		return existingJSON
	}

	// Append new JSON object to existing array
	existingArray = append(existingArray, newObj)

	// Marshal the updated array back to JSON
	updatedJSON, err := json.Marshal(existingArray)
	if err != nil {
		fmt.Println("Error marshaling updated JSON:", err)
		return existingJSON
	}

	return string(updatedJSON)
}

func uploadToIPFS(data string) (string, error) {

	sh := shell.NewShell("http://54.219.7.190:4000")

	reader := bytes.NewReader([]byte(data))

	cid, err := sh.Add(reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		return "", err
	}

	return cid, nil
}

func WriteCIDToFile(cid string) error {
	mutex.Lock()
	defer mutex.Unlock()

	// Open or create the file
	file, err := os.Create("mainCID.json")
	if err != nil {
		return err
	}
	defer file.Close()

	// Create CIDData struct
	cidData := CIDData{
		CID: cid,
	}

	// Encode CIDData struct to JSON and write to file
	encoder := json.NewEncoder(file)
	err = encoder.Encode(cidData)
	if err != nil {
		return err
	}

	return nil
}

func (app *Config) getCIDFromFile(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadFile("mainCID.json")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var mainCID MainCID
	err = json.Unmarshal(data, &mainCID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mainCID)
}

func (app *Config) getCidFromFile() (string, error) {
	data, err := ioutil.ReadFile("mainCID.json")
	if err != nil {
		return "", err
	}

	var mainCID MainCID
	err = json.Unmarshal(data, &mainCID)
	if err != nil {
		return "", err
	}

	return mainCID.CID, nil
}
