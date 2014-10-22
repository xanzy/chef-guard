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
		SaveChefMetrics bool
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
		SaveChefMetrics *bool
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
	ChefClients struct {
		Path string
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
	MongoDB struct {
		Server     string
		Database   string
		Collection string
		User       string
		Password   string
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
	if err := verifyRequiredFields(&tmpConfig); err != nil {
		return err
	}
	if err := verifyGithubTokens(&tmpConfig); err != nil {
		return err
	}
	if err := verifyBlackLists(&tmpConfig); err != nil {
		return err
	}
	if err := parsePaths(&tmpConfig, path.Dir(exe)); err != nil {
		return err
	}
	cfg = tmpConfig
	return nil
}

func verifyRequiredFields(c *Config) error {
	r := map[string]interface{}{
		"Default->Listen":          c.Default.Listen,
		"Default->Logfile":         c.Default.Logfile,
		"Default->Tempdir":         c.Default.Tempdir,
		"Default->Mode":            c.Default.Mode,
		"Default->ValidateChanges": c.Default.ValidateChanges,
		"Chef->Server":             c.Chef.Server,
		"Chef->Port":               c.Chef.Port,
		"Chef->ErchefIP":           c.Chef.ErchefIP,
		"Chef->ErchefPort":         c.Chef.ErchefPort,
		"Chef->S3Key":              c.Chef.S3Key,
		"Chef->S3Secret":           c.Chef.S3Secret,
		"Chef->Version":            c.Chef.Version,
		"Chef->User":               c.Chef.User,
		"Chef->Key":                c.Chef.Key,
		"Community->Supermarket":   c.Community.Supermarket,
	}

	if c.Default.MailChanges {
		r["Default->MailServer"] = c.Default.MailServer
		r["Default->MailPort"] = c.Default.MailPort
		r["Default->MailRecipient"] = c.Default.MailRecipient
	}

	if c.Default.CommitChanges {
		r["Default->GitOrganization"] = c.Default.GitOrganization
	}

	if c.Default.SearchGithub {
		r["Default->GitCookbookOrgs"] = c.Default.GitCookbookOrgs
	}

	if c.Default.SaveChefMetrics {
		r["MongoDB->Server"] = c.MongoDB.Server
		r["MongoDB->Database"] = c.MongoDB.Database
		r["MongoDB->Collection"] = c.MongoDB.Collection
		r["MongoDB->User"] = c.MongoDB.User
		r["MongoDB->Password"] = c.MongoDB.Password
	}

	if c.Default.PublishCookbook {
		r["Supermarket->Server"] = c.Supermarket.Server
		r["Supermarket->Port"] = c.Supermarket.Port
		r["Supermarket->Version"] = c.Supermarket.Version
		r["Supermarket->User"] = c.Supermarket.User
		r["Supermarket->Key"] = c.Supermarket.Key
	}

	for k, v := range r {
		switch v := v.(type) {
		case int:
			if v == 0 {
				return fmt.Errorf("Required configuration value missing for Section->Key: %s", k)
			}
		case string:
			if v == "" {
				return fmt.Errorf("Required configuration value missing for Section->Key: %s", k)
			}
		}
	}

	return nil
}

func verifyGithubTokens(c *Config) error {
	for k, v := range c.Github {
		if v.Token == "" {
			return fmt.Errorf("No token found for Github organization %s! All configured organizations need to have a valid token.", k)
		}
	}
	return nil
}

func verifyBlackLists(c *Config) error {
	rgx := strings.Split(c.Default.Blacklist, "|")
	for _, r := range rgx {
		if _, err := regexp.Compile(r); err != nil {
			return fmt.Errorf("The Default blacklist contains a bad regex: %s", err)
		}
	}
	for k, v := range c.Customer {
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

func parsePaths(c *Config, ep string) error {
	if !path.IsAbs(c.Default.Logfile) {
		c.Default.Logfile = path.Join(ep, c.Default.Logfile)
	}
	if c.Tests.Foodcritic != "" && !path.IsAbs(c.Tests.Foodcritic) {
		c.Tests.Foodcritic = path.Join(ep, c.Tests.Foodcritic)
	}
	if c.Tests.Rubocop != "" && !path.IsAbs(c.Tests.Rubocop) {
		c.Tests.Rubocop = path.Join(ep, c.Tests.Rubocop)
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
