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
func Complete(src Source, q Query) []Candidate {
	panic("engine: not implemented — implementation is the next design discussion")
}
