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

// Package completion provides shell completion for binaries built
// on sxcli.dev/fw, as an ordinary ecosystem module: it consumes the
// framework's Introspector API and registers one System applet per
// supported shell (binary machinery invoked by the generated shell
// scripts, never typed by a person) — nothing here is, or needs to
// be, part of the core.
//
// Each shell lives in its own package and registers one service; a
// binary names the shells it accepts by ID, like every other sxcli
// service, and links only the shells it wants to support:
//
//	import (
//	    "sxcli.dev/completion/bash"
//	    "sxcli.dev/completion/zsh"
//	)
//
//	fw.Builder().Accept(bash.ID, zsh.ID /* , … */).Main()
//
// (Solo and fw.Main() binaries accept everything registered, so
// single-applet binaries just import and go.)
//
//   - The shared candidate computation lives in the public engine
//     package.
//   - The shared --script generation policy lives in the public
//     script package.
//   - The per-shell packages are thin adapters owning only their
//     shell's registration-script template and answer encoding.
//   - Third-party shell adapters build on the same two packages —
//     see the engine package documentation.
package completion
