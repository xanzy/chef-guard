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
	"compress/gzip"
	"crypto/md5"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type ErrorInfo struct {
	Error         string   `json:"error,omitempty"`
	Errors        []string `json:"errors,omitempty"`
	ErrorMessages []string `json:"error_messages,omitempty"`
}

type SourceCookbook struct {
	artifact         bool
	tagged           bool
	gitHubOrg        string
	File             string   `json:"file,omitempty"`
	DownloadURL      *url.URL `json:"url"`
	EndpointPriority int      `json:"endpoint_priority"`
	LocationType     string   `json:"location_type"`
	LocationPath     string   `json:"location_path,omitempty"`
}

type BerksResult map[string]map[string]*SourceCookbook

type Constraints struct {
	CookbookVersions map[string]string   `json:"cookbook_versions"`
	RunList          []string            `json:"run_list"`
	EnvRunLists      map[string][]string `json:"env_run_lists"`
}

func unmarshalConstraints(body []byte) (*Constraints, error) {
	var c Constraints
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, err
	}
	for _, r := range c.EnvRunLists {
		c.RunList = append(c.RunList, r...)
	}
	return &c, nil
}

func (cg *ChefGuard) checkCookbookFrozen() (int, error) {
	if frozen, err := cg.cookbookFrozen(cg.Cookbook.Name, cg.Cookbook.Version); err != nil {
		return http.StatusBadGateway, err
	} else if frozen {
		return http.StatusPreconditionFailed, fmt.Errorf("\n=== Cookbook Upload error found ===\n" +
			"The cookbook you are trying to upload is frozen!\n" +
			"It is not allowed to overwrite a frozen cookbook,\n" +
			"so please bump the version and try again.\n" +
			"===================================\n")
	}
	return 0, nil
}

func (cg *ChefGuard) validateCookbookStatus() (int, error) {
	if cg.Cookbook.Metadata.Dependencies != nil {
		if errCode, err := cg.checkDependencies(parseCookbookVersions(cg.Cookbook.Metadata.Dependencies)); err != nil {
			if errCode == http.StatusPreconditionFailed {
				err = fmt.Errorf("\n=== Dependencies errors found ===\n"+
					"%s\n"+
					"=================================\n", err)
			}
			return errCode, err
		}
	}
	errCode, err := cg.searchSourceCookbook()
	if err != nil {
		if errCode == http.StatusPreconditionFailed {
			err = fmt.Errorf("\n=== Cookbook Compare errors found ===\n"+
				"%s\n"+
				"=====================================\n", err)
		}
		return errCode, err
	}
	if cg.SourceCookbook.artifact == false {
		if errCode, err := cg.executeChecks(); err != nil {
			return errCode, err
		}
	}
	if errCode, err := cg.compareCookbooks(); err != nil {
		if errCode == http.StatusPreconditionFailed {
			switch cg.SourceCookbook.LocationType {
			case "opscode":
				err = fmt.Errorf("\n=== Cookbook Compare errors found ===\n"+
					"%s\nSource: %s\n\n"+
					"Make sure you are using an unchanged community version\n"+
					"or, if you really need to change something, make a fork to\n"+
					"https://github.com and create a pull request back to the\n"+
					"community cookbook before trying to upload the cookbook again.\n"+
					"=====================================\n", err, cg.SourceCookbook.DownloadURL)
			case "github":
				err = fmt.Errorf("\n=== Cookbook Compare errors found ===\n"+
					"%s\nSource: %s\n\n"+
					"Make sure all your changes are merged into the central\n"+
					"repositories before trying to upload the cookbook again.\n"+
					"=====================================\n", err, cg.SourceCookbook.DownloadURL)
			default:
				err = fmt.Errorf("\n=== Cookbook Compare errors found ===\n"+
					"%s\nSource: %s\n"+
					"=====================================\n", err, cg.SourceCookbook.DownloadURL)
			}
		}
		return errCode, err
	}
	return 0, nil
}

func (cg *ChefGuard) validateConstraints(body []byte) (int, error) {
	c, err := unmarshalConstraints(body)
	if err != nil {
		return http.StatusBadGateway, fmt.Errorf("Failed to unmarshal body %s: %s", string(body), err)
	}
	if c.CookbookVersions != nil {
		if errCode, err := cg.checkDependencies(parseCookbookVersions(c.CookbookVersions)); err != nil {
			return errCode, err
		}
	}
	if c.RunList != nil {
		if errCode, err := cg.checkDependencies(parseRunlists(c.RunList)); err != nil {
			return errCode, err
		}
	}
	return 0, nil
}

func (cg *ChefGuard) checkDependencies(constrains map[string][]string) (int, error) {
	for name, versions := range constrains {
		for _, version := range versions {
			if frozen, err := cg.cookbookFrozen(name, version); err != nil {
				return http.StatusBadGateway, err
			} else if frozen == false {
				return http.StatusPreconditionFailed, fmt.Errorf("Your are depending on the %s cookbook version %s which isn't frozen! Please freeze the cookbook first before depending on it!", name, version)
			}
		}
	}
	return 0, nil
}

func (cg *ChefGuard) cookbookFrozen(name, version string) (bool, error) {
	cb, found, err := cg.chefClient.GetCookbookVersion(name, version)
	if err != nil {
		return true, fmt.Errorf("Failed to get info for cookbook %s version %s: %s", name, version, err)
	}
	if found == false {
		return false, nil
	}
	return cb.Frozen, nil
}

func (cg *ChefGuard) compareCookbooks() (int, error) {
	sh, err := getSourceFileHashes(cg.SourceCookbook)
	if err != nil {
		return http.StatusBadGateway, err
	}
	for file, fHash := range cg.FileHashes {
		if sHash, exists := sh[file]; exists {
			if file == "metadata.json" {
				delete(sh, file)
				continue
			}
			if fHash == sHash {
				delete(sh, file)
			} else {
				return http.StatusPreconditionFailed, fmt.Errorf("The file %s is changed!", file)
			}
		} else {
			return http.StatusPreconditionFailed, fmt.Errorf("There is a file missing: %s", file)
		}
	}
	if len(sh) > 0 {
		left := []string{}
		for file, _ := range sh {
			// This needs better code to check additional files!
			if file != "metadata.json" {
				left = append(left, file)
			}
		}
		if len(left) > 0 {
			return http.StatusPreconditionFailed, fmt.Errorf("There are new files added: %s", strings.Join(left, ","))
		}
	}
	return 0, nil
}

func (cg *ChefGuard) searchSourceCookbook() (int, error) {
	var err error
	cg.SourceCookbook, err = searchBerksAPI(cg.Cookbook.Name, cg.Cookbook.Version)
	if err != nil {
		return http.StatusBadGateway, err
	}
	if cg.SourceCookbook != nil {
		return 0, nil
	}
	if getEffectiveConfig("SearchGithub", cg.Organization).(bool) {
		cg.SourceCookbook, err = searchGithub(cg.Organization, cg.Cookbook.Name, cg.Cookbook.Version)
		if err != nil {
			return http.StatusBadGateway, err
		}
		if cg.SourceCookbook != nil {
			return 0, nil
		}
	}
	return http.StatusPreconditionFailed, fmt.Errorf("Failed to locate cookbook %s!", cg.Cookbook.Name)
}

func searchBerksAPI(name, version string) (*SourceCookbook, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s", cfg.BerksAPI.ServerURL, "universe"))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse the Berks API URL %s: %s", cfg.BerksAPI.ServerURL, err)
	}
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("Failed to get cookbook list from %s: %s", u.String(), err)
	}
	defer resp.Body.Close()
	if err := checkHTTPResponse(resp, []int{http.StatusOK}); err != nil {
		return nil, fmt.Errorf("Failed to get cookbook list from %s: %s", u.String(), err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read the response body from %v: %s", resp, err)
	}
	results := make(BerksResult)
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal body %s: %s", string(body), err)
	}
	if cb, exists := results[name]; exists {
		if sc, exists := cb[version]; exists {
			sc.artifact = true
			if sc.LocationType == "opscode" {
				u, err := communityDownloadUrl(sc.LocationPath, name, version)
				if err != nil {
					return nil, err
				}
				sc.DownloadURL = u
				return sc, nil
			}
		}
	}
	return nil, nil
}

func communityDownloadUrl(path, name, version string) (*url.URL, error) {
	u, err := url.Parse(fmt.Sprintf("%s/cookbooks/%s/versions/%s", path, name, strings.Replace(version, ".", "_", -1)))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse the cookbook URL %s: %s", fmt.Sprintf("%s/cookbooks/%s/versions/%s", path, name, strings.Replace(version, ".", "_", -1)), err)
	}
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("Failed to get cookbook info from %s: %s", u.String(), err)
	}
	defer resp.Body.Close()
	if err := checkHTTPResponse(resp, []int{http.StatusOK}); err != nil {
		return nil, fmt.Errorf("Failed to get cookbook info from %s: %s", u.String(), err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read the response body from %v: %s", resp, err)
	}
	sc := &SourceCookbook{}
	if err := json.Unmarshal(body, &sc); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal body %s: %s", string(body), err)
	}
	u, err = url.Parse(sc.File)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse the cookbook download URL %s: %s", sc.File, err)
	}
	return u, nil
}

func searchGithub(org, name, version string) (*SourceCookbook, error) {
	sc := &SourceCookbook{LocationType: "github"}
	for _, gitHubOrg := range cfg.Default.GitCookbookOrg {
		link, tagged, err := searchCookbookRepo(gitHubOrg, name, fmt.Sprintf("v%s", version))
		if err != nil {
			return nil, err
		}
		if link != nil {
			sc.artifact = false
			sc.tagged = tagged
			sc.gitHubOrg = gitHubOrg
			sc.DownloadURL = link
			return sc, nil
		}
	}
	if cfg.Chef.EnterpriseChef {
		if cust, found := cfg.Customer[org]; found {
			if cust.GitCookbookOrg != nil {
				link, tagged, err := searchCookbookRepo(*cust.GitCookbookOrg, name, fmt.Sprintf("v%s", version))
				if err != nil {
					return nil, err
				}
				if link != nil {
					sc.artifact = false
					sc.tagged = tagged
					sc.gitHubOrg = *cust.GitCookbookOrg
					sc.DownloadURL = link
					return sc, nil
				}
			}
		}
	}
	return nil, nil
}

func getSourceFileHashes(sc *SourceCookbook) (map[string][16]byte, error) {
	client, err := newDownloadClient(sc)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a new download client: %s", err)
	}
	resp, err := client.Get(sc.DownloadURL.String())
	if err != nil {
		return nil, fmt.Errorf("Failed to download the cookbook from %s: %s", sc.DownloadURL.String(), err)
	}
	defer resp.Body.Close()
	if err := checkHTTPResponse(resp, []int{http.StatusOK}); err != nil {
		return nil, fmt.Errorf("Failed to download the cookbook from %s: %s", sc.DownloadURL.String(), err)
	}
	var tr *tar.Reader
	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a new gzipReader: %s", err)
	}
	tr = tar.NewReader(gr)
	files := make(map[string][16]byte)
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("Failed to process all files: %s", err)
		}
		if header == nil {
			break
		}
		if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA {
			content, err := ioutil.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("Failed to process all files: %s", err)
			}
			files[strings.SplitN(header.Name, "/", 2)[1]] = md5.Sum(content)
		}
	}
	return files, nil
}

func newDownloadClient(sc *SourceCookbook) (*http.Client, error) {
	if sc.LocationType != "github" {
		return http.DefaultClient, nil
	}
	if _, found := cfg.Github[sc.gitHubOrg]; found == false {
		return nil, fmt.Errorf("No Github config specified for organization: %s!", sc.gitHubOrg)
	}
	t := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.Github[sc.gitHubOrg].SSLNoVerify},
	}
	return &http.Client{Transport: t}, nil
}

func parseCookbookVersions(constrains map[string]string) map[string][]string {
	re := regexp.MustCompile(`^= (\d+\.\d+\.\d+)$`)
	cbs := make(map[string][]string)
	for name, constrain := range constrains {
		if res := re.FindStringSubmatch(constrain); res != nil {
			version := res[1]
			cbs[name] = []string{version}
		}
	}
	return cbs
}

func parseRunlists(runlists []string) map[string][]string {
	re := regexp.MustCompile(`^.*\[(\w+).*@(\d+\.\d+\.\d+)\]$`)
	cbs := make(map[string][]string)
	for _, constrains := range runlists {
		if res := re.FindStringSubmatch(constrains); res != nil {
			name := res[1]
			version := res[2]
			if contains(cbs[name], version) == false {
				cbs[name] = append(cbs[name], version)
			}
		}
	}
	return cbs
}

func contains(versions []string, version string) bool {
	for _, v := range versions {
		if v == version {
			return true
		}
	}
	return false
}
