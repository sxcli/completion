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

// Package bash catalogs the completionbash System applet: bash
// completion for the composing binary. Accept it by ID (AcceptAll
// compositions get it by importing):
//
//	fw.Builder().Accept(bash.ID, …)
//
// Installation (documented by --script's output too):
//
//	eval "$(mybin completionbash --script)"
//
// The generated script is deliberately dumb: it forwards bash's raw
// completion state and every interpretation — dropping the command
// word, slicing at the cursor, reassembling the =-tokens bash splits
// apart, basename dispatch — happens here, in testable Go.
package bash

import (
	"sxcli.dev/fw/system"
)

// Completion is the completionbash System applet. It consumes the
// core's Introspector (by concrete type, cold like any service) and
// answers two operations: --script emission and completion queries.
type Completion struct {
	// I is the core's composition truth; the closure containing it is
	// never ejected, and only completion invocations pay that.
	Sys system.System `inject:""`
	cfg config
}

// config is the wire protocol between the generated bash script and
// the applet:
//
//	<cmd> completionbash [--applet <id>] --cword $COMP_CWORD \
//	    --line "$COMP_LINE" --breaks "$COMP_WORDBREAKS" -- "${COMP_WORDS[@]}"
//
// The raw COMP_WORDS arrive verbatim as positionals — command word
// included, :/=-splits unrepaired — with COMP_CWORD locating the token
// being completed. COMP_LINE disambiguates glued separators from
// spaced ones during reassembly ("--addr:8080" versus "--addr :8080");
// COMP_WORDBREAKS tells which separators bash actually split on and
// segments the word bash will replace. --applet is baked into the
// script at generation time when the script was generated through an
// applet symlink; there is no basename logic at query time. Everything
// is argument-only (env:"-"): the query is per-keystroke transport,
// not configuration.
type config struct {
	Version uint32   `json:"version"`
	Script  bool     `json:"script" conf:"script" env:"-" usage:"print the bash completion script to stdout and exit"`
	Applet  string   `json:"applet" conf:"applet" env:"-" usage:"target applet id baked by the generated script; empty means selector logic applies"`
	CWord   int      `json:"cword" conf:"cword" env:"-" usage:"index of the word being completed within the raw completion words"`
	Line    string   `json:"line" conf:"line" env:"-" usage:"the raw command line being completed (COMP_LINE)"`
	Breaks  string   `json:"breaks" conf:"breaks" env:"-" usage:"the shell's word-break characters (COMP_WORDBREAKS)"`
	Words   []string `json:"words" pos:"rest" usage:"the raw completion words (COMP_WORDS)"`
}
