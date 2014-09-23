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
	"regexp"
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
		ValidateChanges string
		CommitChanges   bool
		MailChanges     bool
		SearchGithub    bool
		PublishCookbook bool
		Blacklist       string
		GitOrganization string
		GitCookbookOrgs string
		IncludeFCs      string
		ExcludeFCs      string
	}
	Customer map[string]*struct {
		Mode            *string
		MailDomain      *string
		MailServer      *string
		MailPort        *int
		MailRecipient   *string
		ValidateChanges *string
		CommitChanges   *bool
		MailChanges     *bool
		SearchGithub    *bool
		PublishCookbook *bool
		Blacklist       *string
		GitCookbookOrgs *string
		ExcludeFCs      *string
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
	Community struct {
		Supermarket string
		Forks       string
	}
	Supermarket struct {
		Server      string
		Port        string
		SSLNoVerify bool
		Version     string
		User        string
		Key         string
	}
	Graphite struct {
		Server string
		Port   int
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
}

var cfg Config

func loadConfig() error {
	exe, err := osext.Executable()
	if err != nil {
		return fmt.Errorf("Failed to get path of %s: %s", path.Base(os.Args[0]), err)
	}
	strings.TrimSuffix(exe, path.Ext(exe))
	var tmpConfig Config
	if err := gcfg.ReadFileInto(&tmpConfig, exe+".conf"); err != nil {
		return fmt.Errorf("Failed to parse config file '%s': %s", exe+".conf", err)
	}
	if err := verifyGithubTokens(); err != nil {
		return err
	}
	if err := verifyBlackLists(); err != nil {
		return err
	}
	if err := parsePaths(path.Dir(exe)); err != nil {
		return err
	}
	cfg = tmpConfig
	return nil
}

func verifyGithubTokens() error {
	for k, v := range cfg.Github {
		if v.Token == "" {
			return fmt.Errorf("No token found for Github organization %s! All configured organizations need to have a valid token.", k)
		}
	}
	return nil
}

func verifyBlackLists() error {
	rgx := strings.Split(cfg.Default.Blacklist, "|")
	for _, r := range rgx {
		if _, err := regexp.Compile(r); err != nil {
			return fmt.Errorf("The Default blacklist contains a bad regex: %s", err)
		}
	}
	for k, v := range cfg.Customer {
		if v.Blacklist != nil {
			rgx := strings.Split(*v.Blacklist, "|")
			for _, r := range rgx {
				if _, err := regexp.Compile(r); err != nil {
					return fmt.Errorf("The blacklist for customer %s contains a bad regex: %s", k, err)
				}
			}
		}
	}
	return nil
}

func parsePaths(ep string) error {
	if !path.IsAbs(cfg.Default.Logfile) {
		cfg.Default.Logfile = path.Join(ep, cfg.Default.Logfile)
	}
	if cfg.Tests.Foodcritic != "" && !path.IsAbs(cfg.Tests.Foodcritic) {
		cfg.Tests.Foodcritic = path.Join(ep, cfg.Tests.Foodcritic)
	}
	if cfg.Tests.Rubocop != "" && !path.IsAbs(cfg.Tests.Rubocop) {
		cfg.Tests.Rubocop = path.Join(ep, cfg.Tests.Rubocop)
	}
	return nil
}

func getEffectiveConfig(key, org string) interface{} {
	if cfg.Chef.EnterpriseChef {
		if c, found := cfg.Customer[org]; found {
			conf := reflect.ValueOf(c).Elem()
			v := conf.FieldByName(key)
			if !v.IsNil() {
				return v.Elem().Interface()
			}
		}
	}
	c := reflect.ValueOf(cfg.Default)
	return c.FieldByName(key).Interface()
}
