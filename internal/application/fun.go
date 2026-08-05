package application

import (
	"fmt"
	"math/rand"
)

// FunService holds the stateless entertainment commands (dice, coin-flip
// style pickers). It depends on nothing but randomness.
type FunService struct{}

// NewFunService builds the service.
func NewFunService() *FunService { return &FunService{} }

// Roll rolls two d100 dice (e.g. player vs. bot), each in [1, 100].
func (FunService) Roll(rng *rand.Rand) (int, int) {
	return rng.Intn(100) + 1, rng.Intn(100) + 1
}

// Pick chooses one option uniformly at random. Fewer than two options makes
// no sense to pick between, so it is rejected.
func (FunService) Pick(rng *rand.Rand, options []string) (string, error) {
	if len(options) < 2 {
		return "", fmt.Errorf("pick needs at least 2 options, got %d", len(options))
	}
	return options[rng.Intn(len(options))], nil
}
