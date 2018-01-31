package rps

import (
	"math/rand"
)

const (
	Rock       = 0
	Paper      = 1
	Scissors   = 2
	NumActions = 3
)

type RPS struct {
	RegretSum []float64

	strategy    []float64
	strategySum []float64
}

func NewRPS() *RPS {
	rps := &RPS{
		RegretSum:   make([]float64, NumActions),
		strategy:    make([]float64, NumActions),
		strategySum: make([]float64, NumActions),
	}
	return rps
}

func (rps *RPS) GetStrategy() []float64 {
	var normalizingSum float64 = 0
	for a := 0; a < NumActions; a++ {
		if rps.RegretSum[a] > 0 {
			rps.strategy[a] = rps.RegretSum[a]
		} else {
			rps.strategy[a] = 0
		}
		normalizingSum += rps.strategy[a]
	}
	for a := 0; a < NumActions; a++ {
		if normalizingSum > 0 {
			rps.strategy[a] /= normalizingSum
		} else {
			rps.strategy[a] = 1 / NumActions
		}
		rps.strategySum[a] += rps.strategy[a]
	}
	return rps.strategy
}

func GetAction(strategy []float64) int {
	r := rand.Float64()
	a := 0
	var cumulativeProbability float64 = 0
	for a < len(strategy)-1 {
		cumulativeProbability += strategy[a]
		if r < cumulativeProbability {
			break
		}
		a++
	}
	return a
}

func newPayoffMatrix() [][]float64 {
	m := [][]float64{
		[]float64{0, 1, -1},
		[]float64{-1, 0, 1},
		[]float64{1, -1, 0},
	}
	return m
}

var Payoff [][]float64 = newPayoffMatrix()

func Train(rps *RPS, oppStrategy []float64, iterations int) {
	for i := 0; i < iterations; i++ {
		// Get regret-matched mixed-strategy actions
		strategy := rps.GetStrategy()
		myAction := GetAction(strategy)
		otherAction := GetAction(oppStrategy)

		// Compute action utilities
		actionUtility := Payoff[otherAction]

		// Accumulate action regrets
		for a := 0; a < NumActions; a++ {
			rps.RegretSum[a] += actionUtility[a] - actionUtility[myAction]
		}
	}
}

func (rps *RPS) GetAverageStrategy() []float64 {
	avgStrategy := make([]float64, NumActions)
	var normalizingSum float64 = 0
	for a := 0; a < NumActions; a++ {
		normalizingSum += rps.strategySum[a]
	}

	for a := 0; a < NumActions; a++ {
		if normalizingSum > 0 {
			avgStrategy[a] = rps.strategySum[a] / normalizingSum
		} else {
			avgStrategy[a] = 1 / NumActions
		}
	}
	return avgStrategy
}
