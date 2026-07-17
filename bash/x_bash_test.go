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
// generated script would send are asserted end to end — real registry,
// real Introspector, real engine. One test sources the generated
// script under a real bash with the completion machinery stubbed.
package bash_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "sxcli.dev/completion/bash"
	sxclifw "sxcli.dev/fw"
)

const personality = "SXCLI_COMPLETION_BASH_SMOKE"

// stock bash COMP_WORDBREAKS, what the generated script forwards
const breaks = " \t\n\"'><=;|&(:"

type smokeCfg struct {
	Level string `json:"level" arg:"level" usage:"verbosity"`
	Out   string `json:"out" arg:"out" usage:"log target"`
}

type smokeApplet struct{ cfg smokeCfg }

func (s *smokeApplet) Configured() error { return nil }
func (s *smokeApplet) Run() int          { return 0 }

func TestMain(m *testing.M) {
	if os.Getenv(personality) == "1" {
		s := &smokeApplet{cfg: smokeCfg{Level: "info", Out: "unix:/dev/log"}}
		sxclifw.Register("srv", s,
			sxclifw.WithConfig(&s.cfg),
			sxclifw.WithMetadata(&sxclifw.Metadata{
				Fields: map[string]any{
					"Level": sxclifw.FieldMetadata[string]{Allowed: []string{"debug", "info", "warn"}},
					"Out":   sxclifw.FieldMetadata[string]{Allowed: []string{"unix:/dev/log", "tcp:remote"}},
				},
			}),
		)
		sxclifw.Main() // never returns
	}
	os.Exit(m.Run())
}

// query re-execs the test binary as the fw application and returns
// completionbash's stdout.
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
	text := query(t, "completionbash", "--script")
	if !strings.Contains(text, "complete -o default -F _sxcli_") {
		t.Errorf("registration line missing:\n%s", text)
	}
	if strings.Contains(text, "--applet") {
		t.Errorf("single-applet script must not bake a target:\n%s", text)
	}
}

func TestSmokeArgumentNames(t *testing.T) {
	out := query(t, "completionbash", "--cword", "1", "--line", "srv --le", "--breaks", breaks,
		"--", "srv", "--le")
	if out != "--level\n" {
		t.Errorf("argument completion wrong: %q", out)
	}
}

func TestSmokeEnumValues(t *testing.T) {
	out := query(t, "completionbash", "--cword", "2", "--line", "srv --level ", "--breaks", breaks,
		"--", "srv", "--level", "")
	if out != "debug\ninfo\nwarn\n" {
		t.Errorf("enum completion wrong: %q", out)
	}
}

func TestSmokeServiceIDs(t *testing.T) {
	out := query(t, "completionbash", "--cword", "2", "--line", "srv --disable ", "--breaks", breaks,
		"--", "srv", "--disable", "")
	if !strings.Contains(out, "srv\n") || !strings.Contains(out, "introspection\n") {
		t.Errorf("service id completion wrong: %q", out)
	}
}

func TestSmokeFileDirective(t *testing.T) {
	out := query(t, "completionbash", "--cword", "2", "--line", "srv --config ", "--breaks", breaks,
		"--", "srv", "--config", "")
	if out != "\x01files\n" {
		t.Errorf("file directive wrong: %q", out)
	}
}

func TestSmokeColonSegmentTrim(t *testing.T) {
	// typed: --out unix:/de<TAB>; bash shreds it into unix : /de and
	// will replace only the /de segment
	out := query(t, "completionbash", "--cword", "4", "--line", "srv --out unix:/de", "--breaks", breaks,
		"--", "srv", "--out", "unix", ":", "/de")
	if out != "/dev/log\n" {
		t.Errorf("colon segment trim wrong: %q", out)
	}
}

// TestSmokeScriptUnderRealBash sources the generated script in a real
// bash, invokes the completion function with bash's variables set, and
// reads COMPREPLY — the stubbed `complete` builtin keeps it headless.
func TestSmokeScriptUnderRealBash(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "comp.bash")
	if werr := os.WriteFile(scriptPath, []byte(query(t, "completionbash", "--script")), 0o600); werr != nil {
		t.Fatal(werr)
	}
	harness := filepath.Join(dir, "harness.bash")
	if werr := os.WriteFile(harness, []byte(`
complete() { :; }
source "$1"
fn=$(grep -om1 '_sxcli_[A-Za-z0-9_]*' "$1")
shift
COMP_WORDS=("$@")
COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))
COMP_LINE="${COMP_WORDS[*]}"
"$fn"
printf '%s\n' "${COMPREPLY[@]}"
`), 0o600); werr != nil {
		t.Fatal(werr)
	}
	cmd := exec.Command(bash, harness, scriptPath, exe, "--le")
	cmd.Env = append(os.Environ(), personality+"=1")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if rerr := cmd.Run(); rerr != nil {
		t.Fatalf("bash harness failed: %v\nstderr:\n%s", rerr, errb.String())
	}
	if out.String() != "--level\n" {
		t.Errorf("COMPREPLY wrong: %q", out.String())
	}
}
