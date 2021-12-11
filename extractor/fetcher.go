package extractor

import (
	"encoding/json"
	"fmt"
	"github.com/go-ldap/ldap"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func DownloadPayload(entry *ldap.Entry) (string, error) {
	addr, err := url.Parse(entry.GetAttributeValue("javaCodeBase"))
	if err != nil {
		return "", err
	}

	if !strings.HasSuffix(addr.Path, ".jar") {
		addr.Path = fmt.Sprintf("/%s.class", entry.GetAttributeValue("javaFactory"))
	}

	filename, err := DownloadFile(addr)
	if err != nil {
		return "", err
	}

	return filename, err
}

func FetchFromLdap(address *url.URL) ([]string, error) {
	dialUrl := &url.URL{
		Scheme: address.Scheme,
		Host:   address.Host,
	}

	l, err := ldap.DialURL(dialUrl.String())
	if err != nil {
		return nil, err
	}
	defer l.Close()

	err = l.UnauthenticatedBind("")
	if err != nil {
		return nil, err
	}

	searchRequest := ldap.NewSearchRequest(
		strings.TrimLeft(address.Path, "/"),
		ldap.ScopeBaseObject, ldap.DerefAlways, 0, 0, false,
		"(objectClass=*)",
		[]string{"javaClassName", "javaCodeBase", "objectClass", "javaFactory"},
		[]ldap.Control{
			&ldap.ControlManageDsaIT{},
		},
	)

	sr, err := l.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	files := []string{}
	for _, entry := range sr.Entries {
		class := entry.GetAttributeValue("objectClass")

		if class == "javaNamingReference" {
			filename, err := DownloadPayload(entry)
			if err != nil {
				log.Printf("Failed to download payload: %v\n", err)
				continue
			}
			files = append(files, filename)
		} else {
			filename, err := SaveDetails(entry)
			if err != nil {
				log.Printf("Failed to save payload: %v\n", err)
				continue
			}
			files = append(files, filename)
		}
	}

	return files, nil
}

func SaveDetails(entry *ldap.Entry) (string, error) {
	filename := uuid.New().String() + ".json"

	err := os.MkdirAll("payloads/", os.ModePerm)
	if err != nil {
		return "", err
	}

	file, _ := json.MarshalIndent(entry, "", " ")

	err = ioutil.WriteFile("payloads/" + filename, file, 0644)
	if err != nil {
		return "", err
	}

	return filename, nil
}

func DownloadFile(url *url.URL) (string, error) {
	// Get the data
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	err = os.MkdirAll("payloads/", os.ModePerm)
	if err != nil {
		return "", err
	}

	// Create the file
	filename := uuid.New().String()
	if strings.HasSuffix(url.Path, ".jar") {
		filename += ".jar"
	} else if strings.HasSuffix(url.Path, ".class") {
		filename += ".class"
	}

	out, err := os.Create("payloads/" + filename)
	if err != nil {
		return "", err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return filename, err
}
