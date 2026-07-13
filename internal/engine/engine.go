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
// binaries from the core's Introspector. It is shell-agnostic: the
// per-shell packages decode their shell's transport into a Query, call
// Complete, and encode the Candidates in their shell's answer format.
// All completion logic lives here, exactly once.
//
// The package is internal for now but its API is written as if public;
// it may be promoted when third-party shell packages materialize.
package engine

import (
	"fmt"
	"reflect"
	"strings"

	sxclifw "sxcli.dev/fw"
)

// Source is the narrow view of the core's *sxclifw.Introspector the
// engine consumes — the honest ledger of what this module needs from
// the framework. *sxclifw.Introspector satisfies it implicitly; tests
// satisfy it with a fake.
type Source interface {
	// Applets returns the ids of the binary's public applets, in
	// registration order (Hidden and System applets are already
	// filtered by the core).
	Applets() []string
	// SingleApplet reports the applet that would run with no selector
	// word — dispatch-mode truth from the core's own dispatch rules.
	// The engine must not re-derive it from Applets: that listing is
	// public-only, while a Hidden non-System applet still counts for
	// the mode.
	SingleApplet() (string, bool)
	// Services returns the ids of every registered service — the
	// candidate pool for values declared HintServiceID (the core's
	// --disable and --enable).
	Services() []string
	// Arguments returns the closure-true argument schema the applet
	// would have if invoked with args — the words BEFORE the cursor:
	// a half-typed token passed as data would be planned as
	// configuration.
	Arguments(appletID string, args []string) ([]sxclifw.ArgInfo, error)
}

// Query is one completion request, already decoded from the shell's
// transport by the calling adapter.
type Query struct {
	// Applet is the target applet id; "" means the binary decides —
	// single-applet binaries have no selector word, and in
	// multi-applet binaries an empty Applet with an empty Words means
	// the first word itself is being completed.
	Applet string
	// Words are the complete words before the cursor, selector
	// excluded. The half-typed token at the cursor is NOT among them.
	Words []string
	// Current is the half-typed token at the cursor, "" at a fresh
	// word. It is only ever used as a filter prefix, never planned.
	Current string
}

// Kind tells the adapter how to render a candidate — or hands the work
// back to the shell's native machinery.
type Kind int

const (
	// KindApplet is an applet id completing the first word.
	KindApplet Kind = iota
	// KindArg is an argument name; Value carries the dashes ("--log-level", "-c").
	KindArg
	// KindValue is a value from a declared domain: an Allowed value,
	// or a service id for fields declared HintServiceID.
	KindValue
	// KindFiles directs the adapter to emit the shell's native file
	// completion (declared via HintFile; Value and Doc are empty).
	KindFiles
	// KindDirs directs the adapter to emit the shell's native
	// directory completion (declared via HintDirectory).
	KindDirs
)

// Candidate is one completion suggestion. Doc carries the one-line
// description (usage text, or the Metadata Doc when present); shells
// that render descriptions (zsh, fish) show it, bash ignores it.
type Candidate struct {
	Value string
	Kind  Kind
	Doc   string
}

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
	long := map[string]*sxclifw.ArgInfo{}
	short := map[string]*sxclifw.ArgInfo{}
	for i := 0; i < len(infos); i++ {
		if infos[i].Long != "" {
			long[infos[i].Long] = &infos[i]
		}
		if infos[i].Short != "" {
			short[infos[i].Short] = &infos[i]
		}
	}
	pending, positional, used := walk(words, long, short)
	if !positional {
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
// bare -- put the cursor in positional land, and which long names were
// already consumed.
func walk(words []string, long, short map[string]*sxclifw.ArgInfo) (*sxclifw.ArgInfo, bool, map[string]bool) {
	var pending *sxclifw.ArgInfo
	positional := false
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
		}
		// bare words are positional data in passing; completion skips them
	}
	return pending, positional, used
}

// values emits the candidates for one field's value position, prefix
// filtered: the enforced Allowed domain first, bools (reachable only
// through the = form), then the advisory hints — file and directory
// directives hand the work to the shell, service ids come from the
// registry. An undeclared value yields nothing and the shell's own
// default takes over.
func values(src Source, f *sxclifw.ArgInfo, prefix string) []Candidate {
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
	} else if f.Hint == sxclifw.HintFile {
		out = append(out, Candidate{Kind: KindFiles})
	} else if f.Hint == sxclifw.HintDirectory {
		out = append(out, Candidate{Kind: KindDirs})
	} else if f.Hint == sxclifw.HintServiceID {
		for _, id := range src.Services() {
			if strings.HasPrefix(id, prefix) {
				out = append(out, Candidate{Value: id, Kind: KindValue})
			}
		}
	}
	return out
}

// isBool mirrors the parser's rule: a non-slice bool field never
// consumes the next word.
func isBool(f *sxclifw.ArgInfo) bool {
	return !f.IsSlice && f.Type != nil && f.Type.Kind() == reflect.Bool
}
