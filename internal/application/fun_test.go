package application

import (
	"math/rand"
	"testing"
)

func TestFunRoll(t *testing.T) {
	svc := NewFunService()
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 1000; i++ {
		a, b := svc.Roll(rng)
		if a < 1 || a > 100 || b < 1 || b > 100 {
			t.Fatalf("roll out of range: %d, %d", a, b)
		}
	}
}

func TestFunPick(t *testing.T) {
	svc := NewFunService()
	rng := rand.New(rand.NewSource(42))

	if _, err := svc.Pick(rng, nil); err == nil {
		t.Fatalf("want error for no options")
	}
	if _, err := svc.Pick(rng, []string{"only"}); err == nil {
		t.Fatalf("want error for a single option")
	}

	options := []string{"a", "b", "c"}
	for i := 0; i < 100; i++ {
		got, err := svc.Pick(rng, options)
		if err != nil {
			t.Fatalf("Pick: %v", err)
		}
		found := false
		for _, o := range options {
			if got == o {
				found = true
			}
		}
		if !found {
			t.Fatalf("pick returned unknown option %q", got)
		}
	}
}
