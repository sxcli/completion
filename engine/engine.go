// Copyright 2026 Plamen K. Kosseff
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package engine computes completion candidates for sxcli.dev/fw
// binaries from the core's Introspector. It is shell-agnostic: a
// shell adapter decodes its shell's transport into a Query, calls
// Complete, and encodes the Candidates in its shell's answer format.
// All completion logic lives here, exactly once.
//
// This is the public API for third-party shell adapters (fish,
// PowerShell, elvish, …): implement the transport and the encoding,
// and the engine answers what completes — the bash and zsh packages
// in this module are the reference implementations. The generation
// policy for --script emission lives in the sibling script package.
package engine

import (
	"fmt"
	"reflect"
	"strings"

	"sxcli.dev/fw"
)

// Complete returns the candidates for one query, already filtered by
// q.Current as a plain prefix. The single entry point: applet-name
// completion, argument names, declared value domains and the
// file/directory directives all come out of the same call — the
// adapter never decides what is being completed, only how to print it.
// The result is best-effort like the Introspector itself: on planning
// violations the schema falls back to registration-level truth, and an
// unanswerable query yields no candidates rather than an error a shell
// script cannot render anyway.
//
// The target applet resolves like core dispatch: an explicit q.Applet
// wins, then single-applet mode, then a bare first word as the
// selector. With no target and no words the first word itself is being
// completed: public applet names.
func Complete(src Source, q Query) []Candidate {
	var out []Candidate
	target := q.Applet
	words := q.Words
	if target == "" {
		if id, single := src.SingleApplet(); single {
			target = id
		} else if len(words) > 0 && !strings.HasPrefix(words[0], "-") {
			target = words[0]
			words = words[1:]
		}
	}
	if target != "" {
		out = arguments(src, target, words, q.Current)
	} else if len(words) == 0 && !strings.HasPrefix(q.Current, "-") {
		for _, id := range src.Applets() {
			if strings.HasPrefix(id, q.Current) {
				out = append(out, Candidate{Value: id, Kind: KindApplet})
			}
		}
	}
	return out
}

// arguments completes within one applet's invocation: the closure-true
// schema is planned from the words (an --enable among them is honored
// by the core's own planning), the words are walked to find the parse
// state at the cursor, and candidates are emitted from that state.
func arguments(src Source, appletID string, words []string, current string) []Candidate {
	var out []Candidate
	infos, _ := src.Arguments(appletID, words) // best effort: the fallback schema still completes
	long := map[string]*fw.ArgInfo{}
	short := map[string]*fw.ArgInfo{}
	for i := 0; i < len(infos); i++ {
		if infos[i].Long != "" {
			long[infos[i].Long] = &infos[i]
		}
		if infos[i].Short != "" {
			short[infos[i].Short] = &infos[i]
		}
	}
	pending, positional, bare, used := walk(words, long, short)
	if !positional && !bare {
		name, joinedValue, joined := strings.Cut(strings.TrimPrefix(current, "--"), "=")
		if pending != nil {
			out = values(src, pending, current)
		} else if joined && strings.HasPrefix(current, "--") {
			// the semantic = split mirrors the parser; bools are
			// completable here and only here
			if f, known := long[name]; known {
				out = values(src, f, joinedValue)
			}
		} else if strings.HasPrefix(current, "-") {
			// argument names: long forms only — shorts are for people
			// who know what they are doing. Used scalars are done;
			// used slices append by repetition and stay offered.
			for i := 0; i < len(infos); i++ {
				f := &infos[i]
				if f.Long != "" && (f.IsSlice || !used[f.Long]) {
					if strings.HasPrefix("--"+f.Long, current) {
						doc := f.Doc
						if doc == "" {
							doc = f.Usage
						}
						out = append(out, Candidate{Value: "--" + f.Long, Kind: KindArg, Doc: doc})
					}
				}
			}
		}
	}
	return out
}

// walk replays the words before the cursor against the schema exactly
// as the parser would read them, returning the argument left expecting
// a value (never a bool — bools take values only =-joined), whether a
// bare -- put the cursor in positional land, whether a bare positional
// word already passed (the strict parser refuses arguments after one,
// so completion goes silent rather than offer what cannot parse), and
// which long names were already consumed.
func walk(words []string, long, short map[string]*fw.ArgInfo) (*fw.ArgInfo, bool, bool, map[string]bool) {
	var pending *fw.ArgInfo
	positional := false
	bare := false
	used := map[string]bool{}
	for k := 0; k < len(words) && !positional; k++ {
		w := words[k]
		if pending != nil {
			pending = nil // this word was the pending argument's value
		} else if w == "--" {
			positional = true
		} else if strings.HasPrefix(w, "--") {
			name, _, joined := strings.Cut(w[2:], "=")
			if f, known := long[name]; known {
				used[name] = true
				if !joined && !isBool(f) {
					pending = f
				}
			}
		} else if strings.HasPrefix(w, "-") && len(w) > 1 {
			// short bundle: every member is a bool except possibly the
			// last, which may leave a pending value
			body, _, joined := strings.Cut(w[1:], "=")
			for b := 0; b < len(body); b++ {
				if f, known := short[string(body[b])]; known {
					used[f.Long] = true
					if b == len(body)-1 && !joined && !isBool(f) {
						pending = f
					}
				}
			}
		} else {
			bare = true // positional data in passing
		}
	}
	return pending, positional, bare, used
}

// values emits the candidates for one field's value position, prefix
// filtered: the enforced Allowed domain first, bools (reachable only
// through the = form), then the advisory hints — file and directory
// directives hand the work to the shell, service ids come from the
// registry. An undeclared value yields nothing and the shell's own
// default takes over.
func values(src Source, f *fw.ArgInfo, prefix string) []Candidate {
	var out []Candidate
	if len(f.Allowed) > 0 {
		for _, v := range f.Allowed {
			s := fmt.Sprint(v)
			if strings.HasPrefix(s, prefix) {
				out = append(out, Candidate{Value: s, Kind: KindValue})
			}
		}
	} else if isBool(f) {
		for _, s := range []string{"true", "false"} {
			if strings.HasPrefix(s, prefix) {
				out = append(out, Candidate{Value: s, Kind: KindValue})
			}
		}
	} else if f.Hint == fw.HintFile {
		out = append(out, Candidate{Kind: KindFiles})
	} else if f.Hint == fw.HintDirectory {
		out = append(out, Candidate{Kind: KindDirs})
	} else if f.Hint == fw.HintServiceID {
		for _, alias := range src.Services() {
			// the synthesized core leads the listing but is not a
			// service reference anyone can disable, enable or override
			if alias != fw.CoreAlias && strings.HasPrefix(alias, prefix) {
				out = append(out, Candidate{Value: alias, Kind: KindValue})
			}
		}
	}
	return out
}

// isBool mirrors the parser's rule: a non-slice bool field never
// consumes the next word.
func isBool(f *fw.ArgInfo) bool {
	return !f.IsSlice && f.Type != nil && f.Type.Kind() == reflect.Bool
}
