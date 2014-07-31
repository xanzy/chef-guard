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
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"

	"github.com/xanzy/chef-guard/Godeps/_workspace/src/github.com/marpaia/chef-golang"
)

func setupSMClient() (*chef.Chef, error) {
	smClient, err := chef.ConnectBuilder(cfg.Supermarket.Server, cfg.Supermarket.Port, cfg.Supermarket.Version, cfg.Supermarket.User, cfg.Supermarket.Key, "")
	if err != nil {
		return nil, fmt.Errorf("Failed to create new Supermarket API connection: %s", err)
	}
	smClient.SSLNoVerify = cfg.Supermarket.SSLNoVerify
	return smClient, nil
}

func (cg *ChefGuard) publishCookbook() error {
	if blackListed(cg.Organization, cg.Cookbook.Name) {
		return nil
	}
	var err error
	if cg.smClient == nil {
		if cg.smClient, err = setupSMClient(); err != nil {
			return err
		}
	}
	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)
	fw, err := mw.CreateFormFile("tarball", fmt.Sprintf("%s.tgz", cg.Cookbook.Name))
	if err != nil {
		return fmt.Errorf("Failed to create form file: %s", err)
	}
	if _, err = fw.Write(cg.TarFile); err != nil {
		return fmt.Errorf("Failed to add tar archive to the request: %s", err)
	}
	if fw, err = mw.CreateFormField("cookbook"); err != nil {
		return fmt.Errorf("Failed to create form field: %s", err)
	}
	if _, err = fw.Write([]byte(`{"category":"other"}`)); err != nil {
		return fmt.Errorf("Failed to add category to the request: %s", err)
	}
	mw.Close()

	resp, err := cg.smClient.Post("api/v1/cookbooks", mw.FormDataContentType(), nil, buf)
	if err != nil {
		return fmt.Errorf("Failed to upload %s to the Supermarket: %s", cg.Cookbook.Name, err)
	}
	if err := checkHTTPResponse(resp, []int{http.StatusCreated}); err != nil {
		return fmt.Errorf("Failed to upload %s to the Supermarket: %s", cg.Cookbook.Name, err)
	}
	return nil
}

func blackListed(org, cookbook string) bool {
	blacklist := cfg.Default.Blacklist
	custBL := getEffectiveConfig("Blacklist", org)
	if blacklist != custBL {
		blacklist = fmt.Sprintf("%s,%s", blacklist, custBL)
	}
	rgx := strings.Split(blacklist, ",")
	for _, r := range rgx {
		re, _ := regexp.Compile(strings.TrimSpace(r))
		if re.MatchString(cookbook) {
			return true
		}
	}
	return false
}
