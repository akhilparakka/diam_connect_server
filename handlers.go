package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/diamcircle/go/clients/auroraclient"
	"github.com/diamcircle/go/keypair"
	"github.com/diamcircle/go/network"
	"github.com/diamcircle/go/txnbuild"
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
	Type        int       `json:"type"`
}

type CIDData struct {
	CID string `json:"CID"`
}

type RequestPayload struct {
	UserAddress string `json:"user_address"`
}

type IPFSData struct {
	Description string         `json:"description"`
	IH          string         `json:"image_hash"`
	Likes       int            `json:"like_count"`
	Name        string         `json:"name"`
	Time        time.Time      `json:"time"`
	UA          string         `json:"user_address"`
	Id          string         `json:"id"`
	Type        int            `json:"type"`
	Mapping     map[string]int `json:"mapping"`
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

func uploadData(token string, uid string, file multipart.File, fileName string) error {
	url := "http://10.0.0.15:3001/v1/upload-data"

	payload := new(bytes.Buffer)
	writer := multipart.NewWriter(payload)
	_ = writer.WriteField("uid", uid)
	part, err := writer.CreateFormFile("files", fileName)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}
	writer.Close()

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return err
	}

	req.Header.Add("Accept", "*/*")
	req.Header.Add("User-Agent", "Thunder Client (https://www.thunderclient.com)")
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	fmt.Println(res)
	fmt.Println(string(body))

	return nil
}

func (app *Config) Upload(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		app.errorJSON(w, errors.New("file size should be less than 32mb"))
		return
	}

	userAddress := r.FormValue("user_address")
	if userAddress == "" {
		app.errorJSON(w, errors.New("Invalid user address"))
		return
	}

	mediaType := r.FormValue("media_type")
	if mediaType == "" {
		app.errorJSON(w, errors.New("Invalid media type"))
		return
	}

	file, fileHeader, err := r.FormFile("image")
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

		sh := shell.NewShell("https://uploadipfs.diamcircle.io")
		buf := new(bytes.Buffer)
		buf.ReadFrom(file)
		log.Println("check")

		imageHash, err = sh.Add(buf)
		if err != nil {
			log.Println(err)
			app.errorJSON(w, errors.New("error adding image to IPFS"))
			return
		}
		log.Println("done")

		imageHash = "https://browseipfs.diamcircle.io/ipfs/" + imageHash
	}

	var _type int

	switch mediaType {
	case "1":
		_type = 1
	case "2":
		_type = 2
	case "3":
		_type = 3
	default:
		app.errorJSON(w, errors.New("Invalid media type"))
		return
	}

	metadata := IPFSData{
		Name:        "",
		Description: r.FormValue("desc"),
		UA:          userAddress,
		Time:        time.Now(),
		Likes:       0,
		IH:          imageHash,
		Id:          StringRandom(10),
		Type:        _type,
		Mapping:     make(map[string]int),
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

	existingJSON, err := fetchFromIPFS(cid)
	if err != nil {
		fmt.Println("Error fetching existing JSON data from IPFS:", err)
		app.errorJSON(w, errors.New(err.Error()))
		return
	}

	log.Println("check 0000000009")

	updatedJSON := appendJSON(existingJSON, string(metadataBytes))

	hash, err := uploadToIPFS(updatedJSON)
	if err != nil {
		fmt.Println("Error uploading updated JSON to IPFS:", err)
		app.errorJSON(w, errors.New(err.Error()))
		return
	}
	fmt.Println("Uploaded updated JSON to IPFS with hash:", hash)

	WriteCIDToFile(hash)

	token, err := getBearerToken()
	if err != nil {
		app.errorJSON(w, errors.New("failed to get bearer token"))
		return
	}

	if file != nil {
		_, err = file.Seek(0, io.SeekStart) // Reset the file pointer to the beginning
		if err != nil {
			app.errorJSON(w, errors.New("error resetting file pointer"))
			return
		}
		err = uploadData(token, userAddress, file, fileHeader.Filename)
		if err != nil {
			app.errorJSON(w, errors.New("failed to upload data"))
			return
		}
	}

	app.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":        true,
		"metadata_hash": hash,
	})
}

func getBearerToken() (string, error) {
	url := "http://10.0.0.15:3001/v1/login"
	payload := strings.NewReader("{ \"userName\":\"diamRoot\", \"mpin\":\"95c21b00cad15f9b1357dafc3bbd8495\" }")

	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return "", err
	}

	req.Header.Add("Accept", "*/*")
	req.Header.Add("User-Agent", "Thunder Client (https://www.thunderclient.com)")
	req.Header.Add("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", err
	}

	token, ok := response["token"].(string)
	if !ok {
		return "", errors.New("invalid token response")
	}

	return token, nil
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

	url := fmt.Sprintf("https://browseipfs.diamcircle.io/ipfs/%s", cid)
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
	resp, err := http.Get("https://browseipfs.diamcircle.io/ipfs/" + cid)
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

	sh := shell.NewShell("https://uploadipfs.diamcircle.io")

	reader := bytes.NewReader([]byte(data))

	cid, err := sh.Add(reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err)
		return "", err
	}

	log.Println(cid)

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

func (app *Config) addLikesToPosts(w http.ResponseWriter, r *http.Request) {
	type likesP struct {
		PublicKey string `json:"public_key"`
		Id        string `json:"id"`
		Count     int    `json:"count"`
	}
	var payload likesP

	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	cid, err := ReadCIDFromFile()
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	url := fmt.Sprintf("https://browseipfs.diamcircle.io/ipfs/%s", cid)
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

	var data []IPFSData
	err = json.Unmarshal([]byte(body), &data)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	for i, item := range data {
		if item.Id == payload.Id {
			if _, ok := item.Mapping[payload.PublicKey]; ok {
				app.writeJSON(w, http.StatusOK, "user has already liked the post!")
				return
			}

			temp := item.Likes
			data[i].Likes = temp + payload.Count

			if data[i].Likes == 99 {
				source := "SBNBAF32CLQYKVUSGLUKSHSGNKMZPYBKYEWXDL6CAAHMQWD5I3DC2ZV4"
				client := auroraclient.DefaultTestNetClient

				sourceKP := keypair.MustParseFull(source)

				sourceAccountRequest := auroraclient.AccountRequest{AccountID: sourceKP.Address()}

				sourceAccount, err := client.AccountDetail(sourceAccountRequest)
				if err != nil {
					app.errorJSON(w, err, http.StatusInternalServerError)
					return
				}

				var Operations []txnbuild.Operation

				for i := 0; i < len(data); i++ {
					Operations = append(Operations, &txnbuild.Payment{
						Destination: item.UA,
						Amount:      "50",
						Asset:       txnbuild.NativeAsset{},
					})
				}

				tx, err := txnbuild.NewTransaction(
					txnbuild.TransactionParams{
						SourceAccount:        &sourceAccount,
						IncrementSequenceNum: true,
						BaseFee:              txnbuild.MinBaseFee,
						Timebounds:           txnbuild.NewInfiniteTimeout(),
						Operations:           Operations,
					},
				)

				if err != nil {
					app.errorJSON(w, err, http.StatusInternalServerError)
					return
				}

				tx, err = tx.Sign(network.TestNetworkPassphrase, sourceKP)
				if err != nil {
					app.errorJSON(w, err, http.StatusInternalServerError)
					return
				}

				_, err = auroraclient.DefaultTestNetClient.SubmitTransaction(tx)
				if err != nil {
					app.errorJSON(w, err, http.StatusInternalServerError)
					return
				}
			}

			if payload.Count > 0 {
				data[i].Mapping[payload.PublicKey] = 1
			} else if payload.Count < 0 {
				delete(data[i].Mapping, payload.PublicKey)
			}

			break
		}
	}

	marshalled, err := json.Marshal(data)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	_cid, err := uploadToIPFS(string(marshalled))
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	err = WriteCIDToFile(_cid)
	if err != nil {
		app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.writeJSON(w, http.StatusOK, "like added !")
}

func (app *Config) getPostFromId(w http.ResponseWriter, r *http.Request) {
	type getPost struct {
		PublicKey  string `json:"user_address"`
		Image_hash string `json:"image_hash"`
	}
	var payload getPost

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

	url := fmt.Sprintf("https://browseipfs.diamcircle.io/ipfs/%s", cid)
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
		if (item.UserAddress == payload.PublicKey) && (item.ImageHash == payload.Image_hash) {
			filteredData = append(filteredData, item)
		}
	}

	app.writeJSON(w, http.StatusOK, filteredData)
}

func (app *Config) getPostFromAddress(w http.ResponseWriter, r *http.Request) {
	type getPost struct {
		PublicKey string `json:"user_address"`
	}
	var payload getPost

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

	url := fmt.Sprintf("https://browseipfs.diamcircle.io/ipfs/%s", cid)
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
		if item.UserAddress == payload.PublicKey {
			filteredData = append(filteredData, item)
		}
	}

	app.writeJSON(w, http.StatusOK, filteredData)
}
