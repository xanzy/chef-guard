//
// Copyright 2014, Sander Botman
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
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// Add type and functions for the Sort interface
type files []string

func (f files) Len() int {
	return len(f)
}

func (f files) Less(i, j int) bool {
	re := regexp.MustCompile(`.*?(\d+)\.(\d+)\.(\d+)-?(\d+)?.*`)
	parts_i := re.FindStringSubmatch(f[i])
	parts_j := re.FindStringSubmatch(f[j])

	for idx := 1; i < len(parts_i); i++ {
		// Convert the part to a int
		i, err := strconv.Atoi(parts_i[idx])
		if err != nil {
			return false
		}
		// Convert the part to a int
		j, err := strconv.Atoi(parts_j[idx])
		if err != nil {
			return false
		}
		// Compare and do a descending sort
		if j < i {
			return true
		}
	}
	return false
}

func (f files) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func getURLParams(r *http.Request) string {
	return filepath.Join(r.FormValue("p"), r.FormValue("pv"), r.FormValue("m"))
}

func getTargetDir(r *http.Request) string {
	params := getURLParams(r)

	if cfg.ChefClients.Path != "" {
		params = filepath.Join(cfg.ChefClients.Path, params)
	}
	return params
}

func getTargetFile(r *http.Request) string {
	dir := getTargetDir(r)

	version := r.FormValue("v")
	if version == "latest" {
		version = "."
	}

	filelist, _ := filepath.Glob(dir + "/*" + version + "*")

	if filelist != nil {
		sort.Sort(files(filelist))
		return filelist[0]
	} else {
		return ""
	}
}

func getTargetURL(target string) string {
	host := cfg.Chef.Server
	switch cfg.Chef.Port {
	case "443":
		host = "https://" + host
	case "80":
		host = "http://" + host
	case "":
		host = "http://" + host
	default:
		host = "http://" + host + ":" + cfg.Chef.Port
	}

	if cfg.Chef.EnterpriseChef {
		// BASEURL needs to be fixed
		return host + "/organizations/chef-guard/clients/" + target
	} else {
		return host + "/chef-guard/clients/" + target
	}
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	path := getURLParams(r)
	dir := getTargetDir(r)

	targetfile := getTargetFile(r)

	if targetfile != "" {
		targetpath := path + targetfile[len(dir):]

		targeturl := getTargetURL(targetpath)
		http.Redirect(w, r, targeturl, http.StatusFound)
	}
}

func metadataHandler(w http.ResponseWriter, r *http.Request) {
	path := getURLParams(r)
	dir := getTargetDir(r)

	targetfile := getTargetFile(r)

	if targetfile != "" {
		targetpath := path + targetfile[len(dir):]
		targeturl := getTargetURL(targetpath)

		data, err := ioutil.ReadFile(targetfile)
		if err != nil {
			log.Fatal(err)
		}

		targetmd5 := md5.Sum(data)
		targetsha := sha256.Sum256(data)
		data = nil

		fmt.Fprintf(w, "url %s md5 %x sha256 %x", targeturl, targetmd5, targetsha)
	}
}
