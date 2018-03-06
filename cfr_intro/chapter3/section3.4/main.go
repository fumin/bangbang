// https://www.aaai.org/Papers/AAAI/2005/AAAI05-123.pdf
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"

	"github.com/fumin/bangbang/cfr/chapter3"
	"github.com/golang/glog"
)

const (
	Pass       = 0
	Bet        = 1
	NumActions = 2
)

var (
// cardsGetter = NewCardsGetter()
)

type CardsGetter struct {
	scanner *bufio.Scanner
}

func NewCardsGetter() *CardsGetter {
	f, err := os.Open("rand.txt")
	if err != nil {
		glog.Fatalf("%+v", err)
	}
	cg := &CardsGetter{
		scanner: bufio.NewScanner(f),
	}
	return cg
}

func (cg *CardsGetter) get() []int {
	cg.scanner.Scan()
	line := cg.scanner.Text()
	cards := []int{1, 2, 3}
	cards[0], _ = strconv.Atoi(line[0:1])
	cards[1], _ = strconv.Atoi(line[1:2])
	cards[2], _ = strconv.Atoi(line[2:3])
	return cards
}

type Agent struct {
	nodeMap map[string]*chapter3.Node
}

func NewAgent() *Agent {
	agent := &Agent{
		nodeMap: make(map[string]*chapter3.Node),
	}
	return agent
}

func train(agent *Agent, iterations int) {
	cards := []int{1, 2, 3}
	var util float64 = 0
	for i := 0; i < iterations; i++ {
		// Shuffle cards
		for c1 := len(cards) - 1; c1 > 0; c1-- {
			c2 := rand.Intn(c1 + 1)
			cards[c1], cards[c2] = cards[c2], cards[c1]
		}
		// cards = cardsGetter.get()

		util += cfr(agent, cards, "", 1, 1)
	}

	glog.Infof("Average game value %f", util/float64(iterations))

	// Sort infoSets and print them
	infoSets := make([]string, 0, len(agent.nodeMap))
	for is, _ := range agent.nodeMap {
		infoSets = append(infoSets, is)
	}
	sort.Strings(infoSets)
	for _, is := range infoSets {
		n := agent.nodeMap[is]
		glog.Infof("%4s: %+v", n.InfoSet, n.AvgStrategy())
	}
}

func terminalPayoff(cards []int, history string, plays, player, opponent int) (float64, bool) {
	if plays > 1 {
		terminalPass := (history[plays-1] == 'p')
		doubleBet := history[plays-2:plays] == "bb"
		isPlayerCardHigher := cards[player] > cards[opponent]
		if terminalPass {
			if history == "pp" {
				if isPlayerCardHigher {
					return 1, true
				} else {
					return -1, true
				}
			} else {
				return 1, true
			}
		} else if doubleBet {
			if isPlayerCardHigher {
				return 2, true
			} else {
				return -2, true
			}
		}
	}
	return -1, false
}

func cfr(agent *Agent, cards []int, history string, p0, p1 float64) float64 {
	plays := len(history)
	player := plays % 2
	opponent := 1 - player

	// Return payoff for terminal states.
	payoff, ok := terminalPayoff(cards, history, plays, player, opponent)
	if ok {
		return payoff
	}

	infoSet := fmt.Sprintf("%d%s", cards[player], history)
	// Get information set node or create it if nonexistant.
	node, ok := agent.nodeMap[infoSet]
	if !ok {
		node = chapter3.NewNode(NumActions)
		node.InfoSet = infoSet
		agent.nodeMap[infoSet] = node
	}

	// For each action, recursively call cfr with additional history and probability.
	strategy := node.GetStrategy()
	var realizationWeight float64 = -1
	if player == 0 {
		realizationWeight = p0
	} else {
		realizationWeight = p1
	}
	node.AccStrategy(strategy, realizationWeight)
	util := make([]float64, NumActions)
	var nodeUtil float64
	for a := 0; a < NumActions; a++ {
		actStr := ""
		if a == 0 {
			actStr = "p"
		} else {
			actStr = "b"
		}
		nextHistory := fmt.Sprintf("%s%s", history, actStr)

		if player == 0 {
			util[a] = -cfr(agent, cards, nextHistory, p0*strategy[a], p1)
		} else {
			util[a] = -cfr(agent, cards, nextHistory, p0, p1*strategy[a])
		}
		nodeUtil += strategy[a] * util[a]
	}

	// For each action, compute and accumulate counterfactual regret.
	for a, aUtil := range util {
		regret := aUtil - nodeUtil
		if player == 0 {
			node.RegretSum[a] += p1 * regret
		} else {
			node.RegretSum[a] += p0 * regret
		}
	}

	return nodeUtil
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	agent := NewAgent()
	iterations := 1000000
	train(agent, iterations)
}
