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

// Package bash registers the completionbash System applet: bash
// completion for the importing binary. Blank-import to link it, like a
// log sink:
//
//	import _ "sxcli.dev/completion/bash"
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
	sxclifw "sxcli.dev/fw"
)

// Completion is the completionbash System applet. It consumes the
// core's Introspector (by concrete type, cold like any service) and
// answers two operations: --script emission and completion queries.
type Completion struct {
	// I is the core's composition truth; the closure containing it is
	// never ejected, and only completion invocations pay that.
	I   *sxclifw.Introspector `inject:""`
	cfg config
}

// config is the wire protocol between the generated bash script and
// the applet:
//
//	mybin completionbash --cword $COMP_CWORD -- "${COMP_WORDS[@]}"
//
// The raw COMP_WORDS arrive verbatim as positionals — command word
// included, =-splits unrepaired — with COMP_CWORD locating the token
// being completed. Everything is argument-only (env:"-"): the query is
// per-keystroke transport, not configuration.
type config struct {
	Script bool `json:"script" arg:"script" env:"-" usage:"print the bash completion script to stdout and exit"`
	CWord  int  `json:"cword" arg:"cword" env:"-" usage:"index of the word being completed within the raw completion words"`
}
