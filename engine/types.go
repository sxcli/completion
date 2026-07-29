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

package engine

import (
	"sxcli.dev/fw/system"
)

// Source is one target-scoped introspection view — the system
// vocabulary IS the engine contract, by alias: the view the framework
// hands out is exactly what completion consumes. Tests satisfy it
// with a fake.
type Source = system.Introspector

// System hands out target-scoped views by dispatch name: "" is the
// binary view (applet listing), an unknown name is nil ("offer
// nothing"). The framework's system.System satisfies it structurally
// — this interface is the honest ledger of what the module needs.
type System interface {
	Introspector(applet string) Source
}

// Query is one completion request, already decoded from the shell's
// transport by the calling adapter.
type Query struct {
	// Applet is the target applet's name (alias or id — the core
	// resolves both); "" means the binary decides —
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
	// KindApplet is an applet's primary alias completing the first
	// selector word.
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
