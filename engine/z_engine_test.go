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
	"reflect"
	"strings"
	"testing"

	"sxcli.dev/fw"
)

type fakeSource struct {
	applets  []string
	single   string // "" = multi-applet mode
	services []string
	infos    []fw.ArgInfo
	asked    string // applet id Arguments was called with
}

func (s *fakeSource) Applets() []string  { return s.applets }
func (s *fakeSource) Services() []string { return s.services }
func (s *fakeSource) SingleApplet() (string, bool) {
	return s.single, s.single != ""
}
func (s *fakeSource) Arguments(appletID string, args []string) ([]fw.ArgInfo, error) {
	s.asked = appletID
	return s.infos, nil
}

var (
	stringT = reflect.TypeOf("")
	boolT   = reflect.TypeOf(true)
)

// schema resembling: app with --config,-c (file), --log-level (enum),
// --tag (slice), --debug (bool), --disable (service ids)
func demoInfos() []fw.ArgInfo {
	return []fw.ArgInfo{
		{Service: "core", Long: "config", Short: "c", Usage: "config path", Type: stringT, Hint: fw.HintFile},
		{Service: "core", Long: "disable", Usage: "drop services", Type: stringT, IsSlice: true, Hint: fw.HintServiceID},
		{Service: "app", Long: "log-level", Usage: "verbosity", Type: stringT, Allowed: []any{"debug", "info", "warn"}},
		{Service: "app", Long: "tag", Usage: "labels", Type: stringT, IsSlice: true},
		{Service: "app", Long: "debug", Usage: "diagnostics", Type: boolT},
	}
}

func vals(cands []Candidate) string {
	var out []string
	for _, c := range cands {
		out = append(out, c.Value)
	}
	return strings.Join(out, ",")
}

func TestFirstWordCompletesPublicApplets(t *testing.T) {
	src := &fakeSource{applets: []string{"alpha", "beta"}}
	got := Complete(src, Query{Current: "a"})
	if vals(got) != "alpha" || got[0].Kind != KindApplet {
		t.Errorf("applet completion wrong: %v", got)
	}
	if all := Complete(src, Query{}); vals(all) != "alpha,beta" {
		t.Errorf("unfiltered applet completion wrong: %v", all)
	}
}

func TestBareSelectorTargetsApplet(t *testing.T) {
	src := &fakeSource{applets: []string{"alpha", "beta"}, infos: demoInfos()}
	got := Complete(src, Query{Words: []string{"beta"}, Current: "--log"})
	if src.asked != "beta" {
		t.Errorf("selector not consumed: asked %q", src.asked)
	}
	if vals(got) != "--log-level" {
		t.Errorf("argument completion wrong: %v", got)
	}
}

func TestSingleAppletModeNeedsNoSelector(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	got := Complete(src, Query{Current: "--ta"})
	if src.asked != "solo" {
		t.Errorf("single-applet target wrong: asked %q", src.asked)
	}
	if vals(got) != "--tag" {
		t.Errorf("argument completion wrong: %v", got)
	}
}

func TestUsedScalarSuppressedUsedSliceOffered(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	got := Complete(src, Query{Words: []string{"--log-level", "info", "--tag", "x"}, Current: "--"})
	joined := vals(got)
	if strings.Contains(joined, "--log-level") {
		t.Errorf("used scalar must be suppressed: %v", joined)
	}
	if !strings.Contains(joined, "--tag") {
		t.Errorf("used slice must stay offered: %v", joined)
	}
	if !strings.Contains(joined, "--config") {
		t.Errorf("unused scalar must be offered: %v", joined)
	}
}

func TestPendingAllowedDomain(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	got := Complete(src, Query{Words: []string{"--log-level"}, Current: "w"})
	if vals(got) != "warn" || got[0].Kind != KindValue {
		t.Errorf("domain completion wrong: %v", got)
	}
}

func TestPendingHintFileEmitsDirective(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	got := Complete(src, Query{Words: []string{"--config"}})
	if len(got) != 1 || got[0].Kind != KindFiles || got[0].Value != "" {
		t.Errorf("file directive wrong: %v", got)
	}
}

func TestPendingServiceIDsFromRegistry(t *testing.T) {
	src := &fakeSource{single: "solo", services: []string{"core", "logfile", "solo"}, infos: demoInfos()}
	got := Complete(src, Query{Words: []string{"--disable"}, Current: "lo"})
	if vals(got) != "logfile" || got[0].Kind != KindValue {
		t.Errorf("service id completion wrong: %v", got)
	}
	// the synthesized core leads Services() but is never a candidate:
	// nobody can disable, enable or override the core
	if got := Complete(src, Query{Words: []string{"--disable"}}); vals(got) != "logfile,solo" {
		t.Errorf("core must be filtered from service references: %v", got)
	}
}

func TestBoolNeverPendingButEqualsCompletes(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	if got := Complete(src, Query{Words: []string{"--debug"}}); len(got) != 0 {
		t.Errorf("bool must not leave a pending value: %v", got)
	}
	got := Complete(src, Query{Current: "--debug=t"})
	if vals(got) != "true" {
		t.Errorf("bool = completion wrong: %v", got)
	}
}

func TestJoinedValueDoesNotPend(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	got := Complete(src, Query{Words: []string{"--log-level=info"}, Current: "--c"})
	if vals(got) != "--config" {
		t.Errorf("=-joined word must not pend: %v", got)
	}
}

func TestShortBundlePends(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	got := Complete(src, Query{Words: []string{"-c"}})
	if len(got) != 1 || got[0].Kind != KindFiles {
		t.Errorf("short pending value wrong: %v", got)
	}
}

func TestPositionalLandIsSilent(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	if got := Complete(src, Query{Words: []string{"--"}, Current: "--c"}); len(got) != 0 {
		t.Errorf("no candidates after --: %v", got)
	}
}

func TestFreshBareWordIsSilent(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	if got := Complete(src, Query{Words: []string{"--debug"}, Current: "positional"}); len(got) != 0 {
		t.Errorf("bare word yields shell default, not candidates: %v", got)
	}
}

func TestArgCandidatesCarryDocs(t *testing.T) {
	src := &fakeSource{single: "solo", infos: demoInfos()}
	got := Complete(src, Query{Current: "--log"})
	if len(got) != 1 || got[0].Doc != "verbosity" {
		t.Errorf("usage must flow into Doc: %+v", got)
	}
}

func TestBarePositionalSilencesArguments(t *testing.T) {
	// the strict parser refuses arguments after a pending bare token
	// ("positionals must come last") — offering names there would
	// complete straight into a parse error
	src := &fakeSource{single: "solo", infos: demoInfos()}
	if got := Complete(src, Query{Words: []string{"datafile"}, Current: "--"}); len(got) != 0 {
		t.Errorf("names after a bare positional must not be offered: %v", got)
	}
	if got := Complete(src, Query{Words: []string{"datafile"}, Current: "--log-level="}); len(got) != 0 {
		t.Errorf("joined values after a bare positional must not be offered: %v", got)
	}
	// a bare word that is a pending argument's VALUE is not positional
	src2 := &fakeSource{single: "solo", infos: demoInfos()}
	if got := Complete(src2, Query{Words: []string{"--log-level", "info"}, Current: "--"}); len(got) == 0 {
		t.Error("a consumed value must not silence completion")
	}
}
