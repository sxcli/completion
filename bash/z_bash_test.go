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
		{Service: "cat", Long: "log-level", Usage: "verbosity", Type: stringT, Allowed: []any{"debug", "info"}},
	}
}

// ask builds the query config the generated script would send.
func ask(applet string, cword int, line string) config {
	return config{Applet: applet, CWord: cword, Line: line, Breaks: defaultBreaks}
}

func TestReassembleJoinsEqualsSplits(t *testing.T) {
	words, cur := reassemble([]string{"bin", "--debug", "=", "fal"}, 3, "bin --debug=fal", defaultBreaks)
	if strings.Join(words, " ") != "bin --debug=fal" || cur != 1 {
		t.Errorf("reassembly wrong: %v cur=%d", words, cur)
	}
	words, cur = reassemble([]string{"bin", "--tag", "=", ""}, 3, "bin --tag=", defaultBreaks)
	if strings.Join(words, " ") != "bin --tag=" || cur != 1 {
		t.Errorf("trailing = reassembly wrong: %q cur=%d", words, cur)
	}
	words, cur = reassemble([]string{"bin", "--config", "x.json"}, 2, "bin --config x.json", defaultBreaks)
	if strings.Join(words, " ") != "bin --config x.json" || cur != 2 {
		t.Errorf("plain words must pass through: %v cur=%d", words, cur)
	}
}

func TestReassembleColonUsesLineAdjacency(t *testing.T) {
	// glued: unix:/dev/log is one token
	words, cur := reassemble([]string{"bin", "--out", "unix", ":", "/dev/log"}, 4, "bin --out unix:/dev/log", defaultBreaks)
	if strings.Join(words, " ") != "bin --out unix:/dev/log" || cur != 2 {
		t.Errorf("glued colon reassembly wrong: %v cur=%d", words, cur)
	}
	// spaced: :8080 stands alone, separate from --addr
	words, cur = reassemble([]string{"bin", "--addr", ":", "8080"}, 3, "bin --addr :8080", defaultBreaks)
	if strings.Join(words, " ") != "bin --addr :8080" || cur != 2 {
		t.Errorf("spaced colon reassembly wrong: %v cur=%d", words, cur)
	}
	// a shell with ":" removed from its breaks never split; passthrough
	words, cur = reassemble([]string{"bin", "unix:/dev/log"}, 1, "bin unix:/dev/log", " \t\n\"'><=;|&(")
	if strings.Join(words, " ") != "bin unix:/dev/log" || cur != 1 {
		t.Errorf("colon-free breaks must pass words through: %v cur=%d", words, cur)
	}
}

func TestAnswerBakedApplet(t *testing.T) {
	src := &fakeSource{applets: []string{"cat", "ls"}, infos: demoInfos()}
	var out bytes.Buffer
	answer(&out, src, ask("cat", 1, "cat --lo"), []string{"cat", "--lo"})
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
	answer(&out, src, ask("", 1, "mybin "), []string{"mybin", ""})
	if out.String() != "cat\nls\n" {
		t.Errorf("applet-name completion wrong: %q", out.String())
	}
}

func TestAnswerFileDirective(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	var out bytes.Buffer
	answer(&out, src, ask("", 2, "solo --config "), []string{"solo", "--config", ""})
	if out.String() != "\x01files\n" {
		t.Errorf("file directive wrong: %q", out.String())
	}
}

func TestAnswerEqualsValue(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	var out bytes.Buffer
	// bash tore --log-level=in into three words; cursor on the value.
	// bash replaces only the post-= segment, so the value comes bare.
	answer(&out, src, ask("", 3, "solo --log-level=in"), []string{"solo", "--log-level", "=", "in"})
	if out.String() != "info\n" {
		t.Errorf("= value completion wrong: %q", out.String())
	}
}

func TestAnswerColonValueIsSegmentTrimmed(t *testing.T) {
	src := &fakeSource{single: "solo", infos: []fw.ArgInfo{
		{Service: "solo", Long: "out", Usage: "sink", Type: stringT, Allowed: []any{"unix:/dev/log", "tcp:remote"}},
	}}
	var out bytes.Buffer
	// typed: --out unix:/de<TAB>; bash's replaceable segment is "/de"
	answer(&out, src, ask("", 4, "solo --out unix:/de"), []string{"solo", "--out", "unix", ":", "/de"})
	if out.String() != "/dev/log\n" {
		t.Errorf("colon segment trim wrong: %q", out.String())
	}
}

func TestAnswerEqualsRebuiltWhenShellDoesNotBreakOnIt(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	var out bytes.Buffer
	// a shell whose breaks lack "=": the whole word gets replaced, so
	// the full --name=value token must be printed
	cfg := config{CWord: 1, Line: "solo --log-level=in", Breaks: " \t\n\"'><;|&(:"}
	answer(&out, src, cfg, []string{"solo", "--log-level=in"})
	if out.String() != "--log-level=info\n" {
		t.Errorf("full-token rebuild wrong: %q", out.String())
	}
}

func TestAnswerCursorPastWords(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	var out bytes.Buffer
	// fresh word at end of line: cword beyond the transported words
	answer(&out, src, ask("", 2, "solo --config "), []string{"solo", "--config"})
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
