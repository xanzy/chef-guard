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

func TestChefIgnore(t *testing.T) {
	files_to_include := []string{"foo", "~foo", "foo.txt", "foo/bar.txt", "foo/bar"}
	files_to_ignore := []string{"foo~", "foo/bar.txt~", "foo/bar/foo.txt.swp", "foo.txt.bak", "test/foo.test", "test/foo/bar.test", "foo/bar_flymake.el"}
	content := []byte("*~\n*.sw[a-z]\n*.b?k\ntest*\n*_flymake.*\nfoo/b[^a]r")

	for _, f := range files_to_include {
		match, err := ChefIgnore(bytes.NewReader(content), f)
		if err != nil {
			t.Fatalf("Received an unexpected error: %s", err)
		}
		if match {
			t.Errorf("ChefIgnore('%s', %s) returned '%v', want 'false'", strings.Replace(string(content), "\n", ", ", -1), f, match)
		}
	}

	for _, f := range files_to_ignore {
		match, err := ChefIgnore(bytes.NewReader(content), f)
		if err != nil {
			t.Fatalf("Received an unexpected error: %s", err)
		}
		if !match {
			t.Errorf("ChefIgnore('%s', %s) returned '%v', want 'true'", strings.Replace(string(content), "\n", ", ", -1), f, match)
		}
	}
}
