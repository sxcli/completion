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

// Package script holds the generation policy shared by the shell
// adapters — the parts of --script emission that are about the
// framework's dispatch semantics, not about any shell. The engine
// stays ignorant of script generation; the adapters own their
// templates and encodings and come here for the decisions.
package script

import (
	"sxcli.dev/completion/internal/engine"
)

// BakedApplet decides the --applet value a generated script bakes in
// for the command name it serves — generation THROUGH a name decides
// for that name, once:
//
//   - single-applet mode: nothing baked ("*"); any name runs the sole
//     applet and the engine resolves it per query.
//   - the name is a public applet (a symlink in the busybox farm): the
//     name itself; dispatch rule 4 would run exactly it.
//   - anything else — the real binary's own name included: nothing
//     baked; selector logic stays live, matching dispatch rules 3–4.
func BakedApplet(src engine.Source, name string) string {
	baked := ""
	if _, single := src.SingleApplet(); !single {
		for _, id := range src.Applets() {
			if id == name {
				baked = name
			}
		}
	}
	return baked
}

// Sanitize turns an arbitrary command name into a shell function name
// suffix: anything outside [A-Za-z0-9] becomes an underscore.
func Sanitize(name string) string {
	out := []byte(name)
	for i := 0; i < len(out); i++ {
		ch := out[i]
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
			out[i] = '_'
		}
	}
	return string(out)
}
