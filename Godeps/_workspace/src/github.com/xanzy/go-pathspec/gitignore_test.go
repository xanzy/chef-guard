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

package pathspec

import (
	"bytes"
	"strings"
	"testing"
)

func TestGitIgnore(t *testing.T) {
	files_to_include := []string{"!.#test", "~foo", "foo/foo.txt", "bar/foobar.txt", "foo/bar.txt", "/bar/foo"}
	files_to_ignore := []string{".#test", "foo/#test#", "foo/bar/.foo.txt.swp", "foo/foobar/foobar.txt", "foo.txt", "test/foo.test", "test/foo/bar.test", "foo/bar", "foo/1/2/bar"}
	content := []byte(".#*\n\\#*#\n.*.sw[a-z]\n**/foobar/foobar.txt\n/foo.txt\ntest/\nfoo/**/bar\n/b[^a]r/foo")

	for _, f := range files_to_include {
		match, err := GitIgnore(bytes.NewReader(content), f)
		if err != nil {
			t.Fatalf("Received an unexpected error: %s", err)
		}
		if match {
			t.Errorf("ChefIgnore('%s', %s) returned '%v', want 'false'", strings.Replace(string(content), "\n", ", ", -1), f, match)
		}
	}

	for _, f := range files_to_ignore {
		match, err := GitIgnore(bytes.NewReader(content), f)
		if err != nil {
			t.Fatalf("Received an unexpected error: %s", err)
		}
		if !match {
			t.Errorf("ChefIgnore('%s', %s) returned '%v', want 'true'", strings.Replace(string(content), "\n", ", ", -1), f, match)
		}
	}
}
