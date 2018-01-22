package main

import (
	"flag"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/fumin/bangbang/cfr/rps"
	"github.com/golang/glog"
)

func combination(n, k int) int {
	up := 1
	for i := n - k + 1; i <= n; i++ {
		up *= i
	}

	down := 1
	for i := 0; i <= k; i++ {
		down *= 1
	}

	return up / down
}

func multiset(n, k int) int {
	return combination(n+k-1, k)
}

func comb(n, k int, emit func([]int)) {
	s := make([]int, k)
	last := k - 1
	var rc func(int, int)
	rc = func(i, next int) {
		for j := next; j < n; j++ {
			s[i] = j
			if i == last {
				// s is sorted.
				emit(s)
			} else {
				rc(i+1, j+1)
			}
		}
		return
	}
	rc(0, 0)
}

func hasZero(ints []int) bool {
	for _, a := range ints {
		if a == 0 {
			return true
		}
	}
	return false
}

type ColonelBlotto struct {
	S int
	N int

	Actions [][]int
}

func NewColonelBlotto(s, n int, allowZero bool) *ColonelBlotto {
	cb := &ColonelBlotto{
		S: s,
		N: n,
	}

	comb(s+n-1, n-1, func(bars []int) {
		act := make([]int, n)

		act[0] = bars[0]
		for i, bi := range bars[1:] {
			act[i+1] = bi - bars[i] - 1
		}
		act[n-1] = (s + n - 1) - bars[n-2] - 1

		if !allowZero && hasZero(act) {
			return
		}

		cb.Actions = append(cb.Actions, act)
	})

	return cb
}

func (cb *ColonelBlotto) Payoff(actIdA, actIdB int) float64 {
	actA := cb.Actions[actIdA]
	actB := cb.Actions[actIdB]
	return float64(multisetPayoff(actA, actB))
}

func (cb *ColonelBlotto) NumActions() int {
	return len(cb.Actions)
}

type ColonelBlottoPartition struct {
	S int
	N int

	Actions [][]int
	payoff  [][]float64
}

func NewColonelBlottoPartition(s, n int, allowZero bool) *ColonelBlottoPartition {
	cb := &ColonelBlottoPartition{
		S: s,
		N: n,
	}

	// List the multiset actions
	msActions := make([][]int, 0)
	comb(s+n-1, n-1, func(bars []int) {
		act := make([]int, n)

		act[0] = bars[0]
		for i, bi := range bars[1:] {
			act[i+1] = bi - bars[i] - 1
		}
		act[n-1] = (s + n - 1) - bars[n-2] - 1

		if !allowZero && hasZero(act) {
			return
		}

		msActions = append(msActions, act)
	})

	actMap := make(map[string]struct{})
	encode := func(ints []int) string {
		buf := make([]int, len(ints))
		copy(buf, ints)

		sort.Ints(buf)
		ss := make([]string, 0, len(buf))
		for _, a := range buf {
			ss = append(ss, fmt.Sprintf("%d", a))
		}
		return strings.Join(ss, ",")
	}

	for _, a := range msActions {
		actStr := encode(a)
		if _, ok := actMap[actStr]; ok {
			continue
		}
		actMap[actStr] = struct{}{}
		cb.Actions = append(cb.Actions, a)
	}

	cb.payoff = make([][]float64, 0)
	for i, myAct := range cb.Actions {
		cb.payoff = append(cb.payoff, make([]float64, len(cb.Actions)))
		myStr := encode(myAct)
		for j, oppAct := range cb.Actions {
			oppStr := encode(oppAct)

			numCases := 0
			casesPayoff := 0
			for _, msMy := range msActions {
				msMyStr := encode(msMy)
				if msMyStr != myStr {
					continue
				}
				for _, msOpp := range msActions {
					msOppStr := encode(msOpp)
					if msOppStr != oppStr {
						continue
					}

					numCases++
					casesPayoff += multisetPayoff(msMy, msOpp)
				}
			}

			cb.payoff[i][j] = float64(casesPayoff) / float64(numCases)
		}
	}

	return cb
}

func multisetPayoff(actA, actB []int) int {
	aWin := 0
	bWin := 0
	for i, a := range actA {
		b := actB[i]
		if a > b {
			aWin++
		} else if a < b {
			bWin++
		}
	}

	if aWin > bWin {
		return 1
	} else if bWin > aWin {
		return -1
	}
	return 0
}

func (cb *ColonelBlottoPartition) Payoff(actIdA, actIdB int) float64 {
	return cb.payoff[actIdA][actIdB]
}

func (cb *ColonelBlottoPartition) NumActions() int {
	return len(cb.Actions)
}

type Game interface {
	NumActions() int
	Payoff(int, int) float64
}

type Player struct {
	regret      []float64
	strategySum []float64

	game     Game
	strategy []float64
}

func NewPlayer(game Game) *Player {
	p := &Player{
		regret: make([]float64, game.NumActions()),

		game:        game,
		strategy:    make([]float64, game.NumActions()),
		strategySum: make([]float64, game.NumActions()),
	}
	return p
}

func (p *Player) GetStrategy() []float64 {
	var z float64 = 0
	for _, r := range p.regret {
		if r < 0 {
			continue
		}
		z += r
	}

	if z == 0 {
		for i := 0; i < p.game.NumActions(); i++ {
			p.strategy[i] = float64(1) / float64(p.game.NumActions())
		}
		return p.strategy
	}

	for i, r := range p.regret {
		if r < 0 {
			p.strategy[i] = 0
		} else {
			p.strategy[i] = float64(r) / float64(z)
		}
	}
	return p.strategy
}

func (p *Player) AccRegret(myAct, oppAct int) {
	for a := 0; a < p.game.NumActions(); a++ {
		p.regret[a] += p.game.Payoff(a, oppAct) - p.game.Payoff(myAct, oppAct)
	}
}

func (p *Player) AccStrategy(strategy []float64) {
	for a := 0; a < p.game.NumActions(); a++ {
		p.strategySum[a] += strategy[a]
	}
}

func (p *Player) AvgStrategy() []float64 {
	var z float64 = 0
	for _, s := range p.strategySum {
		z += s
	}

	avgStrat := make([]float64, p.game.NumActions())
	if z == 0 {
		for i, _ := range p.strategySum {
			avgStrat[i] = 1 / float64(p.game.NumActions())
		}
		return avgStrat
	}

	for i, s := range p.strategySum {
		avgStrat[i] = s / z
	}
	return avgStrat
}

func train(playerA, playerB *Player) {
	strategyA := playerA.GetStrategy()
	actionA := rps.GetAction(strategyA)
	strategyB := playerB.GetStrategy()
	actionB := rps.GetAction(strategyB)

	playerA.AccRegret(actionA, actionB)
	playerB.AccRegret(actionB, actionA)

	playerA.AccStrategy(strategyA)
	playerB.AccStrategy(strategyB)
}

func play(game Game) ([][]float64, [][]float64) {
	playerA := NewPlayer(game)
	playerB := NewPlayer(game)
	for a := 0; a < game.NumActions(); a++ {
		playerA.regret[a] = rand.Float64()
		playerB.regret[a] = rand.Float64()
	}

	initStrat := make([][]float64, 2)
	initStrat[0] = make([]float64, game.NumActions())
	copy(initStrat[0], playerA.GetStrategy())
	initStrat[1] = make([]float64, game.NumActions())
	copy(initStrat[1], playerB.GetStrategy())

	for i := 0; i < 1000000; i++ {
		train(playerA, playerB)
	}

	finalStrat := make([][]float64, 2)
	finalStrat[0] = playerA.AvgStrategy()
	finalStrat[1] = playerB.AvgStrategy()
	return initStrat, finalStrat
}

func fmtFloatSlice(fs []float64) string {
	ss := make([]string, 0, len(fs))
	for _, f := range fs {
		ss = append(ss, fmt.Sprintf("%.2f", f))
	}
	return strings.Join(ss, ", ")
}

func fmtPayoff(game Game) string {
	lines := make([]string, 0, game.NumActions())
	for i := 0; i < game.NumActions(); i++ {
		ss := make([]string, 0, game.NumActions())
		for j := 0; j < game.NumActions(); j++ {
			ss = append(ss, fmt.Sprintf("%.2f", game.Payoff(i, j)))
		}
		lines = append(lines, strings.Join(ss, ", "))
	}
	return strings.Join(lines, "\n")
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	s := 5
	n := 3
	allowZero := true
	// cb := NewColonelBlotto(s, n, allowZero)
	cb := NewColonelBlottoPartition(s, n, allowZero)
	glog.Infof("Actions: %+v", cb.Actions)
	glog.Infof("Payoff: \n%+v", fmtPayoff(cb))

	for i := 0; i < 10; i++ {
		initStrat, finalStrat := play(cb)
		glog.Infof("-------")
		glog.Infof("game %d", i)
		glog.Infof("init strategy A: %+v", fmtFloatSlice(initStrat[0]))
		glog.Infof("init strategy B: %+v", fmtFloatSlice(initStrat[1]))
		glog.Infof("final strategy A: %+v", fmtFloatSlice(finalStrat[0]))
		glog.Infof("final strategy B: %+v", fmtFloatSlice(finalStrat[1]))
	}
}
