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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/xanzy/chef-guard/Godeps/_workspace/src/github.com/gorilla/mux"
)

type changeDetails struct {
	Item string
	Type string
}

type Name struct {
	Name    string `json:"name"`
	RawData struct {
		Id string `json:"id"`
	} `json:"raw_data"`
}

func unmarshalName(body []byte) (*Name, error) {
	n := new(Name)
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, err
	}
	// Needed to get the correct name from data bag items
	if n.RawData.Id != "" {
		n.Name = n.RawData.Id
	}
	return n, nil
}

func processChange(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cg, err := newChefGuard(r)
		if err != nil {
			errorHandler(w, fmt.Sprintf("Failed to create a new ChefGuard structure: %s", err), http.StatusBadGateway)
			return
		}
		reqBody, err := dumpBody(r)
		if err != nil {
			errorHandler(w, fmt.Sprintf("Failed to get body from call to %s: %s", r.URL.String(), err), http.StatusBadGateway)
			return
		}
		if getEffectiveConfig("ValidateChanges", cg.Organization).(bool) && r.Method != "DELETE" {
			if errCode, err := cg.validateConstraints(reqBody); err != nil {
				errorHandler(w, err.Error(), errCode)
				return
			}
		}
		if getEffectiveConfig("CommitChanges", cg.Organization).(bool) == false || (strings.HasPrefix(r.Header.Get("User-Agent"), "Chef Client") && r.Header.Get("X-Ops-Request-Source") != "web") {
			p.ServeHTTP(w, r)
			return
		}
		r.URL, err = url.Parse(fmt.Sprintf("http://%s:%d%s?%s", cfg.Chef.ErchefIP, cfg.Chef.ErchefPort, r.URL.Path, r.URL.RawQuery))
		if err != nil {
			errorHandler(w, fmt.Sprintf("Failed to parse URL %s: %s", fmt.Sprintf("http://%s:%d%s?%s", cfg.Chef.ErchefIP, cfg.Chef.ErchefPort, r.URL.Path, r.URL.RawQuery), err), http.StatusBadGateway)
			return
		}
		resp, err := http.DefaultTransport.RoundTrip(r)
		if err != nil {
			errorHandler(w, fmt.Sprintf("Call to %s failed: %s", r.URL.String(), err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errorHandler(w, fmt.Sprintf("Failed to get body from call to %s: %s", r.URL.String(), err), http.StatusBadGateway)
			return
		}
		if err := checkHTTPResponse(resp, []int{http.StatusOK, http.StatusCreated}); err == nil {
			cg.ChangeDetails, err = getChangeDetails(r, reqBody)
			if err != nil {
				errorHandler(w, fmt.Sprintf("Failed to parse variables from %s: %s", r.URL.String(), err), http.StatusBadGateway)
				return
			}
			if r.Method == "PUT" {
				go cg.syncedGitUpdate(r.Method, respBody)
			} else {
				go cg.syncedGitUpdate(r.Method, reqBody)
			}
		}
		copyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	}
}

func getChangeDetails(r *http.Request, body []byte) (*changeDetails, error) {
	cd := &changeDetails{}
	v := mux.Vars(r)
	// Resolve the name either directly or by unmarshalling the request body
	if _, found := v["name"]; found {
		cd.Item = v["name"]
	} else {
		n, err := unmarshalName(body)
		if err != nil {
			return nil, err
		}
		cd.Item = n.Name
	}
	// When changing data bags, the name of the bag should also be in cg.Vars["name"]
	// and the type should be set to "data_bags"
	if _, found := v["bag"]; found {
		cd.Item = fmt.Sprintf("%s/%s", v["bag"], cd.Item)
		cd.Type = "data_bags"
	} else {
		cd.Type = v["type"]
	}
	return cd, nil
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		if key == "Content-Length" {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
