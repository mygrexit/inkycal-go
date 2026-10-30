package problem

import "testing"

// Every kind has a name: the list of names is kept by hand next to the constants.
func TestEveryKindHasAName(t *testing.T) {
	if len(names) != int(numKinds) {
		t.Fatalf("%d names for %d kinds", len(names), numKinds)
	}
}
