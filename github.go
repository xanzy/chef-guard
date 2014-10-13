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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"time"

	"github.com/xanzy/chef-guard/Godeps/_workspace/src/code.google.com/p/goauth2/oauth"
	"github.com/xanzy/chef-guard/Godeps/_workspace/src/github.com/google/go-github/github"
	"github.com/xanzy/chef-guard/multisyncer"
)

var ms multisyncer.MultiSyncer

func remarshalConfig(action string, data []byte) ([]byte, error) {
	// If the action is DELETE, there is no body to remarshal
	if action == "DELETE" {
		data = append(data, []byte("\n")...)
		return data, nil
	}
	config := make(map[string]interface{})
	err := json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	if _, found := config["automatic"]; found {
		delete(config, "automatic")
	}
	c, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}
	c = append(c, []byte("\n")...)
	return DecodeMarshalledJSON(c), nil
}

func DecodeMarshalledJSON(b []byte) []byte {
	r := strings.NewReplacer(`\u003c`, `<`, `\u003e`, `>`, `\u0026`, `&`)
	s := r.Replace(string(b))
	return []byte(s)
}

func setupGitClient() (*github.Client, error) {
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: cfg.Github[cfg.Default.GitOrganization].Token},
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.Github[cfg.Default.GitOrganization].SSLNoVerify},
		},
	}
	gitClient := github.NewClient(t.Client())
	if cfg.Github[cfg.Default.GitOrganization].ServerURL != "" {
		u, err := url.Parse(cfg.Github[cfg.Default.GitOrganization].ServerURL)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse Github server URL %s: %s", cfg.Github[cfg.Default.GitOrganization].ServerURL, err)
		}
		gitClient.BaseURL = u
	}
	return gitClient, nil
}

func (cg *ChefGuard) syncedGitUpdate(action string, body []byte) {
	if ms == nil {
		ms = multisyncer.New()
	}
	token := <-ms.GetToken(cg.Repo)
	defer func() {
		ms.ReturnToken(cg.Repo) <- token
	}()
	config, err := remarshalConfig(action, body)
	if err != nil {
		itemType := strings.TrimSuffix(cg.ChangeDetails.Type, "s")
		ERROR.Printf("Failed to convert %s config for %s %s for %s: %s", itemType, itemType, strings.TrimSuffix(cg.ChangeDetails.Item, ".json"), cg.User, err)
		return
	}
	if resp, err := cg.writeConfigToGit(action, config); err != nil {
		itemType := strings.TrimSuffix(cg.ChangeDetails.Type, "s")
		ERROR.Printf("Failed to update %s %s for %s in Github: %s", strings.TrimSuffix(cg.ChangeDetails.Item, ".json"), itemType, cg.User, err)
		return
	} else {
		if err := cg.mailChanges(fmt.Sprintf("%s/%s", cg.ChangeDetails.Type, cg.ChangeDetails.Item), *resp.Commit.SHA, action); err != nil {
			ERROR.Printf("Failed to send git spam: %s", err)
			return
		}
	}
	return
}

func (cg *ChefGuard) writeConfigToGit(action string, config []byte) (*github.RepositoryContentResponse, error) {
	var err error
	if cg.gitClient == nil {
		if cg.gitClient, err = setupGitClient(); err != nil {
			return nil, fmt.Errorf("Failed to create Git client: %s", err)
		}
	}
	r := &github.RepositoryContentResponse{}
	mail := fmt.Sprintf("%s@%s", cg.User, getEffectiveConfig("MailDomain", cg.Organization).(string))
	opts := &github.RepositoryContentFileOptions{}
	opts.Committer = &github.CommitAuthor{Name: &cg.User, Email: &mail}
	opts.Content = config

	file, dir, resp, err := cg.gitClient.Repositories.GetContents(cfg.Default.GitOrganization, cg.Repo, fmt.Sprintf("%s/%s", cg.ChangeDetails.Type, cg.ChangeDetails.Item), nil)
	if resp != nil && resp.Response.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("The token configured for Github organization %s is not valid!", cfg.Default.GitOrganization)
	}
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			if action == "DELETE" {
				return nil, fmt.Errorf("Failed to delete non-existing file or directory %s/%s", cg.ChangeDetails.Type, cg.ChangeDetails.Item)
			} else {
				msg := fmt.Sprintf("Config for %s %s created by Chef-Guard", strings.TrimSuffix(cg.ChangeDetails.Item, ".json"), strings.TrimSuffix(cg.ChangeDetails.Type, "s"))
				opts.Message = &msg
				if r, _, err = cg.gitClient.Repositories.CreateFile(cfg.Default.GitOrganization, cg.Repo, fmt.Sprintf("%s/%s", cg.ChangeDetails.Type, cg.ChangeDetails.Item), opts); err != nil {
					return nil, err
				}
			}
		} else {
			return nil, err
		}
	} else {
		if file != nil {
			opts.SHA = file.SHA
			if action == "DELETE" {
				msg := fmt.Sprintf("Config for %s %s deleted by Chef-Guard", strings.TrimSuffix(cg.ChangeDetails.Item, ".json"), strings.TrimSuffix(cg.ChangeDetails.Type, "s"))
				opts.Message = &msg
				if r, _, err = cg.gitClient.Repositories.DeleteFile(cfg.Default.GitOrganization, cg.Repo, fmt.Sprintf("%s/%s", cg.ChangeDetails.Type, cg.ChangeDetails.Item), opts); err != nil {
					return nil, err
				}
			} else {
				msg := fmt.Sprintf("Config for %s %s updated by Chef-Guard", strings.TrimSuffix(cg.ChangeDetails.Item, ".json"), strings.TrimSuffix(cg.ChangeDetails.Type, "s"))
				opts.Message = &msg
				if r, _, err = cg.gitClient.Repositories.UpdateFile(cfg.Default.GitOrganization, cg.Repo, fmt.Sprintf("%s/%s", cg.ChangeDetails.Type, cg.ChangeDetails.Item), opts); err != nil {
					return nil, err
				}
			}
		}
		if dir != nil && action == "DELETE" {
			for _, file := range dir {
				opts.SHA = file.SHA
				msg := fmt.Sprintf("Config for %s %s deleted by Chef-Guard", strings.TrimSuffix(*file.Name, ".json"), strings.TrimSuffix(cg.ChangeDetails.Type, "s"))
				opts.Message = &msg
				if r, _, err = cg.gitClient.Repositories.DeleteFile(cfg.Default.GitOrganization, cg.Repo, fmt.Sprintf("%s/%s", cg.ChangeDetails.Type, file.Name), opts); err != nil {
					return nil, err
				}
			}
		}

		return nil, fmt.Errorf("Unknown error while getting file or directory content of %s/%s", cg.ChangeDetails.Type, cg.ChangeDetails.Item)
	}
	return r, nil
}

func (cg *ChefGuard) mailChanges(file, sha, action string) error {
	if getEffectiveConfig("MailChanges", cg.Organization).(bool) == false {
		return nil
	}
	diff, commit, err := cg.getDiff(sha)
	if commit == nil || err != nil {
		return err
	}
	var subject string
	switch action {
	case "POST":
		subject = fmt.Sprintf("[%s CHEF] created %s", strings.ToUpper(cg.Organization), file)
	case "PUT":
		subject = fmt.Sprintf("[%s CHEF] updated %s", strings.ToUpper(cg.Organization), file)
	case "DELETE":
		subject = fmt.Sprintf("[%s CHEF] deleted %s", strings.ToUpper(cg.Organization), file)
	}
	msg := createMessage(cg.Repo, *commit.Commit.Committer.Name, diff, subject)
	return mailDiff(cg.Repo, *commit.Commit.Committer.Email, msg)
}

func (cg *ChefGuard) getDiff(sha string) (string, *github.RepositoryCommit, error) {
	var err error
	if cg.gitClient == nil {
		if cg.gitClient, err = setupGitClient(); err != nil {
			return "", nil, fmt.Errorf("Failed to create Git client: %s", err)
		}
	}
	commit, _, err := cg.gitClient.Repositories.GetCommit(cfg.Default.GitOrganization, cg.Repo, sha)
	if err != nil {
		return "", nil, err
	}
	if len(commit.Files) == 0 {
		return "", nil, nil
	}
	const layout = "Mon Jan 2 3:04 2006"
	t := time.Now()
	msg := []string{fmt.Sprintf("Commit : %s\nDate   : %s\nUser   : %s", *commit.SHA, t.Format(layout), cg.User)}
	for _, file := range commit.Files {
		patch := fmt.Sprintf("<br />\nFile: %s\n%s\n", *file.Filename, *file.Patch)
		msg = append(msg, patch)
	}
	return strings.Join(msg, "\n"), commit, nil
}

func createMessage(org, user, diff, subject string) string {
	start := fmt.Sprintf(`From: %s
To: %s
Subject: %s
MIME-version: 1.0
Content-Type: text/html; charset="UTF-8"
<html>
<head>
<style><!--
  body {background-color:#ffffff;}
  .patch {margin:0;}
  #added {background-color:#ddffdd;}
  #removed {background-color:#ffdddd;}
  #context {background-color:#eeeeee;}
--></style>
</head>
<body>`, user, getEffectiveConfig("MailRecipient", org).(string), subject)
	end := fmt.Sprint(`</body>
</html>`)

	html := []string{start}
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+"):
			line = fmt.Sprintf(`<pre class="patch" id="added">%s</pre>`, line)
		case strings.HasPrefix(line, "-"):
			line = fmt.Sprintf(`<pre class="patch" id="removed">%s</pre>`, line)
		default:
			line = fmt.Sprintf(`<pre class="patch" id="context">%s</pre>`, line)
		}
		html = append(html, line)
	}
	html = append(html, end)
	return strings.Join(html, "\n")
}

func mailDiff(org, from, msg string) error {
	c, err := smtp.Dial(fmt.Sprintf("%s:%d", getEffectiveConfig("MailServer", org).(string), getEffectiveConfig("MailPort", org).(int)))
	if err != nil {
		return err
	}
	defer c.Close()
	if err = c.Hello(getEffectiveConfig("MailDomain", org).(string)); err != nil {
		return err
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err = c.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
			return err
		}
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	if err = c.Rcpt(getEffectiveConfig("MailRecipient", org).(string)); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}

func searchCookbookRepo(org, repo, tag string, taggedOnly bool) (*url.URL, bool, error) {
	tagged := false
	gitClient, err := getCustomClient(org)
	if err != nil {
		return nil, tagged, fmt.Errorf("Failed to create custom Git client: %s", err)
	}
	// First check if a tag exists
	_, resp, err := gitClient.Git.GetRef(org, repo, fmt.Sprintf("tags/%s", tag))
	if err != nil {
		switch resp.StatusCode {
		case http.StatusNotFound:
			tag = "master"
		case http.StatusUnauthorized:
			return nil, tagged, fmt.Errorf("The token configured for Github organization %s is not valid!", org)
		default:
			return nil, tagged, fmt.Errorf("Received an unexpected reply from Github: %v", err)
		}
	} else {
		tagged = true
	}
	if taggedOnly && !tagged {
		return nil, tagged, nil
	}
	// Get the archive link for the tagged version or master
	link, resp, err := gitClient.Repositories.GetArchiveLink(org, repo, github.Tarball, &github.RepositoryContentGetOptions{Ref: tag})
	if err != nil {
		switch resp.StatusCode {
		case http.StatusNotFound:
			return nil, tagged, nil
		case http.StatusUnauthorized:
			return nil, tagged, fmt.Errorf("The token configured for Github organization %s is not valid!", org)
		default:
			return nil, tagged, fmt.Errorf("Received an unexpected reply from Github: %v", err)
		}
	}
	return link, tagged, nil
}

func tagCookbookRepo(org, repo, version, user, mail string) error {
	gitClient, err := getCustomClient(org)
	if err != nil {
		return fmt.Errorf("Failed to create custom Git client: %s", err)
	}
	master, resp, err := gitClient.Git.GetRef(org, repo, "heads/master")
	if err != nil {
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("The token configured for Github organization %s is not valid!", org)
		} else {
			return fmt.Errorf("Received an unexpected reply from Github: %v", err)
		}
	}
	version = fmt.Sprintf("v%s", version)
	message := fmt.Sprint("Tagged by Chef-Guard\n")
	tag := &github.Tag{Tag: &version, Message: &message, Object: master.Object}
	tag.Tagger = &github.CommitAuthor{Name: &user, Email: &mail}

	tagObject, resp, err := gitClient.Git.CreateTag(org, repo, tag)
	if err != nil {
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("The token configured for Github organization %s is not valid!", org)
		} else {
			return fmt.Errorf("Received an unexpected reply from Github: %v", err)
		}
	}
	refTag := fmt.Sprintf("tags/%s", version)
	ref := &github.Reference{Ref: &refTag, URL: tagObject.URL, Object: &github.GitObject{SHA: tagObject.SHA}}
	if _, resp, err = gitClient.Git.CreateRef(org, repo, ref); err != nil {
		return fmt.Errorf("Received an unexpected reply from Github: %v", resp)
	}
	return nil
}

func untagCookbookRepo(org, repo, version string) error {
	gitClient, err := getCustomClient(org)
	if err != nil {
		return fmt.Errorf("Failed to create custom Git client: %s", err)
	}
	ref := fmt.Sprintf("tags/v%s", version)
	if resp, err := gitClient.Git.DeleteRef(org, repo, ref); err != nil {
		return fmt.Errorf("Received an unexpected reply from Github: %v", resp)
	}
	return nil
}

func getCustomClient(org string) (*github.Client, error) {
	git, found := cfg.Github[org]
	if !found {
		return nil, fmt.Errorf("No Github config specified for organization: %s!", org)
	}
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: cfg.Github[org].Token},
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.Github[org].SSLNoVerify},
		},
	}
	gitClient := github.NewClient(t.Client())
	if git.ServerURL != "" {
		u, err := url.Parse(git.ServerURL)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse Github server URL %s: %s", git.ServerURL, err)
		}
		gitClient.BaseURL = u
	}
	return gitClient, nil
}
