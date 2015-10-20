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

	"github.com/gorilla/mux"
)

type Name struct {
	Name    string `json:"name"`
	RawData struct {
		ID string `json:"id"`
	} `json:"raw_data"`
}

func unmarshalName(body []byte) (*Name, error) {
	var n Name
	if err := json.Unmarshal(body, &n); err != nil {
		return nil, err
	}
	// Needed to get the correct name from data bag items
	if n.RawData.ID != "" {
		n.Name = n.RawData.ID
	}
	return &n, nil
}

func processChange(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cg, err := newChefGuard(r)
		if err != nil {
			errorHandler(w, fmt.Sprintf(
				"Failed to create a new ChefGuard structure: %s", err), http.StatusBadGateway)
			return
		}

		reqBody, err := dumpBody(r)
		if err != nil {
			errorHandler(w, fmt.Sprintf(
				"Failed to get body from call to %s: %s", r.URL.String(), err), http.StatusBadGateway)
			return
		}

		if getEffectiveConfig("ValidateChanges", cg.Organization).(string) == "enforced" &&
			r.Method != "DELETE" {
			if errCode, err := cg.validateConstraints(reqBody); err != nil {
				errorHandler(w, err.Error(), errCode)
				return
			}
		}

		// So, this is kind of an ugly one...
		// 1. If we don't want to commit any changes, just return here.
		// 2. If we do want to commit the changes, but we are a node updating itself also return here
		if getEffectiveConfig("CommitChanges", cg.Organization).(bool) == false ||
			(strings.HasPrefix(r.Header.Get("User-Agent"), "Chef Client") && r.Header.Get("X-Ops-Request-Source") != "web") {
			p.ServeHTTP(w, r)
			return
		}

		r.URL, err = url.Parse(fmt.Sprintf(
			"http://%s:%d%s?%s", cfg.Chef.ErchefIP, cfg.Chef.ErchefPort, r.URL.Path, r.URL.RawQuery))
		if err != nil {
			errorHandler(w, fmt.Sprintf(
				"Failed to parse URL %s: %s", fmt.Sprintf(
					"http://%s:%d%s?%s",
					cfg.Chef.ErchefIP,
					cfg.Chef.ErchefPort,
					r.URL.Path,
					r.URL.RawQuery), err), http.StatusBadGateway)
			return
		}

		resp, err := http.DefaultTransport.RoundTrip(r)
		if err != nil {
			errorHandler(w, fmt.Sprintf(
				"Call to %s failed: %s", r.URL.String(), err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			errorHandler(w, fmt.Sprintf(
				"Failed to get body from call to %s: %s", r.URL.String(), err), http.StatusBadGateway)
			return
		}

		if err := checkHTTPResponse(resp, []int{http.StatusOK, http.StatusCreated}); err != nil {
			errorHandler(w, err.Error(), resp.StatusCode)
			return
		}

		cg.ChangeDetails, err = getChangeDetails(r, reqBody)
		if err != nil {
			errorHandler(w, fmt.Sprintf(
				"Failed to parse variables from %s: %s", r.URL.String(), err), http.StatusBadGateway)
			return
		}

		if r.Method == "PUT" {
			go cg.syncedGitUpdate(r.Method, respBody)
		} else {
			go cg.syncedGitUpdate(r.Method, reqBody)
		}

		if getEffectiveConfig("ValidateChanges", cg.Organization).(string) == "permissive" &&
			r.Method != "DELETE" {
			if errCode, err := cg.validateConstraints(reqBody); err != nil {
				errorHandler(w, err.Error(), errCode)
				return
			}
		}

		copyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	}
}

type changeDetails struct {
	Item string
	Type string
}

func getChangeDetails(r *http.Request, body []byte) (*changeDetails, error) {
	cd := &changeDetails{}
	v := mux.Vars(r)
	// Resolve the name either directly or by unmarshalling the request body
	if _, found := v["name"]; found {
		cd.Item = fmt.Sprintf("%s.json", v["name"])
	} else {
		if len(body) > 0 {
			n, err := unmarshalName(body)
			if err != nil {
				return nil, err
			}
			cd.Item = fmt.Sprintf("%s.json", n.Name)
		}
	}
	// When changing data bags, the name of the bag should also be in cd.Item
	// and the type should be set to "data_bags"
	if _, found := v["bag"]; found {
		// If no item is found by now, set the item to the whole bag instead of a single item
		if cd.Item == "" {
			cd.Item = fmt.Sprintf("%s", v["bag"])
		} else {
			cd.Item = fmt.Sprintf("%s/%s", v["bag"], cd.Item)
		}
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
