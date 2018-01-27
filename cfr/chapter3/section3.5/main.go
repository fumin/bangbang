package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"sort"
	"strings"

	"github.com/golang/glog"
)

const (
	invalidDice uint8 = 255
)

// http://mlanctot.info/files/675proj/report.pdf

// http://cs.gettysburg.edu/~tneller/games/rules/dudo.pdf
func strength(n, r, diceFaces, totalNumDices int) int {
	if r != 1 {
		return (diceFaces-1)*n + (n / 2) - (diceFaces - r) - 1
	}

	if n <= (totalNumDices / 2) {
		return (2 * diceFaces * n) - n - diceFaces
	}
	if n == (totalNumDices/2 + 1) {
		return (diceFaces-1)*totalNumDices + n - 1
	}
	panic(fmt.Sprintf("with r == 1, n %d cannot be larger than %d", n, totalNumDices/2+1))
}

type Node struct {
	InfoSet     string
	RegretSum   []float64
	StrategySum []float64
	strategy    []float64

	// prob []float64
}

func NewNode(numActions int) *Node {
	node := &Node{
		RegretSum:   make([]float64, numActions),
		StrategySum: make([]float64, numActions),
		strategy:    make([]float64, numActions),
		// prob:        make([]float64, 0),
	}
	return node
}

func (node *Node) GetStrategy(outStrategy []float64) {
	var z float64 = 0
	for _, r := range node.RegretSum {
		if r < 0 {
			continue
		}
		z += r
	}

	if z == 0 {
		numActions := len(outStrategy)
		for i := 0; i < numActions; i++ {
			outStrategy[i] = float64(1) / float64(numActions)
		}
		return
	}

	for i, r := range node.RegretSum {
		if r < 0 {
			outStrategy[i] = 0
		} else {
			outStrategy[i] = float64(r) / float64(z)
		}
	}
}

func (node *Node) AvgStrategy() []float64 {
	var z float64 = 0
	for _, s := range node.StrategySum {
		z += s
	}

	numActions := len(node.StrategySum)
	avgStrat := make([]float64, numActions)
	if z == 0 {
		for i, _ := range avgStrat {
			avgStrat[i] = 1 / float64(numActions)
		}
		return avgStrat
	}

	for i, s := range node.StrategySum {
		avgStrat[i] = s / z
	}
	return avgStrat
}

type Claim struct {
	Num  uint8
	Rank uint8
}

type Dudo struct {
	diceFaces uint8
	claims    []Claim

	history []uint8
	dices   [][]uint8
}

func NewDudo(diceFaces uint8, numDices []uint8) Dudo {
	dudo := Dudo{
		diceFaces: diceFaces,
		history:   make([]uint8, 0),
	}

	// Enumerate the claims.
	totalNumDices := 0
	for _, playerNumDices := range numDices {
		totalNumDices += int(playerNumDices)
	}
	dudo.claims = make([]Claim, 0)
	for n := 1; n <= totalNumDices; n++ {
		if n%2 == 0 {
			clm := Claim{Num: uint8(n / 2), Rank: 1}
			dudo.claims = append(dudo.claims, clm)
		}
		for r := 2; r <= int(diceFaces); r++ {
			clm := Claim{Num: uint8(n), Rank: uint8(r)}
			dudo.claims = append(dudo.claims, clm)
		}
	}
	clm := Claim{Num: uint8(totalNumDices/2 + 1), Rank: 1}
	dudo.claims = append(dudo.claims, clm)
	if len(dudo.claims) > 254 {
		panic(fmt.Sprintf("number of claims %d larger than 254", dudo.claims))
	}

	// Initialize all players' dices.
	numPlayers := len(numDices)
	dudo.dices = make([][]uint8, numPlayers)
	for p := 0; p < numPlayers; p++ {
		dudo.dices[p] = make([]uint8, numDices[p])
		for i := 0; i < len(dudo.dices[p]); i++ {
			dudo.dices[p][i] = invalidDice
		}
	}

	return dudo
}

func (dudo Dudo) CurPlayer() int {
	numPlayers := len(dudo.dices)
	player := len(dudo.history) % numPlayers
	return player
}

func (dudo Dudo) Infoset() string {
	playerDices := dudo.dices[dudo.CurPlayer()]

	diceStr := make([]uint8, 0, len(playerDices))
	for _, d := range playerDices {
		diceStr = append(diceStr, d)
	}
	infoset := append(diceStr, '|')
	infoset = append(infoset, dudo.history...)

	return string(infoset)
}

func (dudo Dudo) IsTerminal() bool {
	if len(dudo.history) == 0 {
		return false
	}
	lastAct := dudo.history[len(dudo.history)-1]
	return int(lastAct) == len(dudo.claims)
}

func (dudo Dudo) Payoff() []float64 {
	// Find the player who challenged Dudo.
	numPlayers := len(dudo.dices)
	dudoIdx := len(dudo.history) - 1
	dudoPlayer := dudoIdx % numPlayers

	// Find the player whose claim was challenged.
	claimIdx := len(dudo.history) - 2
	claimPlayer := claimIdx % numPlayers
	claimID := dudo.history[claimIdx]
	claim := dudo.claims[claimID]

	// Count the actual total number of dices that have the claimed rank.
	actual := 0
	for _, playerDices := range dudo.dices {
		for _, d := range playerDices {
			if d == 1 || d == claim.Rank {
				actual++
			}
		}
	}

	// If actual rank count is equal to claim,
	// the player who makes the claim wins, and everyone else pays her one dice.
	payoff := make([]float64, numPlayers)
	if actual == int(claim.Num) {
		for p := 0; p < numPlayers; p++ {
			if p == claimPlayer {
				payoff[p] = float64(numPlayers) - 1
			} else {
				payoff[p] = -1
			}
		}
		return payoff
	}

	payoff[claimPlayer] = float64(actual - int(claim.Num))
	payoff[dudoPlayer] = float64(int(claim.Num) - actual)
	return payoff
}

func (dudo Dudo) IsChanceNode() bool {
	player := dudo.CurPlayer()
	firstDice := 0
	return dudo.dices[player][firstDice] == invalidDice
}

func (dudo Dudo) SampleChance() {
	player := dudo.CurPlayer()
	numDices := len(dudo.dices[player])
	for i := 0; i < numDices; i++ {
		dudo.dices[player][i] = uint8(rand.Intn(int(dudo.diceFaces)) + 1)
	}
}

func (dudo Dudo) Actions() []uint8 {
	if len(dudo.history) == 0 {
		actions := make([]uint8, len(dudo.claims))
		for i := 0; i < len(actions); i++ {
			actions[i] = uint8(i)
		}
		return actions
	}

	lastClaim := dudo.history[len(dudo.history)-1]
	dudoAct := len(dudo.claims)
	actions := make([]uint8, 0, dudoAct-int(lastClaim))
	for i := int(lastClaim) + 1; i <= dudoAct; i++ {
		actions = append(actions, uint8(i))
	}
	return actions
}

func getInfosetNode(dudo Dudo, nodeMap map[string]*Node) *Node {
	infoset := dudo.Infoset()
	node, ok := nodeMap[infoset]
	if !ok {
		numActions := len(dudo.Actions())
		node = NewNode(numActions)
		node.InfoSet = infoset
		nodeMap[infoset] = node
	}
	return node
}

type Stack struct {
	f64      []float64
	f64Cur   int
	f64Enter int
}

func NewStack() *Stack {
	stk := &Stack{
		f64: make([]float64, 0, 1024*1024),
	}
	return stk
}

func (stk *Stack) Enter() {
	stk.f64Enter = stk.f64Cur
}

func (stk *Stack) Leave() {
	stk.f64Cur = stk.f64Enter
}

func (stk *Stack) GrowF64(size int) []float64 {
	cur := stk.f64Cur
	stk.f64Cur += size
	return stk.f64[cur:size]
}

func cfr(dudo Dudo, probs []float64, nodeMap map[string]*Node) []float64 {
	if dudo.IsTerminal() {
		return dudo.Payoff()
	}
	if dudo.IsChanceNode() {
		dudo.SampleChance()
		return cfr(dudo, probs, nodeMap)
	}

	// Calculate the utilities for all players.
	numPlayers := len(dudo.dices)
	util := make([]float64, numPlayers)
	// Calculate the utility for the actions of the current player.
	actions := dudo.Actions()
	actionUtil := make([]float64, len(actions))

	player := dudo.CurPlayer()
	node := getInfosetNode(dudo, nodeMap)
	strategy := make([]float64, len(actions))
	node.GetStrategy(strategy)
	for aIdx, a := range actions {
		actProb := strategy[aIdx]

		// Create the new state in the subtree.
		stDudo := dudo
		stDudo.history = append(stDudo.history, a)

		// Create the history probabilities for the subtree.
		stProbs := make([]float64, len(probs))
		copy(stProbs, probs)
		stProbs[player] *= actProb

		// Calculate all players' utilities of the subtree.
		stUtil := cfr(stDudo, stProbs, nodeMap)

		actionUtil[aIdx] = stUtil[player]
		for p, playerUtil := range util {
			util[p] = playerUtil + actProb*stUtil[p]
		}
	}

	// Calculate the counterfactual probability.
	var probNegI float64 = 1
	for p, prb := range probs {
		if p == player {
			continue
		}
		probNegI *= prb
	}
	// Update the regrets.
	probI := probs[player]
	avgUtil := util[player]
	for aIdx, aUtil := range actionUtil {
		regret := aUtil - avgUtil
		node.RegretSum[aIdx] += probNegI * regret
		node.StrategySum[aIdx] += probI * strategy[aIdx]
	}

	// Debug
	// var nodeProb float64 = 1
	// for _, prb := range probs {
	// 	nodeProb *= prb
	// }
	// node.prob = append(node.prob, nodeProb)

	return util
}

func fmtInfoset(infoset string) string {
	splitted := strings.Split(infoset, "|")

	rawDices := []uint8(splitted[0])
	dices := make([]uint8, 0, len(rawDices))
	for _, d := range rawDices {
		dices = append(dices, d+'0')
	}

	rawHist := []uint8(splitted[1])
	history := make([]uint8, 0, len(rawHist))
	for _, h := range rawHist {
		history = append(history, h+'a')
	}

	return string(dices) + "|" + string(history)
}

func fmtFloatSlice(fs []float64, precision int) string {
	ss := make([]string, 0, len(fs))
	for _, f := range fs {
		fmtStr := fmt.Sprintf("%%.%df", precision)
		ss = append(ss, fmt.Sprintf(fmtStr, f))
	}
	return fmt.Sprintf("[%s]", strings.Join(ss, " "))
}

func printNodeMap(nodeMap map[string]*Node, numPlayers int) {
	// Create the infosets for each player.
	playerInfoset := make([][]string, numPlayers)
	for p := 0; p < numPlayers; p++ {
		playerInfoset[p] = make([]string, 0)
	}
	for infoset, _ := range nodeMap {
		splitted := strings.Split(infoset, "|")
		history := []uint8(splitted[1])
		player := len(history) % numPlayers

		playerInfoset[player] = append(playerInfoset[player], infoset)
	}
	for _, pis := range playerInfoset {
		sort.Strings(pis)
	}

	for player, infosets := range playerInfoset {
		fmt.Printf("Player %d infosets:\n", player)
		for _, is := range infosets {
			n := nodeMap[is]
			avgStrat := n.AvgStrategy()

			// var avgProb float64 = 0
			// for _, prb := range n.prob {
			// 	avgProb += prb
			// }
			// avgProb /= float64(len(n.prob))

			fmt.Printf("%6s: %s\n", fmtInfoset(n.InfoSet), fmtFloatSlice(avgStrat, 2))
			// fmt.Printf("%6s: %f, strat: %s\n", fmtInfoset(n.InfoSet), avgProb, fmtFloatSlice(avgStrat))
		}
		fmt.Printf("\n")
	}
}

type AvgLogger struct {
	Precision int

	sum       []float64
	logEvery  int
	iteration int
	prefix    string
}

func NewAvgLogger(prefix string, length, logEvery int) *AvgLogger {
	al := &AvgLogger{
		Precision: 2,
		sum:       make([]float64, length),
		logEvery:  logEvery,
		prefix:    prefix,
	}
	return al
}

func (al *AvgLogger) Add(fs []float64) {
	for i, f := range fs {
		al.sum[i] += f
	}
	al.iteration++

	if (al.iteration % al.logEvery) == 0 {
		avg := make([]float64, len(al.sum))
		for i, s := range al.sum {
			avg[i] = s / float64(al.iteration)
		}
		avgStr := fmtFloatSlice(avg, al.Precision)
		glog.Infof("%s %d: %s", al.prefix, al.iteration, avgStr)
	}
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	go func() {
		glog.Fatal(http.ListenAndServe("localhost:6061", nil))
	}()

	numDices := []uint8{1, 1}
	// Create the initial subtree probabilities, which are ones.
	numPlayers := len(numDices)
	probs := make([]float64, numPlayers)
	for player := 0; player < len(probs); player++ {
		probs[player] = 1
	}

	var diceFaces uint8 = 3
	nodeMap := make(map[string]*Node)

	dudo := NewDudo(diceFaces, numDices)
	glog.Infof("Claims: %+v", dudo.claims)

	// Train our algorithm.
	iterations := 1000000
	utilLogger := NewAvgLogger("util", numPlayers, iterations/100)
	utilLogger.Precision = 6
	for i := 0; i < iterations; i++ {
		dudo := NewDudo(diceFaces, numDices)
		util := cfr(dudo, probs, nodeMap)

		utilLogger.Add(util)
	}

	printNodeMap(nodeMap, numPlayers)
}
