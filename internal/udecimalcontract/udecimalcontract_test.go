// Package udecimalcontract verifies that regobrick.UseDecimalArithmetic
// fail-fasts when udecimal's process-global defaults no longer match the parsing
// contract regobrick documents (19 decimal places, no silent truncation).
//
// It exists as its own package purely for test-binary isolation. Exercising the
// assertion means calling udecimal.SetDefaultPrecision or
// udecimal.SetDefaultParseMode, which mutate package-level state for the whole
// process; running that inside regobrick's own test binary would change the
// parsing behavior every other test depends on, with the damage depending on
// test ordering. Each case is pushed one level further out, into a fresh child
// process (see TestMain), so that even the ordering between the cases here
// cannot matter and no case leaves a mutated global behind.
package udecimalcontract

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/quagmt/udecimal"
	"github.com/sky1core/regobrick"
)

// probeEnv names the udecimal mutation a child process should apply before
// calling UseDecimalArithmetic. Unset means "run the tests normally".
const probeEnv = "REGOBRICK_UDECIMAL_PROBE"

// completedMarker is printed by a child that reached the end of
// UseDecimalArithmetic without panicking. A marker is used rather than the exit
// code alone so that "the assertion accepted these settings" cannot be confused
// with an exit status produced by anything else.
const completedMarker = "REGOBRICK_PROBE_COMPLETED_WITHOUT_PANIC"

// panicPrefix is the message prefix UseDecimalArithmetic promises in its godoc
// and README; it is pinned here because callers are told to look for it.
const panicPrefix = "regobrick: udecimal default settings have been changed"

const (
	probeUntouched     = "untouched"
	probeLowPrecision  = "low-precision"
	probeTruncatedMode = "truncating-parse-mode"
)

func TestMain(m *testing.M) {
	mode := os.Getenv(probeEnv)
	if mode == "" {
		os.Exit(m.Run())
	}

	// Child process: apply one mutation, then call the real production entry
	// point and let it panic (or not) on its own.
	switch mode {
	case probeUntouched:
		// Deliberately mutate nothing: pins that the assertion accepts udecimal's
		// shipped defaults, so the failing cases below prove the assertion fires
		// on the mutation rather than being broken for every input.
	case probeLowPrecision:
		// Below 19: the 19-decimal-place probe stops parsing.
		udecimal.SetDefaultPrecision(10)
	case probeTruncatedMode:
		// Truncating mode: the 20-decimal-place probe starts parsing, which is
		// the silent-wrong-number case.
		udecimal.SetDefaultParseMode(udecimal.ParseModeTrunc)
	default:
		fmt.Fprintf(os.Stderr, "unknown %s value %q\n", probeEnv, mode)
		os.Exit(2)
	}

	regobrick.UseDecimalArithmetic()

	fmt.Println(completedMarker)
	os.Exit(0)
}

func TestUseDecimalArithmeticFailsFastOnChangedUdecimalDefaults(t *testing.T) {
	tests := []struct {
		name string
		mode string
		// wantPanicFragment is empty when the call is expected to succeed.
		wantPanicFragment string
	}{
		{
			name: "shipped udecimal defaults are accepted",
			mode: probeUntouched,
		},
		{
			name: "SetDefaultPrecision below 19 is rejected",
			mode: probeLowPrecision,
			// The 19-decimal-place probe must fail to parse.
			wantPanicFragment: "whose 19 decimal places the documented contract requires to be supported, failed",
		},
		{
			name: "SetDefaultParseMode truncation is rejected",
			mode: probeTruncatedMode,
			// The 20-decimal-place probe must now parse instead of erroring.
			wantPanicFragment: "succeeded instead of failing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runProbe(tt.mode)

			if tt.wantPanicFragment == "" {
				if err != nil {
					t.Fatalf("UseDecimalArithmetic rejected udecimal's untouched defaults.\n"+
						"That means the assertion's own probe values no longer match udecimal's shipped "+
						"behavior — likely a udecimal upgrade changed the default precision or parse mode, "+
						"so assertUdecimalDefaults in operator.go needs revisiting.\nchild error: %v\nchild output:\n%s",
						err, out)
				}
				if !strings.Contains(out, completedMarker) {
					t.Fatalf("child exited cleanly but never reached the end of UseDecimalArithmetic; output:\n%s", out)
				}
				return
			}

			if err == nil {
				t.Fatalf("UseDecimalArithmetic returned normally after udecimal's defaults were changed (%s).\n"+
					"The documented parsing contract is silently broken in this state, so the call must panic "+
					"instead of proceeding.\nchild output:\n%s", tt.mode, out)
			}
			if strings.Contains(out, completedMarker) {
				t.Fatalf("child reported completing UseDecimalArithmetic despite the changed defaults; output:\n%s", out)
			}

			// A panic, not some other failure to start.
			if !strings.Contains(out, "panic:") {
				t.Fatalf("child failed without panicking (%v); the assertion is expected to panic. output:\n%s", err, out)
			}
			if !strings.Contains(out, panicPrefix) {
				t.Errorf("panic message does not start with the documented prefix %q.\n"+
					"godoc and README tell callers to recognize it by that prefix, so changing it is a "+
					"documented-behavior change.\noutput:\n%s", panicPrefix, out)
			}
			if !strings.Contains(out, tt.wantPanicFragment) {
				t.Errorf("panic message does not identify this failure mode: want it to contain %q.\n"+
					"Both probes panic with the same prefix, so the differing half is what tells an "+
					"operator which udecimal setting was changed.\noutput:\n%s", tt.wantPanicFragment, out)
			}

			// The panic must come from the assertion, not from somewhere deeper
			// that happened to fail under the mutated settings.
			if !strings.Contains(out, "regobrick.assertUdecimalDefaults") {
				t.Errorf("panic stack does not name assertUdecimalDefaults, so the failure came from "+
					"somewhere other than the entry-point check.\noutput:\n%s", out)
			}
		})
	}
}

// runProbe re-executes this test binary with probeEnv set, so the udecimal
// mutation lands in a throwaway process. It returns the child's combined output
// and its exit error (nil when the child exited 0).
func runProbe(mode string) (string, error) {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), probeEnv+"="+mode)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
