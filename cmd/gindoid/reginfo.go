package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/G-Node/libgin/libgin"
	yaml "gopkg.in/yaml.v2"
)

// repoFileURL returns the full URL to a file on the master branch of a
// repository.
func repoFileURL(conf *Configuration, repopath string, filename string) string {
	u, err := url.Parse(GetGINURL(conf))
	if err != nil {
		// not configured properly; return nothing
		return ""
	}
	fetchRepoPath := fmt.Sprintf("%s/raw/master/%s", repopath, filename)
	u.Path = fetchRepoPath
	return u.String()
}

// readFileAtURL returns the contents of a file at a given URL.
func readFileAtURL(url string) ([]byte, error) {
	client := &http.Client{}
	log.Printf("Fetching file at %q", url)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Request failed: %s", err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Request returned non-OK status: %s", resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Could not read file contents: %s", err.Error())
		return nil, err
	}
	return body, nil
}

// readRepoYAML parses the DOI registration info and returns a filled DOIRegInfo struct.
func readRepoYAML(infoyml []byte) (*libgin.RepositoryYAML, error) {
	yamlInfo := &libgin.RepositoryYAML{}
	err := yaml.Unmarshal(infoyml, yamlInfo)
	if err != nil {
		return nil, fmt.Errorf("error while reading DOI info: %s", err.Error())
	}
	if missing := checkMissingValues(yamlInfo); len(missing) > 0 {
		log.Print("DOI file is missing entries")
		return nil, fmt.Errorf(strings.Join(missing, " "))
	}
	return yamlInfo, nil
}

// checkMissingValues returns a list of messages for missing or invalid values.
// If all values are valid, the returned slice is empty.
func checkMissingValues(info *libgin.RepositoryYAML) []string {
	missing := []string{}
	if info.Title == "" {
		missing = append(missing, msgNoTitle)
	}
	if len(info.Authors) == 0 {
		missing = append(missing, msgNoAuthors)
	} else {
		for _, auth := range info.Authors {
			if auth.LastName == "" || auth.FirstName == "" {
				missing = append(missing, msgInvalidAuthors)
			}
		}
	}
	if info.Description == "" {
		missing = append(missing, msgNoDescription)
	}
	if info.License == nil || info.License.Name == "" || info.License.URL == "" {
		missing = append(missing, msgNoLicense)
	}
	if info.References != nil {
		for _, ref := range info.References {
			if (ref.Citation == "" && ref.Name == "") || ref.RefType == "" {
				missing = append(missing, msgInvalidReference)
			}
		}
	}
	return missing
}

// RegistrationRequest holds the encrypted and decrypted data of a registration
// request, as well as the unmarshalled data of the target repository's
// datacite.yml metadata.  It's used to render the preparation page (request
// page) for the user to review the metadata before finalising the request.
type RegistrationRequest struct {
	// Encrypted request data from GIN.
	EncryptedRequestData string
	// Decrypted and unmarshalled request data.
	*libgin.DOIRequestData
	// Used to display error or warning messages to the user through the templates.
	Message template.HTML
	// Metadata for the repository being registered
	Metadata *libgin.RepositoryMetadata
	// Errors during the registration process that get sent in the body of the
	// email to the administrators.
	ErrorMessages []string
}

func (d *RegistrationRequest) GetDOIURI() string {
	var re = regexp.MustCompile(`(.+)\/`)
	return string(re.ReplaceAll([]byte(d.Repository), []byte("doi/")))
}

func (d *RegistrationRequest) AsHTML() template.HTML {
	return template.HTML(d.Message)
}
