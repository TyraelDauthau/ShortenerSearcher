package main

import (
	"testing"
)

func TestSet(t *testing.T) {
	items := []WordSet{
		ToWordSet([]string{"A", "a"}),
		ToWordSet([]string{"B", "b"}),
		ToWordSet([]string{"C", "c"}),
		ToWordSet([]string{"D", "d"}),
		ToWordSet([]string{"E", "e"}),
	}

	results := Permutations(items)
	for _, perm := range results {
		upper := false
		lower := false

		for _, word := range perm {
			if word == "A" {
				upper = true
			}

			if word == "a" {
				lower = true
			}
		}

		if upper && lower {
			t.Errorf("Permutations contains items from the same set")
		}
	}
}
