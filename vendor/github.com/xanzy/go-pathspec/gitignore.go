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
	"bufio"
	"bytes"
	"io"
	"regexp"
	"strings"
)

type GitIgnorePattern struct {
	Regex   string
	Include bool
}

// A blank line matches no files, so it can serve as a separator for readability.
//
// A line starting with # serves as a comment. Put a backslash ("\") in front of
// the first hash for patterns that begin with a hash.
//
// An optional prefix "!" which negates the pattern; any matching file excluded
// by a previous pattern will become included again. If a negated pattern matches,
// this will override lower precedence patterns sources. Put a backslash ("\") in
// front of the first "!" for patterns that begin with a literal "!", for example,
// "\!important!.txt".
//
// If the pattern ends with a slash, it is removed for the purpose of the following
// description, but it would only find a match with a directory. In other words,
// foo/ will match a directory foo and paths underneath it, but will not match a
// regular file or a symbolic link foo (this is consistent with the way how pathspec
// works in general in Git).
//
// If the pattern does not contain a slash /, Git treats it as a shell glob pattern
// and checks for a match against the pathname relative to the location of the
// .gitignore file (relative to the toplevel of the work tree if not from a
// .gitignore file).
//
// Otherwise, Git treats the pattern as a shell glob suitable for consumption by
// fnmatch(3) with the FNM_PATHNAME flag: wildcards in the pattern will not match
// a / in the pathname. For example, "Documentation/*.html" matches
// "Documentation/git.html" but not "Documentation/ppc/ppc.html" or/
// "tools/perf/Documentation/perf.html".
//
// A leading slash matches the beginning of the pathname. For example, "/*.c"
// matches "cat-file.c" but not "mozilla-sha1/sha1.c".
//
// Two consecutive asterisks ("**") in patterns matched against full pathname
// may have special meaning:
//
// A leading "**" followed by a slash means match in all directories. For example,
// "**/foo" matches file or directory "foo" anywhere, the same as pattern "foo".
// "**/foo/bar" matches file or directory "bar" anywhere that is directly under
// directory "foo".
//
// A trailing "/" matches everything inside. For example, "abc/" matches all files
// inside directory "abc", relative to the location of the .gitignore file, with
// infinite depth.
//
// A slash followed by two consecutive asterisks then a slash matches zero or more
// directories. For example, "a/**/b" matches "a/b", "a/x/b", "a/x/y/b" and so on.
//
// Other consecutive asterisks are considered invalid.

func GitIgnore(content io.Reader, name string) (ignore bool, err error) {
	scanner := bufio.NewScanner(content)

	for scanner.Scan() {
		pattern := strings.TrimSpace(scanner.Text())
		if len(pattern) == 0 || pattern[0] == '#' {
			continue
		}
		p := parsePattern(pattern)
		match, err := regexp.MatchString(p.Regex, name)
		if err != nil {
			return ignore, err
		}
		if match {
			if p.Include {
				return false, scanner.Err()
			}
			ignore = true
		}
	}
	return ignore, scanner.Err()
}

func parsePattern(pattern string) *GitIgnorePattern {
	p := &GitIgnorePattern{}

	// An optional prefix "!" which negates the pattern; any matching file
	// excluded by a previous pattern will become included again.
	if strings.HasPrefix(pattern, "!") {
		pattern = pattern[1:]
		p.Include = true
	} else {
		p.Include = false
	}

	// Remove leading back-slash escape for escaped hash ('#') or
	// exclamation mark ('!').
	if strings.HasPrefix(pattern, "\\") {
		pattern = pattern[1:]
	}

	// Split pattern into segments.
	pattern_segs := strings.Split(pattern, "/")

	// A pattern beginning with a slash ('/') will only match paths
	// directly on the root directory instead of any descendant paths.
	// So remove empty first segment to make pattern absoluut to root.
	// A pattern without a beginning slash ('/') will match any
	// descendant path. This is equivilent to "**/{pattern}". So
	// prepend with double-asterisks to make pattern relative to
	// root.
	if pattern_segs[0] == "" {
		pattern_segs = pattern_segs[1:]
	} else if pattern_segs[0] != "**" {
		pattern_segs = append([]string{"**"}, pattern_segs...)
	}

	// A pattern ending with a slash ('/') will match all descendant
	// paths of if it is a directory but not if it is a regular file.
	// This is equivilent to "{pattern}/**". So, set last segment to
	// double asterisks to include all descendants.
	if pattern_segs[len(pattern_segs)-1] == "" {
		pattern_segs[len(pattern_segs)-1] = "**"
	}

	// Build regular expression from pattern.
	var expr bytes.Buffer
	expr.WriteString("^")
	need_slash := false

	for i, seg := range pattern_segs {
		switch seg {
		case "**":
			switch {
			case i == 0 && i == len(pattern_segs)-1:
				// A pattern consisting solely of double-asterisks ('**')
				// will match every path.
				expr.WriteString(".+")
			case i == 0:
				// A normalized pattern beginning with double-asterisks
				// ('**') will match any leading path segments.
				expr.WriteString("(?:.+/)?")
				need_slash = false
			case i == len(pattern_segs)-1:
				// A normalized pattern ending with double-asterisks ('**')
				// will match any trailing path segments.
				expr.WriteString("/.+")
			default:
				// A pattern with inner double-asterisks ('**') will match
				// multiple (or zero) inner path segments.
				expr.WriteString("(?:/.+)?")
				need_slash = true
			}
		case "*":
			// Match single path segment.
			if need_slash {
				expr.WriteString("/")
			}
			expr.WriteString("[^/]+")
			need_slash = true
		default:
			// Match segment glob pattern.
			if need_slash {
				expr.WriteString("/")
			}
			expr.WriteString(translateGlob(seg))
			need_slash = true
		}
	}
	expr.WriteString("$")
	p.Regex = expr.String()
	return p
}

// NOTE: This is derived from `fnmatch.translate()` and is similar to
// the POSIX function `fnmatch()` with the `FNM_PATHNAME` flag set.
func translateGlob(glob string) string {
	var regex bytes.Buffer
	escape := false

	for i := 0; i < len(glob); i++ {
		char := glob[i]
		// Escape the character.
		switch {
		case escape:
			escape = false
			regex.WriteString(regexp.QuoteMeta(string(char)))
		case char == '\\':
			// Escape character, escape next character.
			escape = true
		case char == '*':
			// Multi-character wildcard. Match any string (except slashes),
			// including an empty string.
			regex.WriteString("[^/]*")
		case char == '?':
			// Single-character wildcard. Match any single character (except
			// a slash).
			regex.WriteString("[^/]")
		case char == '[':
			regex.WriteString(translateBraketExpression(&i, glob))
		default:
			// Regular character, escape it for regex.
			regex.WriteString(regexp.QuoteMeta(string(char)))
		}
	}
	return regex.String()
}

// Braket expression wildcard. Except for the beginning
// exclamation mark, the whole braket expression can be used
// directly as regex but we have to find where the expression
// ends.
// - "[][!]" matchs ']', '[' and '!'.
// - "[]-]" matchs ']' and '-'.
// - "[!]a-]" matchs any character except ']', 'a' and '-'.
func translateBraketExpression(i *int, glob string) string {
	regex := string(glob[*i])
	*i++
	j := *i

	// Pass brack expression negation.
	if j < len(glob) && glob[j] == '!' {
		j++
	}
	// Pass first closing braket if it is at the beginning of the
	// expression.
	if j < len(glob) && glob[j] == ']' {
		j++
	}
	// Find closing braket. Stop once we reach the end or find it.
	for j < len(glob) && glob[j] != ']' {
		j++
	}

	if j < len(glob) {
		if glob[*i] == '!' {
			regex = regex + "^"
			*i++
		}
		regex = regexp.QuoteMeta(glob[*i:j])
		*i = j
	} else {
		// Failed to find closing braket, treat opening braket as a
		// braket literal instead of as an expression.
		regex = regexp.QuoteMeta(string(glob[*i]))
	}
	return "[" + regex + "]"
}
