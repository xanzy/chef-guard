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
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/xanzy/chef-guard/Godeps/_workspace/src/github.com/google/go-github/github"
	"github.com/xanzy/chef-guard/Godeps/_workspace/src/github.com/gorilla/mux"
	"github.com/xanzy/chef-guard/Godeps/_workspace/src/github.com/marpaia/chef-golang"
)

// The ChefGuard struct holds all required info needed to process a request made through Chef-Guard
type ChefGuard struct {
	smClient       *chef.Chef
	chefClient     *chef.Chef
	gitClient      *github.Client
	User           string
	Repo           string
	Organization   string
	OrganizationID *string
	Cookbook       *chef.CookbookVersion
	CookbookPath   string
	SourceCookbook *SourceCookbook
	ChangeDetails  *changeDetails
	ForcedUpload   bool
	FileHashes     map[string][16]byte
	TarFile        []byte
}

func newChefGuard(r *http.Request) (*ChefGuard, error) {
	cg := &ChefGuard{
		User:         r.Header.Get("X-Ops-Userid"),
		Organization: getOrgFromRequest(r),
		ForcedUpload: dropForce(r),
	}
	// Set the repo dependend on the Organization (could become a configurable in the future)
	if cg.Organization != "" {
		cg.Repo = cg.Organization
	} else {
		cg.Repo = "config"
	}
	// Initialize map for the file hashes
	cg.FileHashes = make(map[string][16]byte)
	// Setup chefClient
	var err error
	cg.chefClient, err = chef.ConnectBuilder(cfg.Chef.Server, cfg.Chef.Port, cfg.Chef.Version, cfg.Chef.User, cfg.Chef.Key, cg.Organization)
	if err != nil {
		return nil, fmt.Errorf("Failed to create new Chef API connection: %s", err)
	}
	cg.chefClient.SSLNoVerify = cfg.Chef.SSLNoVerify
	return cg, nil
}

func main() {
	// Load and parse the config file
	if err := loadConfig(); err != nil {
		log.Fatal(err)
	}
	// Initialize logging
	if err := initLogging(); err != nil {
		log.Fatal(err)
	}
	// Parse the ErChef API URL
	u, err := url.Parse(fmt.Sprintf("http://%s:%d", cfg.Chef.ErchefIP, cfg.Chef.ErchefPort))
	if err != nil {
		log.Fatal(fmt.Errorf("Failed to parse ErChef API URL %s: %s", fmt.Sprintf("http://%s:%d", cfg.Chef.ErchefIP, cfg.Chef.ErchefPort), err))
	}
	// All critical parts are started now, so let's log a 'started' message :)
	INFO.Println("Server started...")

	// Initialize Graphite connection
	initGraphite()

	// Setup the ErChef proxy
	p := httputil.NewSingleHostReverseProxy(u)

	// Configure all needed handlers
	rtr := mux.NewRouter()
	if cfg.Chef.EnterpriseChef {
		rtr.Path("/organizations/{org}/{type:data}/{bag}").HandlerFunc(processChange(p)).Methods("POST")
		rtr.Path("/organizations/{org}/{type:data}/{bag}/{name}").HandlerFunc(processChange(p)).Methods("PUT", "DELETE")
		rtr.Path("/organizations/{org}/{type:environments|nodes|roles}").HandlerFunc(processChange(p)).Methods("POST")
		rtr.Path("/organizations/{org}/{type:environments|nodes|roles}/{name}").HandlerFunc(processChange(p)).Methods("PUT", "DELETE")
		rtr.Path("/organizations/{org}/{type:cookbooks}/{name}/{version}").HandlerFunc(processCookbook(p)).Methods("PUT", "DELETE")
	} else {
		rtr.Path("/{type:data}/{bag}").HandlerFunc(processChange(p)).Methods("POST")
		rtr.Path("/{type:data}/{bag}/{name}").HandlerFunc(processChange(p)).Methods("PUT", "DELETE")
		rtr.Path("/{type:environments|nodes|roles}").HandlerFunc(processChange(p)).Methods("POST")
		rtr.Path("/{type:environments|nodes|roles}/{name}").HandlerFunc(processChange(p)).Methods("PUT", "DELETE")
		rtr.Path("/{type:cookbooks}/{name}/{version}").HandlerFunc(processCookbook(p)).Methods("PUT", "DELETE")
	}
	rtr.NotFoundHandler = p
	http.Handle("/", rtr)

	// Start the server
	startExitHandler()
	err = http.ListenAndServe(fmt.Sprintf("%s:%d", cfg.Default.Listen, cfg.Chef.ErchefPort), nil)
	if err != nil {
		e := fmt.Errorf("Failed to start Chef-Guard server on port %d: %s", cfg.Chef.ErchefPort, err)
		ERROR.Println(e)
		log.Fatal(e)
	}
}

func startExitHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for _ = range c {
			fmt.Println("Waiting for all connections to end before stopping the server...")
			INFO.Println("Server stopped...")
			os.Exit(0)
		}
	}()
}

func errorHandler(w http.ResponseWriter, err string, statusCode int) {
	if statusCode == http.StatusPreconditionFailed {
		WARNING.Println(err)
	} else {
		ERROR.Println(err)
	}
	http.Error(w, err, statusCode)
}

func getOrgFromRequest(r *http.Request) string {
	if cfg.Chef.EnterpriseChef == false {
		return ""
	}
	return mux.Vars(r)["org"]
}

func dropForce(r *http.Request) bool {
	v := r.URL.Query()
	if _, exists := v["force"]; exists {
		v.Del("force")
		r.URL.RawQuery = v.Encode()
		return true
	}
	return false
}
