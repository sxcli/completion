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

// Package engine computes completion candidates for any binary that
// can answer the Source contract — sxcli.dev/fw binaries hand it
// their target-scoped Introspector, and the engine depends on no
// framework, only on the conf engine's schema types. It is
// shell-agnostic: a shell adapter decodes its shell's transport into
// a Query, calls Complete, and encodes the Candidates in its shell's
// answer format. All completion logic lives here, exactly once.
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

	conf "sxcli.dev/conf/engine"
)

// The operator vocabulary the engine speaks without a framework: the
// core family's names are reserved in every sxcli binary, and the
// service-id hint is the first consumer-defined ValueHint (fw's
// HintServiceID).
const (
	coreAlias     = conf.CoreID
	systemAlias   = "system"
	hintServiceID = conf.HintCustom
)

// Complete returns the candidates for one query, already filtered by
// q.Current as a plain prefix. The single entry point: applet-name
// completion, argument names, declared value domains and the
// file/directory directives all come out of the same call — the
// adapter never decides what is being completed, only how to print it.
// Every answer comes from a target-scoped view built from the catalog
// alone — deterministic, environment-free — and an unanswerable query
// (an unknown target included) yields no candidates rather than an
// error a shell script cannot render anyway.
//
// The target applet resolves like core dispatch: an explicit q.Applet
// wins, then single-applet mode, then a bare first word as the
// selector. With no target and no words the first word itself is being
// completed: public applet names.
func Complete(sys System, q Query) []Candidate {
	var out []Candidate
	binary := sys.Introspector("")
	if binary == nil {
		return nil // no composition attached: nothing to offer
	}
	target := q.Applet
	words := q.Words
	if target == "" {
		if id, single := binary.SingleApplet(); single {
			target = id
		} else if len(words) > 0 && !strings.HasPrefix(words[0], "-") {
			target = words[0]
			words = words[1:]
		}
	}
	if target != "" {
		if view := sys.Introspector(target); view != nil {
			out = arguments(view, words, q.Current)
		}
	} else if len(words) == 0 && !strings.HasPrefix(q.Current, "-") {
		for _, id := range binary.Applets() {
			if strings.HasPrefix(id, q.Current) {
				out = append(out, Candidate{Value: id, Kind: KindApplet})
			}
		}
	}
	return out
}

// arguments completes within one applet's invocation: the view IS
// the schema true to the target's resolved service set, the words
// are walked to find the
// parse state at the cursor, and candidates are emitted from that
// state.
func arguments(src Source, words []string, current string) []Candidate {
	var out []Candidate
	infos := src.Arguments(words)
	long := map[string]*conf.ArgInfo{}
	short := map[string]*conf.ArgInfo{}
	for i := 0; i < len(infos); i++ {
		if infos[i].Long != "" {
			long[infos[i].Long] = &infos[i]
		}
		if infos[i].Short != "" {
			short[infos[i].Short] = &infos[i]
		}
	}
	pending, positional, filled, used := walk(words, long, short)
	slot, hasSlot := currentSlot(src.Positionals(), filled)
	names := func() {
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
	if positional {
		// past the terminator every word belongs to a slot — offer
		// what the slot knows how to complete, or nothing
		if hasSlot {
			out = slotValues(slot, current)
		}
	} else {
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
			names()
		} else if hasSlot {
			// a bare word belongs to the next unfilled slot; typing
			// '-' asks for argument names instead
			out = slotValues(slot, current)
		} else {
			// no positionals declared, or every slot already filled:
			// a bare word here can only become an argument
			names()
		}
	}
	return out
}

// currentSlot picks the slot the next bare word would fill: the
// indexed slot at position filled, else the rest collector, which
// never fills up.
func currentSlot(slots []conf.PosInfo, filled int) (conf.PosInfo, bool) {
	for i := 0; i < len(slots); i++ {
		if slots[i].Rest || i == filled {
			if slots[i].Rest && i < filled {
				return slots[i], true
			}
			if i == filled {
				return slots[i], true
			}
		}
	}
	if len(slots) > 0 && slots[len(slots)-1].Rest {
		return slots[len(slots)-1], true
	}
	return conf.PosInfo{}, false
}

// slotValues emits a positional slot's candidates: the shell's
// native file or directory completion for hinted slots, the domain
// values for a closed one, nothing for an open unhinted slot — the
// position is the slot's even when there is nothing to offer.
func slotValues(slot conf.PosInfo, prefix string) []Candidate {
	var out []Candidate
	if len(slot.Allowed) > 0 {
		for _, v := range slot.Allowed {
			rendered := fmt.Sprint(v)
			if strings.HasPrefix(rendered, prefix) {
				out = append(out, Candidate{Value: rendered, Kind: KindValue, Doc: slot.Usage})
			}
		}
	} else if slot.Hint == conf.HintFile {
		out = append(out, Candidate{Kind: KindFiles})
	} else if slot.Hint == conf.HintDirectory {
		out = append(out, Candidate{Kind: KindDirs})
	}
	return out
}

// walk replays the words before the cursor against the schema exactly
// as the parser would read them, returning the argument left expecting
// a value (never a bool — bools take values only =-joined), whether a
// bare -- put the cursor in positional land (the parser's single
// escape: bare tokens interleave freely with arguments, so passing
// one silences nothing), and which long names were already consumed.
// The int result counts the positional slots the words already
// filled: bare tokens not consumed as values, plus every word past
// the -- terminator.
func walk(words []string, long, short map[string]*conf.ArgInfo) (*conf.ArgInfo, bool, int, map[string]bool) {
	var pending *conf.ArgInfo
	positional := false
	filled := 0
	used := map[string]bool{}
	for k := 0; k < len(words) && !positional; k++ {
		w := words[k]
		if pending != nil {
			pending = nil // this word was the pending argument's value
		} else if w == "--" {
			positional = true
			filled += len(words) - k - 1
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
			// a bare word is positional data in passing: it fills the
			// next slot, and the parser accepts arguments on either
			// side of it
			filled++
		}
	}
	return pending, positional, filled, used
}

// values emits the candidates for one field's value position, prefix
// filtered: the enforced Allowed domain first, bools (reachable only
// through the = form), then the advisory hints — file and directory
// directives hand the work to the shell, service ids come from the
// registry. An undeclared value yields nothing and the shell's own
// default takes over.
func values(src Source, f *conf.ArgInfo, prefix string) []Candidate {
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
	} else if f.Hint == conf.HintFile {
		out = append(out, Candidate{Kind: KindFiles})
	} else if f.Hint == conf.HintDirectory {
		out = append(out, Candidate{Kind: KindDirs})
	} else if f.Hint == hintServiceID {
		for _, alias := range src.Services() {
			// the core family leads the listing but is never a
			// control target (#23) — a candidate that is always a
			// startup violation is never offered
			if alias != coreAlias && alias != systemAlias && strings.HasPrefix(alias, prefix) {
				out = append(out, Candidate{Value: alias, Kind: KindValue})
			}
		}
	}
	return out
}

// isBool mirrors the parser's rule: a non-slice bool field never
// consumes the next word.
func isBool(f *conf.ArgInfo) bool {
	return !f.IsSlice && f.Type != nil && f.Type.Kind() == reflect.Bool
}
