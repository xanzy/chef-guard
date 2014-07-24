//
// Copyright 2014, Sander van Harmelen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/xanzy/chef-guard/Godeps/_workspace/src/github.com/marpaia/chef-golang"
)

type Sandbox struct {
	SandboxId string                 `json:"sandbox_id"`
	Uri       string                 `json:"uri"`
	Checksums map[string]SandboxItem `json:"checksums"`
}

type SandboxItem struct {
	Url         string `json:"url"`
	NeedsUpload bool   `json:"needs_upload"`
}

func processCookbook(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if getEffectiveConfig("Mode", getOrgFromRequest(r)).(string) == "silent" || r.Method == "DELETE" {
			p.ServeHTTP(w, r)
			return
		}
		cg, err := newChefGuard(r)
		if err != nil {
			errorHandler(w, fmt.Sprintf("Failed to create a new ChefGuard structure: %s", err), http.StatusBadGateway)
			return
		}
		body, err := dumpBody(r)
		if err != nil {
			errorHandler(w, fmt.Sprintf("Failed to get body from call to %s: %s", r.URL.String(), err), http.StatusBadGateway)
			return
		}
		if err := json.Unmarshal(body, &cg.Cookbook); err != nil {
			errorHandler(w, fmt.Sprintf("Failed to unmarshal body %s: %s", string(body), err), http.StatusBadGateway)
			return
		}
		if cg.Cookbook.Frozen {
			if errCode, err := cg.checkCookbookFrozen(); err != nil {
				errorHandler(w, err.Error(), errCode)
				return
			}
			cg.CookbookPath = path.Join(cfg.Default.Tempdir, fmt.Sprintf("%s-%s", r.Header.Get("X-Ops-Userid"), cg.Cookbook.Name))
			if err := cg.processCookbookFiles(); err != nil {
				errorHandler(w, err.Error(), http.StatusBadGateway)
				return
			}
			defer func() {
				if err := os.RemoveAll(cg.CookbookPath); err != nil {
					WARNING.Printf("Failed to cleanup temp cookbook folder %s: %s", cg.CookbookPath, err)
				}
			}()
			if errCode, err := cg.validateCookbookStatus(); err != nil {
				errorHandler(w, err.Error(), errCode)
				return
			}
		}
		go metric.SimpleSend("chef-guard.success", "1")
		p.ServeHTTP(w, r)
	}
}

func (cg *ChefGuard) processCookbookFiles() error {
	if cg.OrganizationID == nil {
		if err := cg.getOrganizationID(); err != nil {
			return fmt.Errorf("Failed to get organization ID for %s: %s", cg.Organization, err)
		}
	}
	buf := new(bytes.Buffer)
	gw := gzip.NewWriter(buf)
	tw := tar.NewWriter(gw)

	for _, f := range cg.getAllCookbookFiles() {
		content, err := downloadCookbookFile(*cg.OrganizationID, f.Checksum)
		if err != nil {
			return fmt.Errorf("Failed to dowload %s from the %s cookbook: %s", f.Path, cg.Cookbook.Name, err)
		}
		if err := writeFileToDisk(path.Join(cg.CookbookPath, f.Path), strings.NewReader(string(content))); err != nil {
			return fmt.Errorf("Failed to write file %s to disk: %s", path.Join(cg.CookbookPath, f.Path), err)
		}
		// Save the md5 hash to the ChefGuard struct
		cg.FileHashes[f.Path] = md5.Sum(content)
		// Add the file to the tar archive
		header := &tar.Header{
			Name:    fmt.Sprintf("%s/%s", cg.Cookbook.Name, f.Path),
			Size:    int64(len(content)),
			Mode:    0644,
			ModTime: time.Now(),
		}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("Failed to create header for file %s: %s", f.Name, err)
		}
		if _, err := tw.Write(content); err != nil {
			return fmt.Errorf("Failed to write file %s to archive: %s", f.Name, err)
		}
	}
	if err := addMetadataJSON(tw, cg.Cookbook); err != nil {
		return fmt.Errorf("Failed to create metadata.json: %s", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("Failed to close the tar archive: %s", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("Failed to close the gzip archive: %s", err)
	}
	cg.TarFile = buf.Bytes()
	return nil
}

func (cg *ChefGuard) getOrganizationID() error {
	resp, err := cg.chefClient.Post("sandboxes", "application/json", nil, strings.NewReader(`{"checksums":{"00000000000000000000000000000000":null}}`))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := checkHTTPResponse(resp, []int{http.StatusOK, http.StatusCreated}); err != nil {
		return err
	}
	sb := new(Sandbox)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to get body from call to %s: %s", resp.Request.URL.String(), err)
	}
	if err := json.Unmarshal(body, &sb); err != nil {
		return err
	}
	re := regexp.MustCompile(`^.*/organization-(.*)\/checksum-.*$`)
	u := sb.Checksums["00000000000000000000000000000000"].Url
	if res := re.FindStringSubmatch(u); res != nil {
		cg.OrganizationID = &res[1]
		return nil
	}
	return fmt.Errorf("Could not find an organization ID in reply: %s", string(body))
}

func (cg *ChefGuard) getAllCookbookFiles() []struct{ chef.CookbookItem } {
	allFiles := []struct{ chef.CookbookItem }{}
	allFiles = append(allFiles, cg.Cookbook.Files...)
	allFiles = append(allFiles, cg.Cookbook.Definitions...)
	allFiles = append(allFiles, cg.Cookbook.Libraries...)
	allFiles = append(allFiles, cg.Cookbook.Attributes...)
	allFiles = append(allFiles, cg.Cookbook.Recipes...)
	allFiles = append(allFiles, cg.Cookbook.Providers...)
	allFiles = append(allFiles, cg.Cookbook.Resources...)
	allFiles = append(allFiles, cg.Cookbook.Templates...)
	allFiles = append(allFiles, cg.Cookbook.RootFiles...)
	return allFiles
}

func downloadCookbookFile(orgID, checksum string) ([]byte, error) {
	u, err := generateSignedURL(orgID, checksum)
	if err != nil {
		return nil, err
	}
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	if err := checkHTTPResponse(resp, []int{http.StatusOK}); err != nil {
		return nil, err
	}
	c, err := dumpBody(resp)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func generateSignedURL(orgID, checksum string) (*url.URL, error) {
	expires := time.Now().Unix() + 10
	stringToSign := fmt.Sprintf("GET\n\n\n%d\n/bookshelf/organization-%s/checksum-%s", expires, orgID, checksum)

	h := hmac.New(sha1.New, []byte(cfg.Chef.S3Secret))
	h.Write([]byte(stringToSign))
	signature := url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))

	var baseURL string
	switch cfg.Chef.Port {
	case "443":
		baseURL = fmt.Sprintf("https://%s", cfg.Chef.Server)
	case "80":
		baseURL = fmt.Sprintf("http://%s", cfg.Chef.Server)
	default:
		baseURL = fmt.Sprintf("%s:%s", cfg.Chef.Server, cfg.Chef.Port)
	}

	u, err := url.Parse(fmt.Sprintf("%s/bookshelf/organization-%s/checksum-%s?AWSAccessKeyId=%s&Expires=%d&Signature=%s", baseURL, orgID, checksum, cfg.Chef.S3Key, expires, signature))
	if err != nil {
		return nil, err
	}
	return u, nil
}

func writeFileToDisk(filePath string, content io.Reader) error {
	if err := os.MkdirAll(path.Dir(filePath), 0755); err != nil {
		return err
	}
	fo, err := os.Create(filePath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(fo, content); err != nil {
		return err
	}
	fo.Close()
	return nil
}

func addMetadataJSON(tw *tar.Writer, cb *chef.CookbookVersion) error {
	for _, f := range cb.RootFiles {
		if f.Name == "metadata.json" {
			return nil
		}
	}
	md, err := json.MarshalIndent(cb.Metadata, "", "  ")
	if err != nil {
		return err
	}
	md = DecodeMarshalledJSON(md)
	header := &tar.Header{
		Name:    fmt.Sprintf("%s/%s", cb.Name, "metadata.json"),
		Size:    int64(len(md)),
		Mode:    0644,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if _, err := tw.Write(md); err != nil {
		return err
	}
	return nil
}

func DecodeMarshalledJSON(b []byte) []byte {
	r := strings.NewReplacer(`\u003c`, `<`, `\u003e`, `>`, `\u0026`, `&`)
	s := r.Replace(string(b))
	return []byte(s)
}

func checkHTTPResponse(resp *http.Response, allowedStates []int) error {
	for _, s := range allowedStates {
		if resp.StatusCode == s {
			return nil
		}
	}
	errInfo := new(ErrorInfo)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to get body from call to %s: %s", resp.Request.URL.String(), err)
	}
	if err := json.Unmarshal(body, errInfo); err != nil {
		return err
	}
	if errInfo.Errors != nil {
		return fmt.Errorf(strings.Join(errInfo.Errors, ";"))
	}
	if errInfo.ErrorMessages != nil {
		return fmt.Errorf(strings.Join(errInfo.ErrorMessages, ";"))
	}
	return fmt.Errorf(string(body))
}

func dumpBody(r interface{}) (body []byte, err error) {
	switch r.(type) {
	case *http.Request:
		body, err = ioutil.ReadAll(r.(*http.Request).Body)
		r.(*http.Request).Body = ioutil.NopCloser(bytes.NewBuffer(body))
	case *http.Response:
		body, err = ioutil.ReadAll(r.(*http.Response).Body)
		r.(*http.Response).Body = ioutil.NopCloser(bytes.NewBuffer(body))
	}
	return body, err
}
