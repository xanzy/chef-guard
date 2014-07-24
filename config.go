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
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/xanzy/chef-guard/Godeps/_workspace/src/bitbucket.org/kardianos/osext"
	"github.com/xanzy/chef-guard/Godeps/_workspace/src/code.google.com/p/gcfg"
)

type Config struct {
	Default struct {
		Listen          string
		Logfile         string
		Tempdir         string
		Mode            string
		MailDomain      string
		MailServer      string
		MailPort        int
		MailRecipient   string
		ValidateChanges bool
		CommitChanges   bool
		MailChanges     bool
		SearchGithub    bool
		PublishCookbook bool
		GitOrganization string
		GitCookbookOrg  []string
	}
	Customer map[string]*struct {
		Mode            *string
		MailDomain      *string
		MailServer      *string
		MailPort        *int
		MailRecipient   *string
		ValidateChanges *bool
		CommitChanges   *bool
		MailChanges     *bool
		SearchGithub    *bool
		PublishCookbook *bool
		GitCookbookOrg  *string
	}
	Chef struct {
		EnterpriseChef bool
		Server         string
		Port           string
		SSLNoVerify    bool
		ErchefIP       string
		ErchefPort     int
		S3Key          string
		S3Secret       string
		Version        string
		User           string
		Key            string
	}
	Supermarket struct {
		Server      string
		Port        string
		SSLNoVerify bool
		Version     string
		User        string
		Key         string
	}
	BerksAPI struct {
		ServerURL string
	}
	Tests struct {
		Foodcritic string
		Rubocop    string
	}
	Github map[string]*struct {
		ServerURL   string
		SSLNoVerify bool
		Token       string
	}
	Graphite struct {
		Server string
		Port   int
	}
}

var cfg Config

func loadConfig() error {
	exe, err := osext.Executable()
	if err != nil {
		return fmt.Errorf("Failed to get path of %s: %s", path.Base(os.Args[0]), err)
	}
	strings.TrimSuffix(exe, path.Ext(exe))
	if err := gcfg.ReadFileInto(&cfg, exe+".conf"); err != nil {
		return fmt.Errorf("Failed to parse config file '%s': %s", exe+".conf", err)
	}
	if err := verifyGithubTokens(); err != nil {
		return err
	}
	return parsePaths(path.Dir(exe))
}

func verifyGithubTokens() error {
	for k, v := range cfg.Github {
		if v.Token == "" {
			return fmt.Errorf("No token found for Github organization %s! All configured organizations need to have a valid token.", k)
		}
	}
	return nil
}

func parsePaths(ep string) error {
	if path.IsAbs(cfg.Default.Logfile) == false {
		cfg.Default.Logfile = path.Join(ep, cfg.Default.Logfile)
	}
	if cfg.Tests.Foodcritic != "" && path.IsAbs(cfg.Tests.Foodcritic) == false {
		cfg.Tests.Foodcritic = path.Join(ep, cfg.Tests.Foodcritic)
	}
	if cfg.Tests.Rubocop != "" && path.IsAbs(cfg.Tests.Rubocop) == false {
		cfg.Tests.Rubocop = path.Join(ep, cfg.Tests.Rubocop)
	}
	return nil
}

func getEffectiveConfig(key, org string) interface{} {
	if cfg.Chef.EnterpriseChef {
		if c, found := cfg.Customer[org]; found {
			conf := reflect.ValueOf(c).Elem()
			v := conf.FieldByName(key)
			if v.IsNil() == false {
				return v.Elem().Interface()
			}
		}
	}
	c := reflect.ValueOf(cfg.Default)
	return c.FieldByName(key).Interface()
}
