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

// Integration smoke tests: the test binary re-execs itself as a real
// fw application (fw's own x_ personality pattern) and the queries the
// generated script would send are asserted end to end. One test
// sources the generated script under a real zsh with the completion
// machinery stubbed.
package zsh_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "sxcli.dev/completion/zsh"
	"sxcli.dev/fw"
)

const personality = "SXCLI_COMPLETION_ZSH_SMOKE"

type smokeCfg struct {
	Level string `json:"level" arg:"level" usage:"verbosity"`
	Out   string `json:"out" arg:"out" usage:"log target"`
}

type smokeApplet struct{ cfg smokeCfg }

func (s *smokeApplet) Configured() error { return nil }
func (s *smokeApplet) Run() int          { return 0 }

func TestMain(m *testing.M) {
	if os.Getenv(personality) == "1" {
		fw.NewRegistration("example.com/smoke/srv", func() *smokeApplet {
			return &smokeApplet{cfg: smokeCfg{Level: "info", Out: "unix:/dev/log"}}
		}, func(s *smokeApplet) *smokeCfg { return &s.cfg }).
			Alias("srv").
			Metadata(&fw.Metadata{
				Fields: map[string]any{
					"Level": fw.FieldMetadata[string]{Allowed: []string{"debug", "info", "warn"}},
					"Out":   fw.FieldMetadata[string]{Allowed: []string{"unix:/dev/log", "tcp:remote"}},
				},
			}).
			Register()
		fw.Main() // never returns
	}
	os.Exit(m.Run())
}

// query re-execs the test binary as the fw application and returns
// completionzsh's stdout.
func query(t *testing.T, args ...string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), personality+"=1")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("re-exec failed: %v\nstderr:\n%s", err, errb.String())
	}
	return out.String()
}

func TestSmokeScriptEmission(t *testing.T) {
	text := query(t, "completionzsh", "--script")
	if !strings.Contains(text, "compdef _sxcli_") {
		t.Errorf("compdef line missing:\n%s", text)
	}
	if strings.Contains(text, "--applet") {
		t.Errorf("single-applet script must not bake a target:\n%s", text)
	}
}

func TestSmokeArgumentNamesCarryDescriptions(t *testing.T) {
	out := query(t, "completionzsh", "--cword", "1", "--current", "--le", "--", "srv", "--le")
	if out != "--level:verbosity\n" {
		t.Errorf("describe pair wrong: %q", out)
	}
}

func TestSmokeEnumValues(t *testing.T) {
	out := query(t, "completionzsh", "--cword", "2", "--current", "", "--", "srv", "--level", "")
	if out != "debug\ninfo\nwarn\n" {
		t.Errorf("enum completion wrong: %q", out)
	}
}

func TestSmokeColonValueStaysWholeAndEscaped(t *testing.T) {
	// zsh never shreds words: the full token completes, colons escaped
	// for _describe
	out := query(t, "completionzsh", "--cword", "2", "--current", "unix:", "--", "srv", "--out", "unix:")
	if out != `unix\:/dev/log`+"\n" {
		t.Errorf("colon value wrong: %q", out)
	}
}

func TestSmokeFileDirective(t *testing.T) {
	out := query(t, "completionzsh", "--cword", "2", "--current", "", "--", "srv", "--config", "")
	if out != "\x01files\n" {
		t.Errorf("file directive wrong: %q", out)
	}
}

func TestSmokeServiceIDs(t *testing.T) {
	out := query(t, "completionzsh", "--cword", "2", "--current", "", "--", "srv", "--disable", "")
	if !strings.Contains(out, "srv\n") || !strings.Contains(out, "introspection\n") {
		t.Errorf("service id completion wrong: %q", out)
	}
}

// TestSmokeScriptUnderRealZsh sources the generated script in a real
// zsh, invokes the completion function with zsh's variables set, and
// reads what _describe would render — the stubs keep it headless.
func TestSmokeScriptUnderRealZsh(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh not available")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "comp.zsh")
	if werr := os.WriteFile(scriptPath, []byte(query(t, "completionzsh", "--script")), 0o600); werr != nil {
		t.Fatal(werr)
	}
	harness := filepath.Join(dir, "harness.zsh")
	if werr := os.WriteFile(harness, []byte(`
compdef()   { : }
_describe() { local -a items; items=("${(@P)2}"); print -rl -- "${(@)items}" }
_files()    { print -r -- "FILES${1:+ $1}" }
_default()  { print -r -- "DEFAULT" }
source "$1"
fn=$(grep -om1 '_sxcli_[A-Za-z0-9_]*' "$1")
shift
words=("$@")
CURRENT=${#words}
PREFIX="${words[CURRENT]}"
"$fn"
`), 0o600); werr != nil {
		t.Fatal(werr)
	}
	cmd := exec.Command(zsh, "-f", harness, scriptPath, exe, "--le")
	cmd.Env = append(os.Environ(), personality+"=1")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if rerr := cmd.Run(); rerr != nil {
		t.Fatalf("zsh harness failed: %v\nstderr:\n%s", rerr, errb.String())
	}
	if out.String() != "--level:verbosity\n" {
		t.Errorf("_describe pairs wrong: %q", out.String())
	}
}
