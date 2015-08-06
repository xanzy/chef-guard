//
// Copyright 2015, Sander van Harmelen
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

package git

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"crypto/tls"

	"code.google.com/p/goauth2/oauth"
	"github.com/google/go-github/github"
	"github.com/xanzy/go-gitlab"
)

// Git is an interface that must be implemented by any git service
// that can be used with Chef-Guard
type Git interface {
	// GetContents retrieves file and/or directory contents from git
	GetContent(string, string, string) (*File, interface{}, error)

	// CreateFile creates a new repository file
	CreateFile(string, string, string, string, *User, []byte) (string, error)

	// UpdateFile updates a repository file
	UpdateFile(string, string, string, string, string, *User, []byte) (string, error)

	// DeleteFile deletes a repository file
	DeleteFile(string, string, string, string, string, *User) (string, error)

	// DeleteDirectory deletes a repository directory including all content
	DeleteDirectory(string, string, string, interface{}, *User) error

	// GetDiff returns the diff and committer details
	GetDiff(string, string, string, string) (string, error)

	// GetArchiveLink returns a download link for the repo/tag combo
	GetArchiveLink(string, string, string) (*url.URL, error)

	// TagRepo creates a new tag on a project
	TagRepo(string, string, string, *User) error

	// TagExists returns true if the tag exists
	TagExists(string, string, string) (bool, error)

	// UntagRepo removes a new tag from a project
	UntagRepo(string, string, string) error
}

// User represents the user that is making the change
type User struct {
	Name string
	Mail string
}

// File represents a single file and it's the user that is making the change
type File struct {
	Content string
	Path    string
	SHA     string
}

// Config represents the configuration of a git service
type Config struct {
	Type        string
	ServerURL   string
	SSLNoVerify bool
	Token       string
}

// GitHub represents a GitHub client
type GitHub struct {
	client *github.Client
}

// GitLab represents a GitLab client
type GitLab struct {
	client *gitlab.Client
}

// NewGitClient returns either a GitHub or GitLab client as Git interface
func NewGitClient(c *Config) (Git, error) {
	switch c.Type {
	case "github":
		return newGitHubClient(c)
	case "gitlab":
		return newGitLabClient(c)
	default:
		return nil, fmt.Errorf("Unknown Git type: %q", c.Type)
	}
}

func newGitHubClient(c *Config) (Git, error) {
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: c.Token},
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.SSLNoVerify},
		},
	}

	g := new(GitHub)
	g.client = github.NewClient(t.Client())

	if c.ServerURL != "" {
		// Make sure the URL ends with a single forward slash as the go-github package requires that
		u, err := url.Parse(strings.Trim(c.ServerURL, "/") + "/")
		if err != nil {
			return nil, fmt.Errorf("Failed to parse Github server URL %s: %s", c.ServerURL, err)
		}

		g.client.BaseURL = u
	}

	return g, nil
}

func newGitLabClient(c *Config) (Git, error) {
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.SSLNoVerify},
		},
	}

	g := new(GitLab)
	g.client = gitlab.NewClient(client, c.Token)

	if c.ServerURL != "" {
		g.client.SetBaseURL(c.ServerURL)
	}

	return g, nil
}
