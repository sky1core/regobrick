package regobrick

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Concurrency stress tests for the documented-safe usage pattern: configure
// decimal arithmetic (and register builtins) once at startup, then evaluate
// concurrently. Run under -race (as the CI test job does) these verify that
// the evaluation read path — decimalConfig, the builtin registry, prepared
// queries — is free of data races. Reconfiguring concurrently with evaluation
// is documented as unsafe and is intentionally not exercised.

func init() {
	RegisterBuiltin2[string, int, string]("stress_repeat", func(_ rego.BuiltinContext, s string, n int) (string, error) {
		out := ""
		for i := 0; i < n; i++ {
			out += s
		}
		return out, nil
	})
}

func TestConcurrentEval_DecimalOperators(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	module := `package test
import rego.v1
result := (input.a * input.b) + (input.a / input.b)`
	pq, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(context.Background())
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	workers := runtime.GOMAXPROCS(0) * 2
	const evalsPerWorker = 100

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			ctx := context.Background()
			for i := 0; i < evalsPerWorker; i++ {
				// Distinct inputs per worker so cross-goroutine result mixups
				// would surface as value mismatches, not just races.
				a := fmt.Sprintf("%d.5", w+1)
				input := map[string]any{"a": Number(a), "b": Number("2")}
				rs, err := pq.Eval(ctx, rego.EvalInput(input))
				if err != nil {
					errs <- fmt.Errorf("worker %d: eval: %w", w, err)
					return
				}
				if len(rs) == 0 || len(rs[0].Expressions) == 0 {
					errs <- fmt.Errorf("worker %d: undefined result", w)
					return
				}
				got := rs[0].Expressions[0].Value.(json.Number).String()
				// (w+1).5 * 2 + (w+1).5 / 2
				af := float64(w+1) + 0.5
				want := json.Number(fmt.Sprintf("%v", af*2+af/2)).String()
				if got != want {
					errs <- fmt.Errorf("worker %d: got %s, want %s", w, got, want)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestConcurrentEval_StringCoercion(t *testing.T) {
	// Coercion ON is a distinct read path of the process-global decimalConfig;
	// the config is written once here, before any concurrent evaluation starts.
	enableStringCoercion(t)

	module := `package test
import rego.v1
result := input.qty - input.pos`
	pq, err := rego.New(
		rego.Query("data.test.result"),
		rego.Module("test.rego", module),
	).PrepareForEval(context.Background())
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	workers := runtime.GOMAXPROCS(0) * 2
	const evalsPerWorker = 100

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			ctx := context.Background()
			input := map[string]any{"qty": fmt.Sprintf("%d.75", w+1), "pos": "0.5"}
			want := fmt.Sprintf("%d.25", w+1)
			for i := 0; i < evalsPerWorker; i++ {
				rs, err := pq.Eval(ctx, rego.EvalInput(input))
				if err != nil {
					errs <- fmt.Errorf("worker %d: eval: %w", w, err)
					return
				}
				if len(rs) == 0 || len(rs[0].Expressions) == 0 {
					errs <- fmt.Errorf("worker %d: undefined result", w)
					return
				}
				if got := rs[0].Expressions[0].Value.(json.Number).String(); got != want {
					errs <- fmt.Errorf("worker %d: got %s, want %s", w, got, want)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestConcurrentEval_CustomBuiltinAndPrepare(t *testing.T) {
	ensureDecimalArithmeticEnabled()

	// Concurrent PrepareForEval + Eval: each worker builds its own prepared
	// query over a custom builtin, then evaluates it repeatedly.
	workers := runtime.GOMAXPROCS(0) * 2
	const evalsPerWorker = 20

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			ctx := context.Background()
			module := fmt.Sprintf(`package test
import rego.v1
result := stress_repeat("x", %d)`, w+1)
			pq, err := rego.New(
				rego.Query("data.test.result"),
				rego.Module("test.rego", module),
			).PrepareForEval(ctx)
			if err != nil {
				errs <- fmt.Errorf("worker %d: prepare: %w", w, err)
				return
			}
			want := ""
			for i := 0; i <= w; i++ {
				want += "x"
			}
			for i := 0; i < evalsPerWorker; i++ {
				rs, err := pq.Eval(ctx)
				if err != nil {
					errs <- fmt.Errorf("worker %d: eval: %w", w, err)
					return
				}
				if len(rs) == 0 || len(rs[0].Expressions) == 0 {
					errs <- fmt.Errorf("worker %d: undefined result", w)
					return
				}
				if got := rs[0].Expressions[0].Value.(string); got != want {
					errs <- fmt.Errorf("worker %d: got %q, want %q", w, got, want)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
