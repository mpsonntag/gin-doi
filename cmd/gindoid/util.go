package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
)

func readBody(r *http.Request) (*string, error) {
	body, err := ioutil.ReadAll(r.Body)
	x := string(body)
	return &x, err
}

// Decrypt from base64 to decrypted string
func Decrypt(key []byte, cryptoText string) (string, error) {
	// TODO: Move to libgin
	ciphertext, _ := base64.URLEncoding.DecodeString(cryptoText)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	if len(ciphertext) < aes.BlockSize {
		return "", err
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)

	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(ciphertext, ciphertext)

	return fmt.Sprintf("%s", ciphertext), nil
}

// IsRegisteredDOI returns True if a given DOI is registered publicly.
// It simply checks if https://doi.org/<doi> returns a status code other than NotFound.
func IsRegisteredDOI(doi string) bool {
	url := fmt.Sprintf("https://doi.org/%s", doi)
	resp, err := http.Get(url)
	if err != nil {
		log.Errorf("Could not query for doi: %s at %s", doi, url)
		return false
	}
	if resp.StatusCode != http.StatusNotFound {
		return true
	}
	return false
}

// DoDOIJob starts the DOI registration process by authenticating with the GIN server and adding a new DOIJob to the jobQueue.
func DoDOIJob(w http.ResponseWriter, r *http.Request, jobQueue chan DOIJob, conf *Configuration) {
	// Make sure we can only be called with an HTTP POST request.
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	dReq := DOIReq{}
	// TODO: Error checking
	body, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &dReq)
	log.WithFields(log.Fields{
		"request": fmt.Sprintf("%+v", dReq),
		"source":  "DoDOIJob",
	}).Debug("Unmarshaled a DOI request")

	user, err := conf.GIN.Session.RequestAccount(dReq.OAuthLogin)
	if err != nil {
		log.WithFields(log.Fields{
			"request": fmt.Sprintf("%+v", dReq),
			"source":  "DoDOIJob",
			"error":   err,
		}).Debug("Could not get userdata")
		dReq.Message = template.HTML(msgNotLoggedIn)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	dReq.User = user
	// TODO Error checking
	uuid := makeUUID(dReq.URI)
	ok, doiInfo := ValidDOIFile(dReq.URI, conf)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	doiInfo.UUID = uuid
	doi := conf.DOIBase + doiInfo.UUID[:6]
	doiInfo.DOI = doi
	dReq.DOIInfo = doiInfo

	if IsRegisteredDOI(doi) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(msgAlreadyRegistered, doi, doi)))
		return
	}
	job := DOIJob{Source: dReq.URI, User: user, Request: dReq, Name: doiInfo.UUID, Config: conf}
	jobQueue <- job
	// Render success.
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(fmt.Sprintf(msgServerIsArchiving, doi)))
}

// InitDOIJob renders the page for the staging area, where information is provided to the user and offers to start the DOI registration request.
// It validates the metadata provided from the GIN repository and shows appropriate error messages and instructions.
func InitDOIJob(w http.ResponseWriter, r *http.Request, conf *Configuration) {
	log.Infof("Got a new DOI request")
	if err := r.ParseForm(); err != nil {
		log.WithFields(log.Fields{
			"source": "Init",
		}).Debug("Could not parse form data")
		w.WriteHeader(http.StatusInternalServerError)
		// TODO: Notify via email (maybe)
		return
	}
	t, err := template.ParseFiles(filepath.Join(conf.TemplatePath, "initjob.tmpl")) // Parse template file.
	if err != nil {
		log.WithFields(log.Fields{
			"source": "DoDOIJob",
			"error":  err,
		}).Debug("Could not parse init template")
		w.WriteHeader(http.StatusInternalServerError)
		// TODO: Notify via email
		return
	}

	URI := r.Form.Get("repo")
	enctoken := r.Form.Get("verification")
	username := r.Form.Get("user")

	log.Infof("Got request: [URI: %s] [username: %s] [Encrypted token: %s]", URI, username, enctoken)
	dReq := DOIReq{}
	dReq.DOIInfo = &DOIRegInfo{}

	// If all are missing, redirect to root path?

	// If any of the values is missing, render invalid request page
	if len(URI) == 0 || len(username) == 0 || len(enctoken) == 0 {
		log.WithFields(log.Fields{
			"source":       "InitDOIJob",
			"URI":          URI,
			"username":     username,
			"verification": enctoken,
		}).Error("Invalid request: missing fields in query string")
		w.WriteHeader(http.StatusBadRequest)
		dReq.Message = template.HTML(msgInvalidRequest)
		t.Execute(w, dReq)
		return
	}

	dReq.URI = URI
	dReq.OAuthLogin = username

	// Check verification string
	if !verifyRequest(URI, username, enctoken, conf.Key) {
		log.WithFields(log.Fields{
			"source":       "InitDOIJob",
			"URI":          URI,
			"username":     username,
			"verification": enctoken,
		}).Error("Invalid request: failed to verify")
		w.WriteHeader(http.StatusBadRequest)
		dReq.Message = template.HTML(msgInvalidRequest)
		t.Execute(w, dReq)
		return
	}

	// check for doifile
	if ok, doiInfo := ValidDOIFile(URI, conf); ok {
		j, _ := json.MarshalIndent(doiInfo, "", "  ")
		log.Debugf("Received DOI information: %s", string(j))
		dReq.DOIInfo = doiInfo
		err = t.Execute(w, dReq)
		if err != nil {
			log.WithFields(log.Fields{
				"request": dReq,
				"source":  "Init",
				"error":   err,
			}).Error("Could not parse template")
			return
		}
	} else if doiInfo != nil {
		log.WithFields(log.Fields{
			"doiInfo": doiInfo,
			"source":  "Init",
			"error":   err,
		}).Debug("DOIfile File invalid")
		if doiInfo.Missing != nil {
			dReq.Message = template.HTML(msgInvalidDOI + " <p>Issue:<i> " + doiInfo.Missing[0] + "</i>")
		} else {
			dReq.Message = template.HTML(msgInvalidDOI + msgBadEncoding)
		}
		dReq.DOIInfo = &DOIRegInfo{}
		err = t.Execute(w, dReq)
		if err != nil {
			log.WithFields(log.Fields{
				"doiInfo": doiInfo,
				"request": dReq,
				"source":  "Init",
				"error":   err,
			}).Error("Could not parse template")
			return
		}
		return
	} else {
		dReq.Message = template.HTML(msgInvalidDOI)
		t.Execute(w, dReq)
		if err != nil {
			log.WithFields(log.Fields{
				"request": dReq,
				"source":  "Init",
				"error":   err,
			}).Error("Could not parse template")
			return
		}
		return
	}
}

// DOIMData holds all the metadata for a dataset that's in the process of being registered.
type DOIMData struct {
	Data struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			DOI        string      `json:"doi"`
			Identifier string      `json:"identifier"`
			URL        interface{} `json:"url"`
			Author     []struct {
				Literal string `json:"literal"`
			} `json:"author"`
			Title               string      `json:"title"`
			ContainerTitle      string      `json:"container-title"`
			Description         string      `json:"description"`
			ResourceTypeSubtype string      `json:"resource-type-subtype"`
			DataCenterID        string      `json:"data-center-id"`
			MemberID            string      `json:"member-id"`
			ResourceTypeID      string      `json:"resource-type-id"`
			Version             string      `json:"version"`
			License             interface{} `json:"license"`
			SchemaVersion       string      `json:"schema-version"`
			Results             []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Count int    `json:"count"`
			} `json:"results"`
			RelatedIdentifiers []struct {
				RelationTypeID    string `json:"relation-type-id"`
				RelatedIdentifier string `json:"related-identifier"`
			} `json:"related-identifiers"`
			Published  string      `json:"published"`
			Registered time.Time   `json:"registered"`
			Updated    time.Time   `json:"updated"`
			Media      interface{} `json:"media"`
			XML        string      `json:"xml"`
		} `json:"attributes"`
		Relationships struct {
			DataCenter struct {
				Meta struct {
				} `json:"meta"`
			} `json:"data-center"`
			Member struct {
				Meta struct {
				} `json:"meta"`
			} `json:"member"`
			ResourceType struct {
				Meta struct {
				} `json:"meta"`
			} `json:"resource-type"`
		} `json:"relationships"`
	} `json:"data"`
}

func verifyRequest(repo, username, verification, key string) bool {
	plaintext, err := Decrypt([]byte(key), verification)
	if err != nil {
		log.WithFields(log.Fields{
			"source":       "verifyRequest",
			"repo":         repo,
			"username":     username,
			"verification": verification,
		}).Error("Invalid request: failed to decrypt verification string")
		return false
	}

	return plaintext == repo+username
}
