// Copyright 2026 Christopher O'Connell
// SPDX-License-Identifier: Apache-2.0
package prudence

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// budgetSpec is a small authored module whose acquisition walks well over
// fifty syntax-tree nodes and well under the default budget.
const budgetSpec = "---- MODULE Budget ----\nEXTENDS Naturals\nVARIABLE x\nA == x + 1\nB == A * 2\nC == IF B > 3 THEN A ELSE B\nInit == x = 0\nNext == x' = C\n====\n"

var acquisitionLimitText = regexp.MustCompile(`^semantic analysis is incomplete: the module acquisition (syntax-node|module discovery|module resolution|cycle check|dependency order) pass made (\d+) visits, exceeding the budget of (\d+); set Options\.MaxAcquisitionVisits to a larger value to raise it$`)

// sameResult compares results apart from the call-owned Tree pointer.
func sameResult(a, b Result) bool {
	a.Tree, b.Tree = nil, nil
	return reflect.DeepEqual(a, b)
}

func checkVisits(t *testing.T, path, text string, visits int, resolve Resolver) Result {
	t.Helper()
	r, err := Check(context.Background(), Request{Path: path, Text: []byte(text), Options: Options{MaxAcquisitionVisits: visits}, Resolve: resolve})
	if err != nil {
		t.Fatalf("MaxAcquisitionVisits=%d: %v", visits, err)
	}
	return r
}

// acquisitionStop returns the pass of r's single CodeAcquisitionLimit
// diagnostic after checking its shape, or "" when r has none.
func acquisitionStop(t *testing.T, r Result, path string, limit int) string {
	t.Helper()
	for _, d := range r.Diagnostics {
		if d.Code != CodeAcquisitionLimit {
			continue
		}
		m := acquisitionLimitText.FindStringSubmatch(d.Message)
		if r.Completion != CompletionIncomplete || len(r.Diagnostics) != 1 || m == nil || m[2] != fmt.Sprint(limit+1) || m[3] != fmt.Sprint(limit) ||
			d.Severity != SeverityInfo || d.Phase != LevelSemantic || d.Path != path || d.Span != (Span{}) {
			t.Fatalf("limit %d: %+v", limit, r)
		}
		return m[1]
	}
	return ""
}

func TestCheckAcquisitionBudgetDefault(t *testing.T) {
	if DefaultMaxAcquisitionVisits != 1000000 {
		t.Fatal(DefaultMaxAcquisitionVisits)
	}
	r, err := Check(context.Background(), Request{Path: "Budget.tla", Text: []byte(budgetSpec)})
	requireCompleteCheck(t, r, err, LevelComplete)
	if !sameResult(checkVisits(t, "Budget.tla", budgetSpec, 0, nil), r) || !sameResult(checkVisits(t, "Budget.tla", budgetSpec, DefaultMaxAcquisitionVisits, nil), r) {
		t.Fatal("zero and DefaultMaxAcquisitionVisits must equal the default")
	}
}

func TestCheckAcquisitionBudgetLowCap(t *testing.T) {
	for _, limit := range []int{1, 50} {
		r := checkVisits(t, "Budget.tla", budgetSpec, limit, nil)
		if pass := acquisitionStop(t, r, "Budget.tla", limit); pass != "syntax-node" {
			t.Fatalf("limit %d: pass %q: %+v", limit, pass, r)
		}
	}
	want := "semantic analysis is incomplete: the module acquisition syntax-node pass made 51 visits, exceeding the budget of 50; set Options.MaxAcquisitionVisits to a larger value to raise it"
	if got := checkVisits(t, "Budget.tla", budgetSpec, 50, nil).Diagnostics[0].Message; got != want {
		t.Fatal(got)
	}
	// Syntax-only checking performs no acquisition and ignores the budget.
	r, err := Check(context.Background(), Request{Stage: LevelSyntax, Path: "Budget.tla", Text: []byte(budgetSpec), Options: Options{MaxAcquisitionVisits: 1}})
	requireCompleteCheck(t, r, err, LevelSyntax)
}

func TestCheckAcquisitionBudgetAdequateCap(t *testing.T) {
	root := "---- MODULE Root ----\nEXTENDS Dep\nASSUME D = 1\n====\n"
	dep := "---- MODULE Dep ----\nD == 1\nE == <<D, D, D, D, D, D, D, D, D, D, D, D, D, D, D, D>>\n====\n"
	resolve := func(_ context.Context, name string) (Source, error) {
		if name == "Dep" {
			return Source{Path: "Dep.tla", Text: []byte(dep), Origin: OriginWorkspace}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	full, err := Check(context.Background(), Request{Path: "Root.tla", Text: []byte(root), Resolve: resolve})
	requireCompleteCheck(t, full, err, LevelComplete)
	// The smallest admitted budget T completes identically; T-1 stops with
	// the typed diagnostic.
	lo, hi := 1, 100000
	for lo < hi {
		mid := (lo + hi) / 2
		if checkVisits(t, "Root.tla", root, mid, resolve).Completion == CompletionComplete {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	if !sameResult(checkVisits(t, "Root.tla", root, lo, resolve), full) {
		t.Fatal("adequate budget must match the default result")
	}
	if acquisitionStop(t, checkVisits(t, "Root.tla", root, lo-1, resolve), "Root.tla", lo-1) == "" {
		t.Fatalf("budget %d must stop on the acquisition budget", lo-1)
	}
}

// passesSeen checks text at every budget in 1..max and returns the passes
// whose budget stopped it.
func passesSeen(t *testing.T, path, text string, max int, resolve Resolver) map[string]bool {
	t.Helper()
	seen := map[string]bool{}
	for v := 1; v <= max; v++ {
		if pass := acquisitionStop(t, checkVisits(t, path, text, v, resolve), path, v); pass != "" {
			seen[pass] = true
		}
	}
	return seen
}

func TestCheckAcquisitionBudgetDiscoveryVisits(t *testing.T) {
	// Sibling inner modules make discovery's binding visits grow faster than
	// the syntax-node walk, so the visit budget stops discovery once the
	// node walk fits.
	var b strings.Builder
	b.WriteString("---- MODULE Root ----\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "---- MODULE M%d ----\n====\n", i)
	}
	b.WriteString("====\n")
	seen := passesSeen(t, "Root.tla", b.String(), 1500, nil)
	if !seen["syntax-node"] || !seen["module discovery"] {
		t.Fatal(seen)
	}
}

func TestCheckAcquisitionBudgetResolutionVisits(t *testing.T) {
	var refs []string
	files := map[string]string{}
	for i := 0; i < 30; i++ {
		n := fmt.Sprintf("A%d", i)
		refs = append(refs, n)
		files[n] = "---- MODULE " + n + " ----\n====\n"
	}
	resolve := func(_ context.Context, name string) (Source, error) {
		if text, ok := files[name]; ok {
			return Source{Path: name + ".tla", Text: []byte(text), Origin: OriginWorkspace}, nil
		}
		return Source{}, ErrModuleNotFound
	}
	seen := passesSeen(t, "Root.tla", "---- MODULE Root ----\nEXTENDS "+strings.Join(refs, ", ")+"\n====\n", 1500, resolve)
	if !seen["module resolution"] {
		t.Fatal(seen)
	}
}

func TestCheckAcquisitionBudgetNegative(t *testing.T) {
	r, err := Check(context.Background(), Request{Path: "Budget.tla", Text: []byte(budgetSpec), Options: Options{MaxAcquisitionVisits: -1}})
	if err == nil || err.Error() != "prudence: negative maximum acquisition visits" || r.Completion != CompletionIncomplete || len(r.Diagnostics) != 0 {
		t.Fatalf("%+v %v", r, err)
	}
	// Parse does not use the budget.
	p, err := Parse(context.Background(), "Budget.tla", []byte(budgetSpec), Options{MaxAcquisitionVisits: -1})
	if err != nil || p.Completion != CompletionComplete {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestCheckAcquisitionBudgetUnrelatedIncomplete(t *testing.T) {
	// An override leaves the other incomplete stops on CodeIncomplete.
	for _, visits := range []int{0, 100000} {
		r := checkVisits(t, "Root.tla", "---- MODULE Root ----\nEXTENDS A\n====\n", visits, nil)
		if r.Completion != CompletionIncomplete || len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != CodeIncomplete {
			t.Fatalf("%+v", r)
		}
		r = checkVisits(t, "Root.tla", "---- MODULE Root ----\n---- MODULE Inner ----\nk == 1\n====\nV == LET I == INSTANCE Inner IN 1\nASSUME V!:!I!k = 1\n====\n", visits, nil)
		if r.Completion != CompletionIncomplete || r.Diagnostics[len(r.Diagnostics)-1].Code != CodeIncomplete {
			t.Fatalf("%+v", r)
		}
		r, err := Check(context.Background(), Request{Path: "e1", Text: []byte(e1Spec), Options: Options{MaxDepth: 3, MaxAcquisitionVisits: visits}})
		if err != nil || r.Completion != CompletionIncomplete {
			t.Fatalf("%+v %v", r, err)
		}
		for _, d := range r.Diagnostics {
			if d.Code == CodeAcquisitionLimit {
				t.Fatalf("%+v", r)
			}
		}
	}
}

func TestCheckAcquisitionBudgetPerCall(t *testing.T) {
	low := checkVisits(t, "Budget.tla", budgetSpec, 50, nil)
	full := checkVisits(t, "Budget.tla", budgetSpec, 0, nil)
	var wg sync.WaitGroup
	errs := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			visits, want := 50, low
			if i%2 == 1 {
				visits, want = 0, full
			}
			r, err := Check(context.Background(), Request{Path: "Budget.tla", Text: []byte(budgetSpec), Options: Options{MaxAcquisitionVisits: visits}})
			if err != nil || !sameResult(r, want) {
				errs <- fmt.Sprintf("call %d: %+v %v", i, r, err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	// Cancellation still wins over the budget.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := Check(ctx, Request{Path: "Budget.tla", Text: []byte(budgetSpec), Options: Options{MaxAcquisitionVisits: 1}}); err == nil || r.Completion == CompletionComplete {
		t.Fatalf("%+v %v", r, err)
	}
}
