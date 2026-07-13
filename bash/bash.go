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

package bash

import (
	sxclifw "sxcli.dev/fw"
)

func init() {
	c := &Completion{}
	sxclifw.Register("completionbash", c,
		sxclifw.System(),
		sxclifw.WithConfig(&c.cfg),
		sxclifw.WithMetadata(&sxclifw.Metadata{
			Description: "bash completion for this binary: --script prints the registration script, completion queries arrive as raw COMP_WORDS/COMP_CWORD and are answered on stdout",
		}),
	)
}

// Configured validates nothing: the query fields are per-invocation
// transport with no illegal states the parser does not already refuse.
func (c *Completion) Configured() error { return nil }

// Run answers the operation selected by the arguments.
func (c *Completion) Run() int {
	panic("completionbash: not implemented — implementation is the next design discussion")
}
