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

func (dudo Dudo) InfosetLen() int {
	playerDices := dudo.dices[dudo.CurPlayer()]
	size := len(playerDices)
	size += 1 // for the separator '|'
	size += len(dudo.history)
	return size
}

func (dudo Dudo) Infoset(outInfoset []uint8) {
	cursor := 0

	playerDices := dudo.dices[dudo.CurPlayer()]
	copy(outInfoset[cursor:], playerDices)
	cursor += len(playerDices)

	outInfoset[cursor] = '|'
	cursor += 1

	copy(outInfoset[cursor:], dudo.history)
}

func (dudo Dudo) IsTerminal() bool {
	if len(dudo.history) == 0 {
		return false
	}
	lastAct := dudo.history[len(dudo.history)-1]
	return int(lastAct) == len(dudo.claims)
}

func (dudo Dudo) Payoff(outPayoff []float64) {
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
	if actual == int(claim.Num) {
		for p := 0; p < numPlayers; p++ {
			if p == claimPlayer {
				outPayoff[p] = float64(numPlayers) - 1
			} else {
				outPayoff[p] = -1
			}
		}
		return
	}

	outPayoff[claimPlayer] = float64(actual - int(claim.Num))
	outPayoff[dudoPlayer] = float64(int(claim.Num) - actual)
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

func (dudo Dudo) ActionsLen() int {
	if len(dudo.history) == 0 {
		return len(dudo.claims)
	}

	lastClaim := dudo.history[len(dudo.history)-1]
	dudoAct := len(dudo.claims)
	return dudoAct - int(lastClaim)
}

func (dudo Dudo) Actions(outActions []uint8) {
	if len(dudo.history) == 0 {
		for i := 0; i < len(outActions); i++ {
			outActions[i] = uint8(i)
		}
		return
	}

	lastClaim := int(dudo.history[len(dudo.history)-1])
	for i := 0; i < len(outActions); i++ {
		outActions[i] = uint8(i + lastClaim + 1)
	}
}

func getInfosetNode(dudo Dudo, nodeMap map[string]*Node, isBuf []uint8) *Node {
	dudo.Infoset(isBuf)
	node, ok := nodeMap[string(isBuf)]
	if !ok {
		node = NewNode(dudo.ActionsLen())
		node.InfoSet = string(isBuf)
		nodeMap[node.InfoSet] = node
	}
	return node
}

type F64Stack struct {
	buf []float64
	cur int
}

func NewF64Stack() *F64Stack {
	stk := &F64Stack{
		buf: make([]float64, 1024*1024),
	}
	return stk
}

func (stk *F64Stack) Enter() int {
	return stk.cur
}

func (stk *F64Stack) Leave(cur int) {
	stk.cur = cur
}

func (stk *F64Stack) Grow(size int) []float64 {
	cur := stk.cur
	stk.cur += size
	if stk.cur > len(stk.buf) {
		newBuf := make([]float64, stk.cur*2)
		copy(newBuf, stk.buf)
		stk.buf = newBuf
	}

	res := stk.buf[cur:stk.cur]
	for i := range res {
		res[i] = 0
	}
	return res
}

type Uint8Stack struct {
	buf []uint8
	cur int
}

func NewUint8Stack() *Uint8Stack {
	stk := &Uint8Stack{
		buf: make([]uint8, 1024*1024),
	}
	return stk
}

func (stk *Uint8Stack) Enter() int {
	return stk.cur
}

func (stk *Uint8Stack) Leave(cur int) {
	stk.cur = cur
}

func (stk *Uint8Stack) Grow(size int) []uint8 {
	cur := stk.cur
	stk.cur += size
	if stk.cur > len(stk.buf) {
		newBuf := make([]uint8, stk.cur*2)
		copy(newBuf, stk.buf)
		stk.buf = newBuf
	}

	res := stk.buf[cur:stk.cur]
	for i := range res {
		res[i] = 0
	}
	return res
}

type Stack struct {
	f64Stk   *F64Stack
	uint8Stk *Uint8Stack
}

func NewStack() *Stack {
	stk := &Stack{
		f64Stk:   NewF64Stack(),
		uint8Stk: NewUint8Stack(),
	}
	return stk
}

func (stk *Stack) Enter() [2]int {
	cursor := [2]int{
		stk.f64Stk.Enter(),
		stk.uint8Stk.Enter(),
	}
	return cursor
}

func (stk *Stack) Leave(cursor [2]int) {
	stk.f64Stk.Leave(cursor[0])
	stk.uint8Stk.Leave(cursor[1])
}

func (stk *Stack) GrowF64(size int) []float64 {
	return stk.f64Stk.Grow(size)
}

func (stk *Stack) GrowUint8(size int) []uint8 {
	return stk.uint8Stk.Grow(size)
}

func cfr(dudo Dudo, probs []float64, nodeMap map[string]*Node, stack *Stack) []float64 {
	numPlayers := len(dudo.dices)
	if dudo.IsTerminal() {
		cursor := stack.Enter()
		payoff := stack.GrowF64(numPlayers)
		dudo.Payoff(payoff)
		stack.Leave(cursor)
		return payoff
	}
	if dudo.IsChanceNode() {
		dudo.SampleChance()
		return cfr(dudo, probs, nodeMap, stack)
	}

	cursor := stack.Enter()

	// Create buffer for the utilities for all players.
	util := stack.GrowF64(numPlayers)
	// Get the list of allowed actions.
	actions := stack.GrowUint8(dudo.ActionsLen())
	dudo.Actions(actions)
	// Create buffer for the utility for the actions of the current player.
	actionUtil := stack.GrowF64(len(actions))
	// Get the strategy, which is the probabilities of each action.
	infosetBuf := stack.GrowUint8(dudo.InfosetLen())
	node := getInfosetNode(dudo, nodeMap, infosetBuf)
	strategy := stack.GrowF64(len(actions))
	node.GetStrategy(strategy)

	// Calculate the utilities.
	player := dudo.CurPlayer()
	for aIdx, a := range actions {
		actProb := strategy[aIdx]

		// Create the new state in the subtree.
		stDudo := dudo
		stDudo.history = append(stDudo.history, a)

		// Create the history probabilities for the subtree.
		stProbs := stack.GrowF64(len(probs))
		copy(stProbs, probs)
		stProbs[player] *= actProb

		// Calculate all players' utilities of the subtree.
		stUtil := cfr(stDudo, stProbs, nodeMap, stack)

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

	stack.Leave(cursor)
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
		glog.Fatal(http.ListenAndServe("localhost:6060", nil))
	}()

	numDices := []uint8{1, 1}
	// Create the initial subtree probabilities, which are ones.
	numPlayers := len(numDices)
	probs := make([]float64, numPlayers)
	for player := 0; player < len(probs); player++ {
		probs[player] = 1
	}

	var diceFaces uint8 = 6
	nodeMap := make(map[string]*Node)

	dudo := NewDudo(diceFaces, numDices)
	glog.Infof("Claims: %+v", dudo.claims)
	stack := NewStack()

	// Train our algorithm.
	iterations := 1000000
	utilLogger := NewAvgLogger("util", numPlayers, iterations/100)
	utilLogger.Precision = 6
	for i := 0; i < iterations; i++ {
		dudo := NewDudo(diceFaces, numDices)
		util := cfr(dudo, probs, nodeMap, stack)

		utilLogger.Add(util)
	}

	printNodeMap(nodeMap, numPlayers)
}
