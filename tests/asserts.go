package tests

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func AssertStringSliceEqual(t *testing.T, exp, act []string) {
	t.Helper()

	sort.Strings(exp)
	sort.Strings(act)

	if !cmp.Equal(exp, act) {
		t.Error(cmp.Diff(exp, act))
	}
}
