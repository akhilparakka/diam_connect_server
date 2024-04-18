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

	// likeCountStr := r.FormValue("like_count")
	// jsonData.LikeCount, err = strconv.ParseInt(likeCountStr, 10, 64)
	// if err != nil {
	// 	app.errorJSON(w, errors.New("error parsing like_count"))
	// 	return
	// }

	log.Println("start")

	sh := shell.NewShell("http://54.219.7.190:4000")
	buf := new(bytes.Buffer)
	buf.ReadFrom(file)
	log.Println("check")

	imageHash, err := sh.Add(buf)
	if err != nil {
		log.Println(err)
		app.errorJSON(w, errors.New("error adding image to IPFS"))

	}
	log.Println("done")

	metadata := IPFSData{
		Name:        jsonData.Name,
		Description: jsonData.Desc,
		UA:          jsonData.UserAddress,
		Time:        time.Now(),
		Likes:       0,
		IH:          "http://54.219.7.190:8080/ipfs/" + imageHash,
	}
	log.Println("check 22")

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		app.errorJSON(w, errors.New("error encoding metadata"))
	}

	cid, err := ReadCIDFromFile()
	if err != nil {
		app.errorJSON(w, errors.New(err.Error()))

	}
	log.Println("check 6666")

	// Retrieve existing JSON data from IPFS
	existingJSON, err := fetchFromIPFS(cid) //QmZrXqJR7zuzxwhZyo1Sr92kwY1HmyeYyKqT96rxuPcFcq
	if err != nil {
		fmt.Println("Error fetching existing JSON data from IPFS:", err)
		app.errorJSON(w, errors.New(err.Error()))
	}

	// // Append a new JSON object
	// newJSON := `{"description":"tes4444t3",
	//             "image_hash":"4444",
	//             "like_count":444442,
	//             "name":"444444",
	//             "time":"2024-04-17T06:06:25Z",
	//             "user_address":"testaddress3"}`
	log.Println("check 0000000009")

	// Append newJSON to existingJSON array
	updatedJSON := appendJSON(existingJSON, string(metadataBytes))

	// Upload the updated JSON to IPFS
	hash, err := uploadToIPFS(updatedJSON)
	if err != nil {
		fmt.Println("Error uploading updated JSON to IPFS:", err)
		app.errorJSON(w, errors.New(err.Error()))
	}
	fmt.Println("Uploaded updated JSON to IPFS with hash:", hash)

	WriteCIDToFile(hash)
	app.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":        true,
		"metadata_hash": hash,
	})

}

type IPFSData struct {
	Description string    `json:"description"`
	IH          string    `json:"image_hash"`
	Likes       int       `json:"like_count"`
	Name        string    `json:"name"`
	Time        time.Time `json:"time"`
	UA          string    `json:"user_address"`
}

func appendJSON(existingJSON, newJSON string) string {
	// Unmarshal existing JSON array
	var existingArray []IPFSData

	if err := json.Unmarshal([]byte(existingJSON), &existingArray); err != nil {
		fmt.Println("Error unmarshaling existing JSON:", err)
		return existingJSON
	}
	log.Println(existingArray, " < - - - - ")

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

	log.Println(string(updatedJSON))

	return string(updatedJSON)
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

func uploadToIPFS(data string) (string, error) {
	// Prepare the request body
	// body := bytes.NewBufferString(data)

	// // Send the HTTP request to IPFS API for adding data
	// resp, err := http.Post(ipfsAPIURL+"/add", "text/plain", body)
	// if err != nil {
	// 	return "", err
	// }
	// defer resp.Body.Close()

	// // Read the response body
	// respBody, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	return "", err
	// }

	// // Unmarshal the response JSON
	// var ipfsResp IPFSResponse
	// err = json.Unmarshal(respBody, &ipfsResp)
	// if err != nil {
	// 	return "", err
	// }

	// return ipfsResp.Hash, nil
	sh := shell.NewShell("http://54.219.7.190:4000")

	//
	// Add the file to IPFS
	//

	// tsdBin, _ := json.Marshal(data)
	reader := bytes.NewReader([]byte(data))

	cid, err := sh.Add(reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		return "", err
	}
	fmt.Printf("added %s\n", cid)

	return cid, nil
}

var (
	mutex sync.Mutex
)

type CIDData struct {
	CID string `json:"CID"`
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

func (app *Config) retriveUserFeeds(w http.ResponseWriter, r *http.Request) {

}

// app.errorJSON(w, errors.New("error processing file"))
// app.writeJSON(w, http.StatusOK, map[string]interface{}{
// 	"metadata_hash": metadataHash,
// })
