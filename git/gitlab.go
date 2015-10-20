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
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/xanzy/go-gitlab"
)

const (
	invalidGitLabToken = "The token configured for GitLab group %s is not valid!"
)

// GetContent implements the Git interface
func (g *GitLab) GetContent(group, project, path string) (*File, interface{}, error) {
	ns := fmt.Sprintf("%s/%s", group, project)

	treeOpts := &gitlab.ListTreeOptions{
		Path: path,
	}
	tree, resp, err := g.client.Repositories.ListTree(ns, treeOpts)
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusNotFound:
				return nil, nil, nil
			case http.StatusUnauthorized:
				return nil, nil, fmt.Errorf(invalidGitLabToken, group)
			}
		}
		return nil, nil, fmt.Errorf("Error retrieving tree for %s: %v", path, err)
	}

	if len(tree) > 0 {
		var files []string
		for _, file := range tree {
			files = append(files, filepath.Join(path, file.Name))
		}

		return nil, files, nil
	}

	fileOpts := &gitlab.GetFileOptions{
		FilePath: path,
		Ref:      "master",
	}
	file, resp, err := g.client.RepositoryFiles.GetFile(ns, fileOpts)
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusNotFound:
				return nil, nil, nil
			case http.StatusUnauthorized:
				return nil, nil, fmt.Errorf(invalidGitLabToken, group)
			}
		}
		return nil, nil, fmt.Errorf("Error retrieving file %s: %v", path, err)
	}

	f := &File{
		Content: file.Content,
		SHA:     file.CommitID,
	}

	if file.Encoding == "base64" {
		content, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return nil, nil, fmt.Errorf("Error decoding file %s, %v", path, err)
		}

		f.Content = string(content)
	}

	return f, nil, nil
}

// CreateFile implements the Git interface
func (g *GitLab) CreateFile(group, project, path, msg string, usr *User, content []byte) (string, error) {
	ns := fmt.Sprintf("%s/%s", group, project)

	opts := &gitlab.CreateFileOptions{
		FilePath:      path,
		BranchName:    "master",
		Content:       string(content),
		CommitMessage: msg,
	}
	_, resp, err := g.client.RepositoryFiles.CreateFile(ns, opts)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return "", fmt.Errorf(invalidGitLabToken, group)
		}
		return "", fmt.Errorf("Error creating file %s: %v", path, err)
	}

	return g.shaOfLatestCommit(group, project)
}

// UpdateFile implements the Git interface
func (g *GitLab) UpdateFile(group, project, path, sha, msg string, usr *User, content []byte) (string, error) {
	ns := fmt.Sprintf("%s/%s", group, project)

	opts := &gitlab.UpdateFileOptions{
		FilePath:      path,
		BranchName:    "master",
		Content:       string(content),
		CommitMessage: msg,
	}
	_, resp, err := g.client.RepositoryFiles.UpdateFile(ns, opts)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return "", fmt.Errorf(invalidGitLabToken, group)
		}
		return "", fmt.Errorf("Error updating file %s: %v", path, err)
	}

	return g.shaOfLatestCommit(group, project)
}

// DeleteFile implements the Git interface
func (g *GitLab) DeleteFile(group, project, path, sha, msg string, usr *User) (string, error) {
	ns := fmt.Sprintf("%s/%s", group, project)

	opts := &gitlab.DeleteFileOptions{
		FilePath:      path,
		BranchName:    "master",
		CommitMessage: msg,
	}
	_, resp, err := g.client.RepositoryFiles.DeleteFile(ns, opts)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return "", fmt.Errorf(invalidGitLabToken, group)
		}
		return "", fmt.Errorf("Error deleting file %s: %v", path, err)
	}

	return g.shaOfLatestCommit(group, project)
}

// DeleteDirectory implements the Git interface
func (g *GitLab) DeleteDirectory(group, project, msg string, dir interface{}, usr *User) error {
	ns := fmt.Sprintf("%s/%s", group, project)

	for _, file := range dir.([]string) {
		// Need a special case for when deleting data bag items
		fn := strings.TrimPrefix(file, "data_bags/")
		msg := fmt.Sprintf(msg, strings.TrimSuffix(fn, ".json"))

		opts := &gitlab.DeleteFileOptions{
			FilePath:      file,
			BranchName:    "master",
			CommitMessage: msg,
		}
		_, resp, err := g.client.RepositoryFiles.DeleteFile(ns, opts)
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusUnauthorized {
				return fmt.Errorf(invalidGitLabToken, group)
			}
			return fmt.Errorf("Error deleting file %s: %v", file, err)
		}
	}

	return nil
}

// GetDiff implements the Git interface
func (g *GitLab) GetDiff(group, project, user, sha string) (string, error) {
	ns := fmt.Sprintf("%s/%s", group, project)

	diffs, resp, err := g.client.Commits.GetCommitDiff(ns, sha)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return "", fmt.Errorf(invalidGitLabToken, group)
		}
		return "", fmt.Errorf("Error retrieving diff of commit %s: %v", sha, err)
	}

	if len(diffs) == 0 {
		return "", nil
	}

	const layout = "Mon Jan 2 3:04 2006"
	t := time.Now()
	msg := []string{fmt.Sprintf("Commit : %s\nDate   : %s\nUser   : %s",
		sha,
		t.Format(layout),
		user,
	)}

	for _, diff := range diffs {
		start := strings.Index(diff.Diff, "@@")
		patch := fmt.Sprintf("<br />\nFile: %s\n%s\n", diff.NewPath, diff.Diff[start:])
		msg = append(msg, patch)
	}

	return strings.Join(msg, "\n"), nil
}

// GetArchiveLink implements the Git interface
func (g *GitLab) GetArchiveLink(group, project, tag string) (*url.URL, error) {
	ns := fmt.Sprintf("%s/%s", group, project)

	_, resp, err := g.client.Projects.GetProject(ns)
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusNotFound:
				return nil, nil
			case http.StatusUnauthorized:
				return nil, fmt.Errorf(invalidGitLabToken, group)
			}
		}
		return nil, fmt.Errorf("Error retrieving archive link of project %s: %v", project, err)
	}

	u, err := url.Parse(fmt.Sprintf("/%s/repository/archive?ref=%s", ns, tag))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse archive link: %v", err)
	}

	return g.client.BaseURL().ResolveReference(u), nil
}

// TagRepo implements the Git interface
func (g *GitLab) TagRepo(group, project, tag string, usr *User) error {
	ns := fmt.Sprintf("%s/%s", group, project)
	message := fmt.Sprint("Tagged by Chef-Guard\n")

	opts := &gitlab.CreateTagOptions{
		TagName: tag,
		Ref:     "master",
		Message: message,
	}
	_, resp, err := g.client.Repositories.CreateTag(ns, opts)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf(invalidGitLabToken, group)
		}
		return fmt.Errorf("Error creating tag for project %s: %v", project, err)
	}

	return nil
}

// TagExists implements the Git interface
func (g *GitLab) TagExists(group, project, tag string) (bool, error) {
	ns := fmt.Sprintf("%s/%s", group, project)

	tags, resp, err := g.client.Repositories.ListTags(ns)
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusNotFound:
				return false, nil
			case http.StatusUnauthorized:
				return false, fmt.Errorf(invalidGitLabToken, group)
			}
		}
		return false, fmt.Errorf("Error retrieving tags of project %s: %v", project, err)
	}

	for _, t := range tags {
		if t.Name == tag {
			return true, nil
		}
	}

	return false, nil
}

// UntagRepo implements the Git interface
func (g *GitLab) UntagRepo(group, project, version string) error {
	// Not implemented in the GitLab API, so could not implement this
	// functionality at this moment...
	return nil
}

func (g *GitLab) shaOfLatestCommit(group, project string) (string, error) {
	ns := fmt.Sprintf("%s/%s", group, project)

	commit, resp, err := g.client.Commits.GetCommit(ns, "master")
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return "", fmt.Errorf(invalidGitLabToken, group)
		}
		return "", fmt.Errorf("Error retrieving SHA of latest commit: %v", err)
	}

	return commit.ID, nil
}
