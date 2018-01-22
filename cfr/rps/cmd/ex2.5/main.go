package main

import (
	"flag"
	"math/rand"

	"github.com/fumin/bangbang/cfr/rps"
	"github.com/golang/glog"
)

func train(playerA *rps.RPS, playerB *rps.RPS, iterations int) {
	for i := 0; i < iterations; i++ {
		// Get regret-matched mixed-strategy actions
		strategyA := playerA.GetStrategy()
		actionA := rps.GetAction(strategyA)
		strategyB := playerB.GetStrategy()
		actionB := rps.GetAction(strategyB)

		// Compute action utilities
		utilityA := rps.Payoff[actionB]
		utilityB := rps.Payoff[actionA]

		// Accumulate action regrets
		for a := 0; a < rps.NumActions; a++ {
			playerA.RegretSum[a] += utilityA[a] - utilityA[actionA]
			playerB.RegretSum[a] += utilityB[a] - utilityB[actionB]
		}
	}
}

func play() ([][]float64, [][]float64) {
	playerA := rps.NewRPS()
	playerB := rps.NewRPS()
	for a := 0; a < rps.NumActions; a++ {
		playerA.RegretSum[a] = rand.Float64()
		playerB.RegretSum[a] = rand.Float64()
	}

	// Calculate the strategy sum as a side effect.
	playerA.GetStrategy()
	playerB.GetStrategy()
	// Get the average strategy from the strategy sum.
	initStrat := make([][]float64, 0, 2)
	initStrat = append(initStrat, playerA.GetAverageStrategy())
	initStrat = append(initStrat, playerB.GetAverageStrategy())

	train(playerA, playerB, 1000000)

	finalStrat := make([][]float64, 0, 2)
	finalStrat = append(finalStrat, playerA.GetAverageStrategy())
	finalStrat = append(finalStrat, playerB.GetAverageStrategy())
	return initStrat, finalStrat
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	for i := 0; i < 10; i++ {
		initStrat, finalStrat := play()
		glog.Infof("-------")
		glog.Infof("game %d", i)
		glog.Infof("init strategy A: %+v", initStrat[0])
		glog.Infof("init strategy B: %+v", initStrat[1])
		glog.Infof("final strategy A: %+v", finalStrat[0])
		glog.Infof("final strategy B: %+v", finalStrat[1])
	}
}
