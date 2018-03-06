package chapter3

type Node struct {
	InfoSet     string
	RegretSum   []float64
	strategy    []float64
	strategySum []float64
}

func NewNode(numActions int) *Node {
	node := &Node{
		RegretSum:   make([]float64, numActions),
		strategy:    make([]float64, numActions),
		strategySum: make([]float64, numActions),
	}
	return node
}

func (node *Node) GetStrategy() []float64 {
	var z float64 = 0
	for _, r := range node.RegretSum {
		if r < 0 {
			continue
		}
		z += r
	}

	if z == 0 {
		numActions := len(node.strategy)
		for i := 0; i < numActions; i++ {
			node.strategy[i] = float64(1) / float64(numActions)
		}
		return node.strategy
	}

	for i, r := range node.RegretSum {
		if r < 0 {
			node.strategy[i] = 0
		} else {
			node.strategy[i] = float64(r) / float64(z)
		}
	}
	return node.strategy
}

func (node *Node) AccStrategy(strategy []float64, realizationWeight float64) {
	for i, s := range strategy {
		node.strategySum[i] += realizationWeight * s
	}
}

func (node *Node) AvgStrategy() []float64 {
	var z float64 = 0
	for _, s := range node.strategySum {
		z += s
	}

	numActions := len(node.strategySum)
	avgStrat := make([]float64, numActions)
	if z == 0 {
		for i, _ := range node.strategySum {
			avgStrat[i] = 1 / float64(numActions)
		}
		return avgStrat
	}

	for i, s := range node.strategySum {
		avgStrat[i] = s / z
	}
	return avgStrat
}
