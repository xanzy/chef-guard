//
// Copyright (c) 2010, 2012 The Go Authors. All rights reserved.
//
// Copyright (c) 2014 Sander van Harmelen. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//
// Orginal file [1] customized to fit our needs for the pathspec package.
// [1] https://code.google.com/p/go/source/browse/src/pkg/path/match.go
//

package pathspec

import (
	"bufio"
	"errors"
	"io"
	"runtime"
	"strings"
	"unicode/utf8"
)

// ErrBadPattern indicates a globbing pattern was malformed.
var ErrBadPattern = errors.New("syntax error in pattern")

// ChefIgnore returns true if name matches the shell file name pattern.
// The pattern syntax is:
//
//  pattern:
//    { term }
//  term:
//    '*'         matches any sequence characters
//    '?'         matches any single character
//    '[' [ '^' ] { character-range } ']'
//                character class (must be non-empty)
//    c           matches character c (c != '*', '?', '\\', '[')
//    '\\' c      matches character c
//
//  character-range:
//    c           matches character c (c != '\\', '-', ']')
//    '\\' c      matches character c
//    lo '-' hi   matches character c for lo <= c <= hi
//
// ChefIgnore requires pattern to match all of name, not just a
// substring. The only possible returned error is ErrBadPattern,
// when pattern is malformed.
//
// On Windows, escaping is disabled. Instead, '\\' is treated as
// path separator.

func ChefIgnore(content io.Reader, name string) (ignore bool, err error) {
	scanner := bufio.NewScanner(content)
	for scanner.Scan() {
		pattern := strings.TrimSpace(scanner.Text())
		if len(pattern) == 0 || pattern[0] == '#' {
			continue
		}
		if ignore, err = match(pattern, name); ignore || err != nil {
			return ignore, err
		}
	}
	return false, scanner.Err()
}

func match(pattern, name string) (ignore bool, err error) {
Pattern:
	for len(pattern) > 0 {
		var star bool
		var chunk string
		star, chunk, pattern = scanChunk(pattern)
		if star && chunk == "" {
			// Trailing * matches rest of string
			return true, nil
		}
		// Look for match at current position.
		t, ok, err := matchChunk(chunk, name)
		// if we're the last chunk, make sure we've exhausted the name
		// otherwise we'll give a false result even if we could still match
		// using the star
		if ok && (len(t) == 0 || len(pattern) > 0) {
			name = t
			continue
		}
		if err != nil {
			return false, err
		}
		if star {
			// Look for match skipping i+1 bytes.
			for i := 0; i < len(name); i++ {
				t, ok, err := matchChunk(chunk, name[i+1:])
				if ok {
					// if we're the last chunk, make sure we exhausted the name
					if len(pattern) == 0 && len(t) > 0 {
						continue
					}
					name = t
					continue Pattern
				}
				if err != nil {
					return false, err
				}
			}
		}
		return false, nil
	}
	return len(name) == 0, nil
}

// scanChunk gets the next segment of pattern, which is a non-star string
// possibly preceded by a star.
func scanChunk(pattern string) (star bool, chunk, rest string) {
	for len(pattern) > 0 && pattern[0] == '*' {
		pattern = pattern[1:]
		star = true
	}
	inrange := false
	var i int
Scan:
	for i = 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\':
			if runtime.GOOS != "windows" {
				// error check handled in matchChunk: bad pattern.
				if i+1 < len(pattern) {
					i++
				}
			}
		case '[':
			inrange = true
		case ']':
			inrange = false
		case '*':
			if !inrange {
				break Scan
			}
		}
	}
	return star, pattern[0:i], pattern[i:]
}

// matchChunk checks whether chunk matches the beginning of s.
// If so, it returns the remainder of s (after the match).
// Chunk is all single-character operators: literals, char classes, and ?.
func matchChunk(chunk, s string) (rest string, ok bool, err error) {
	for len(chunk) > 0 {
		if len(s) == 0 {
			return
		}
		switch chunk[0] {
		case '[':
			// character class
			r, n := utf8.DecodeRuneInString(s)
			s = s[n:]
			chunk = chunk[1:]
			// We can't end right after '[', we're expecting at least
			// a closing bracket and possibly a caret.
			if len(chunk) == 0 {
				err = ErrBadPattern
				return
			}
			// possibly negated
			negated := chunk[0] == '^'
			if negated {
				chunk = chunk[1:]
			}
			// parse all ranges
			match := false
			nrange := 0
			for {
				if len(chunk) > 0 && chunk[0] == ']' && nrange > 0 {
					chunk = chunk[1:]
					break
				}
				var lo, hi rune
				if lo, chunk, err = getEsc(chunk); err != nil {
					return
				}
				hi = lo
				if chunk[0] == '-' {
					if hi, chunk, err = getEsc(chunk[1:]); err != nil {
						return
					}
				}
				if lo <= r && r <= hi {
					match = true
				}
				nrange++
			}
			if match == negated {
				return
			}

		case '?':
			_, n := utf8.DecodeRuneInString(s)
			s = s[n:]
			chunk = chunk[1:]

		case '\\':
			if runtime.GOOS != "windows" {
				chunk = chunk[1:]
				if len(chunk) == 0 {
					err = ErrBadPattern
					return
				}
			}
			fallthrough

		default:
			if chunk[0] != s[0] {
				return
			}
			s = s[1:]
			chunk = chunk[1:]
		}
	}
	return s, true, nil
}

// getEsc gets a possibly-escaped character from chunk, for a character class.
func getEsc(chunk string) (r rune, nchunk string, err error) {
	if len(chunk) == 0 || chunk[0] == '-' || chunk[0] == ']' {
		err = ErrBadPattern
		return
	}
	if chunk[0] == '\\' && runtime.GOOS != "windows" {
		chunk = chunk[1:]
		if len(chunk) == 0 {
			err = ErrBadPattern
			return
		}
	}
	r, n := utf8.DecodeRuneInString(chunk)
	if r == utf8.RuneError && n == 1 {
		err = ErrBadPattern
	}
	nchunk = chunk[n:]
	if len(nchunk) == 0 {
		err = ErrBadPattern
	}
	return
}
