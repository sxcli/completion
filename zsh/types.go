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

// Package zsh registers the completionzsh System applet: zsh
// completion for the importing binary. Blank-import to link it, like a
// log sink:
//
//	import _ "sxcli.dev/completion/zsh"
//
// Installation (compinit must have run first — standard zsh setup):
//
//	eval "$(mybin completionzsh --script)"
//
// Zsh tokenizes the command line properly — no COMP_WORDBREAKS
// shredding, quotes honored — so the transport is simpler than bash's:
// no line, no breaks, no reassembly. $PREFIX is the part of the
// current word before the cursor, which is exactly the engine's
// Current contract. And zsh renders descriptions: the usage/Doc
// metadata finally shows, rendered through Tr.
package zsh

import (
	"sxcli.dev/fw"
)

// Completion is the completionzsh System applet. It consumes the
// core's Introspector (by concrete type, cold like any service) and
// answers two operations: --script emission and completion queries.
type Completion struct {
	// I is the core's composition truth; the closure containing it is
	// never ejected, and only completion invocations pay that.
	I   *fw.Introspector `inject:""`
	cfg config
}

// config is the wire protocol between the generated zsh script and
// the applet:
//
//	<cmd> completionzsh [--applet <id>] --cword $((CURRENT-1)) \
//	    --current "$PREFIX" -- "${(@)words}"
//
// The words arrive as true tokens (command word included), --cword is
// the 0-based index of the word being completed (zsh's CURRENT is
// 1-based), and --current is $PREFIX — the pre-cursor part of that
// word. --applet is baked at generation time when the script was
// generated through an applet symlink. Everything is argument-only
// (env:"-"): the query is per-keystroke transport, not configuration.
type config struct {
	Version uint32   `json:"version"`
	Script  bool     `json:"script" conf:"script" env:"-" usage:"print the zsh completion script to stdout and exit"`
	Applet  string   `json:"applet" conf:"applet" env:"-" usage:"target applet id baked by the generated script; empty means selector logic applies"`
	CWord   int      `json:"cword" conf:"cword" env:"-" usage:"0-based index of the word being completed"`
	Current string   `json:"current" conf:"current" env:"-" usage:"the pre-cursor part of the word being completed (PREFIX)"`
	Words   []string `json:"words" pos:"rest" usage:"the raw completion words"`
}
