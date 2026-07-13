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
	"bytes"
	"reflect"
	"strings"
	"testing"

	sxclifw "sxcli.dev/fw"
)

type fakeSource struct {
	applets  []string
	single   string
	services []string
	infos    []sxclifw.ArgInfo
	asked    string
	words    []string
}

func (s *fakeSource) Applets() []string  { return s.applets }
func (s *fakeSource) Services() []string { return s.services }
func (s *fakeSource) SingleApplet() (string, bool) {
	return s.single, s.single != ""
}
func (s *fakeSource) Arguments(appletID string, args []string) ([]sxclifw.ArgInfo, error) {
	s.asked = appletID
	s.words = args
	return s.infos, nil
}

var stringT = reflect.TypeOf("")

func demoInfos() []sxclifw.ArgInfo {
	return []sxclifw.ArgInfo{
		{Service: "core", Long: "config", Short: "c", Usage: "config path", Type: stringT, Hint: sxclifw.HintFile},
		{Service: "cat", Long: "log-level", Usage: "verbosity", Type: stringT, Allowed: []any{"debug", "info"}},
	}
}

func TestReassembleJoinsEqualsSplits(t *testing.T) {
	words, cur := reassemble([]string{"bin", "--debug", "=", "fal"}, 3)
	if strings.Join(words, " ") != "bin --debug=fal" || cur != 1 {
		t.Errorf("reassembly wrong: %v cur=%d", words, cur)
	}
	words, cur = reassemble([]string{"bin", "--tag", "=", ""}, 3)
	if strings.Join(words, " ") != "bin --tag=" || cur != 1 {
		t.Errorf("trailing = reassembly wrong: %q cur=%d", words, cur)
	}
	words, cur = reassemble([]string{"bin", "--config", "x.json"}, 2)
	if strings.Join(words, " ") != "bin --config x.json" || cur != 2 {
		t.Errorf("plain words must pass through: %v cur=%d", words, cur)
	}
}

func TestAnswerBakedApplet(t *testing.T) {
	src := &fakeSource{applets: []string{"cat", "ls"}, infos: demoInfos()}
	var out bytes.Buffer
	answer(&out, src, "cat", 1, []string{"cat", "--lo"})
	if src.asked != "cat" {
		t.Errorf("baked applet not targeted: %q", src.asked)
	}
	if out.String() != "--log-level\n" {
		t.Errorf("answer wrong: %q", out.String())
	}
}

func TestAnswerSelectorMode(t *testing.T) {
	src := &fakeSource{applets: []string{"cat", "ls"}}
	var out bytes.Buffer
	answer(&out, src, "", 1, []string{"mybin", ""})
	if out.String() != "cat\nls\n" {
		t.Errorf("applet-name completion wrong: %q", out.String())
	}
}

func TestAnswerFileDirective(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	var out bytes.Buffer
	answer(&out, src, "", 2, []string{"solo", "--config", ""})
	if out.String() != "\x01files\n" {
		t.Errorf("file directive wrong: %q", out.String())
	}
}

func TestAnswerEqualsValue(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	var out bytes.Buffer
	// bash tore --log-level=in into three words; cursor on the value
	answer(&out, src, "", 3, []string{"solo", "--log-level", "=", "in"})
	if out.String() != "info\n" {
		t.Errorf("= value completion wrong: %q", out.String())
	}
}

func TestAnswerCursorPastWords(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	var out bytes.Buffer
	// fresh word at end of line: cword beyond the transported words
	answer(&out, src, "", 2, []string{"solo", "--config"})
	if out.String() != "\x01files\n" {
		t.Errorf("pending value at fresh word wrong: %q", out.String())
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
	if !strings.Contains(text, "complete -o default -F _sxcli_srv srv") {
		t.Errorf("registration line wrong:\n%s", text)
	}
}

func TestScriptAppletSymlinkBakesTarget(t *testing.T) {
	src := &fakeSource{applets: []string{"cat", "ls"}}
	var out bytes.Buffer
	script(&out, src, "./cat")
	text := out.String()
	if !strings.Contains(text, "completionbash --applet cat --cword") {
		t.Errorf("symlink script must bake the applet:\n%s", text)
	}
	if !strings.Contains(text, "complete -o default -F _sxcli_cat cat") {
		t.Errorf("registration line wrong:\n%s", text)
	}
}

func TestScriptRealBinaryKeepsSelectorLogic(t *testing.T) {
	src := &fakeSource{applets: []string{"cat", "ls"}}
	var out bytes.Buffer
	script(&out, src, "/opt/tools/my-box.exe")
	text := out.String()
	if strings.Contains(text, "--applet") {
		t.Errorf("real-binary script must not bake a target:\n%s", text)
	}
	if !strings.Contains(text, "complete -o default -F _sxcli_my_box my-box") {
		t.Errorf("sanitized function name wrong:\n%s", text)
	}
}
