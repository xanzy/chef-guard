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
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/xanzy/chef-guard/Godeps/_workspace/src/github.com/gorilla/mux"
)

// Add files type and functions for the Sort interface
type files []string

func (f files) Len() int {
	return len(f)
}

func (f files) Less(i, j int) bool {
	re := regexp.MustCompile(`.*?(\d+)\.(\d+)\.(\d+)-?(\d+)?.*`)
	parts_i := re.FindStringSubmatch(f[i])
	parts_j := re.FindStringSubmatch(f[j])

	for idx := 1; idx < len(parts_i); idx++ {
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
		// If equal, move on to the next part
		if i == j {
			continue
		}
		// Compare and do a descending sort
		if i > j {
			return true
		}
		return false
	}
	return false
}

func (f files) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func processDownload(w http.ResponseWriter, r *http.Request) {
	path := getFilePath(r)
	dir := filepath.Join(cfg.ChefClients.Path, path)

	targetfile, err := getTargetFile(dir, r.FormValue("v"))
	if err != nil {
		errorHandler(w, err.Error(), http.StatusBadGateway)
	}

	if targetfile != "" {
		targetpath := path + targetfile[len(dir):]
		targeturl := getChefBaseURL() + "/chef-guard/clients/" + targetpath

		// For download calls, redirect to the actuall file
		if mux.Vars(r)["type"] == "download" {
			http.Redirect(w, r, targeturl, http.StatusFound)
		}
		// For metadata calls, return the requested meta data
		if mux.Vars(r)["type"] == "metadata" {
			data, err := ioutil.ReadFile(targetfile)
			if err != nil {
				errorHandler(w, "Failed to read client file: %s"+err.Error(), http.StatusBadGateway)
			}

			targetmd5 := md5.Sum(data)
			targetsha := sha256.Sum256(data)
			data = nil

			fmt.Fprintf(w, "url %s md5 %x sha256 %x", targeturl, targetmd5, targetsha)
		}
	}
}

func getFilePath(r *http.Request) string {
	return filepath.Join(r.FormValue("p"), r.FormValue("pv"), r.FormValue("m"))
}

func getTargetFile(dir, version string) (string, error) {
	if version == "latest" {
		version = "."
	}

	filelist, err := filepath.Glob(dir + "/*" + version + "*")
	if err != nil {
		return "", fmt.Errorf("Failed to read clients from disk: %s", err)
	}

	if filelist != nil {
		sort.Sort(files(filelist))
		return filelist[0], nil
	}
	return "", nil
}
