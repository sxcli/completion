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

package zsh

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"sxcli.dev/fw"
)

type fakeSource struct {
	applets  []string
	single   string
	services []string
	infos    []fw.ArgInfo
	asked    string
	words    []string
}

func (s *fakeSource) Applets() []string  { return s.applets }
func (s *fakeSource) Services() []string { return s.services }
func (s *fakeSource) SingleApplet() (string, bool) {
	return s.single, s.single != ""
}
func (s *fakeSource) Arguments(appletID string, args []string) ([]fw.ArgInfo, error) {
	s.asked = appletID
	s.words = args
	return s.infos, nil
}

var stringT = reflect.TypeOf("")

func demoInfos() []fw.ArgInfo {
	return []fw.ArgInfo{
		{Service: "core", Long: "config", Short: "c", Usage: "config path", Type: stringT, Hint: fw.HintFile},
		{Service: "cat", Long: "out", Usage: "log target", Type: stringT, Allowed: []any{"unix:/dev/log", "tcp:remote"}},
		{Service: "cat", Long: "log-level", Usage: "verbosity", Type: stringT, Allowed: []any{"debug", "info"}},
	}
}

func TestAnswerDescribePairs(t *testing.T) {
	src := &fakeSource{applets: []string{"cat", "ls"}, infos: demoInfos()}
	var out bytes.Buffer
	answer(&out, src, config{Applet: "cat", CWord: 1, Current: "--lo"}, []string{"cat", "--lo"})
	if src.asked != "cat" {
		t.Errorf("baked applet not targeted: %q", src.asked)
	}
	if out.String() != "--log-level:verbosity\n" {
		t.Errorf("describe pair wrong: %q", out.String())
	}
}

func TestAnswerEscapesValueColons(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	var out bytes.Buffer
	// value position for --out: candidates carry colons, zsh does not
	// shred words, so the full token completes — colons escaped for
	// _describe, no description on values
	answer(&out, src, config{CWord: 2, Current: "unix:"}, []string{"solo", "--out", "unix:"})
	if out.String() != `unix\:/dev/log`+"\n" {
		t.Errorf("colon escaping wrong: %q", out.String())
	}
}

func TestAnswerSelectorMode(t *testing.T) {
	src := &fakeSource{applets: []string{"cat", "ls"}}
	var out bytes.Buffer
	answer(&out, src, config{CWord: 1}, []string{"mybin", ""})
	if out.String() != "cat\nls\n" {
		t.Errorf("applet-name completion wrong: %q", out.String())
	}
}

func TestAnswerFileDirective(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	var out bytes.Buffer
	answer(&out, src, config{CWord: 2}, []string{"solo", "--config", ""})
	if out.String() != "\x01files\n" {
		t.Errorf("file directive wrong: %q", out.String())
	}
	if strings.Join(src.words, " ") != "--config" {
		t.Errorf("words before cursor wrong: %v", src.words)
	}
}

func TestScriptSingleAppletBakesNothing(t *testing.T) {
	src := &fakeSource{single: "solo"}
	var out bytes.Buffer
	script(&out, src, "/usr/local/bin/srv")
	text := out.String()
	if strings.Contains(text, "--applet") {
		t.Errorf("single-applet script must not bake a target:\n%s", text)
	}
	if !strings.Contains(text, "compdef _sxcli_srv srv") {
		t.Errorf("compdef line wrong:\n%s", text)
	}
}

func TestScriptAppletSymlinkBakesTarget(t *testing.T) {
	src := &fakeSource{applets: []string{"cat", "ls"}}
	var out bytes.Buffer
	script(&out, src, "./cat")
	text := out.String()
	if !strings.Contains(text, "completionzsh --applet cat --cword") {
		t.Errorf("symlink script must bake the applet:\n%s", text)
	}
	if !strings.Contains(text, "compdef _sxcli_cat cat") {
		t.Errorf("compdef line wrong:\n%s", text)
	}
}

func TestScriptRealBinaryKeepsSelectorLogic(t *testing.T) {
	src := &fakeSource{applets: []string{"cat", "ls"}}
	var out bytes.Buffer
	script(&out, src, "/opt/tools/my-box")
	text := out.String()
	if strings.Contains(text, "--applet") {
		t.Errorf("real-binary script must not bake a target:\n%s", text)
	}
	if !strings.Contains(text, "compdef _sxcli_my_box my-box") {
		t.Errorf("sanitized function name wrong:\n%s", text)
	}
}

func TestAnswerRebuildsEqualsJoinedTokens(t *testing.T) {
	// zsh matches candidates against the whole $PREFIX: a value
	// completed through --name= must come back as the full token
	src := &fakeSource{single: "solo", infos: demoInfos()}
	var out bytes.Buffer
	answer(&out, src, config{CWord: 1, Current: "--log-level=de"}, []string{"solo", "--log-level=de"})
	if out.String() != "--log-level=debug\n" {
		t.Errorf("=-joined value must be rebuilt to the full token: %q", out.String())
	}
	// bools travel the same road
	src2 := &fakeSource{single: "solo", infos: []fw.ArgInfo{
		{Service: "solo", Long: "debug", Usage: "verbose", Type: reflect.TypeOf(true)},
	}}
	var out2 bytes.Buffer
	answer(&out2, src2, config{CWord: 1, Current: "--debug=t"}, []string{"solo", "--debug=t"})
	if out2.String() != "--debug=true\n" {
		t.Errorf("=-joined bool must be rebuilt too: %q", out2.String())
	}
	// an Allowed value containing '=' cuts at the FIRST =, the
	// engine's own semantic split
	src3 := &fakeSource{single: "solo", infos: []fw.ArgInfo{
		{Service: "solo", Long: "set", Usage: "kv", Type: stringT, Allowed: []any{"k=v"}},
	}}
	var out3 bytes.Buffer
	answer(&out3, src3, config{CWord: 1, Current: "--set=k"}, []string{"solo", "--set=k"})
	if out3.String() != "--set=k=v\n" {
		t.Errorf("first-= cut wrong: %q", out3.String())
	}
}
