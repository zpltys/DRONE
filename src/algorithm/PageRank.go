package algorithm

import (
	"graph"
	"math"
	"log"
	"Set"
)

const eps = 0.01
const alpha = 0.85

type PRPair struct {
	PRValue float64
	ID int64
}

func PageRank_PEVal(g graph.Graph, prVal map[int64]float64, accVal map[int64]float64, diffVal map[int64]float64,targetsNum map[int64]int, updatedSet Set.Set, updatedMaster Set.Set, updatedMirror Set.Set) (bool, map[int][]*PRPair, int64) {
	initVal := 1.0
	for id := range g.GetNodes() {
		prVal[id] = initVal
		accVal[id] = 0.0
	}

	var iterationNum int64 = 0

	for u := range g.GetNodes() {
		temp := prVal[u] / float64(targetsNum[u])
		iterationNum += int64(len(g.GetTargets(u)))
		for v := range g.GetTargets(u) {
			diffVal[v] += temp
			updatedSet.Add(v)
			if g.IsMirror(v) {
				updatedMirror.Add(v)
			}
			if g.IsMaster(v) {
				updatedMaster.Add(v)
			}
		}
	}

	messageMap := make(map[int][]*PRPair)
	mirrorMap := g.GetMirrors()
	for v := range updatedMirror {
		workerId := mirrorMap[v]
		if _, ok := messageMap[workerId]; !ok {
			messageMap[workerId] = make([]*PRPair, 0)
		}
		messageMap[workerId] = append(messageMap[workerId], &PRPair{ID:v,PRValue:diffVal[v]})
	}

	return true, messageMap, iterationNum
}

func PageRank_IncEval(g graph.Graph, prVal map[int64]float64, accVal map[int64]float64, diffVal map[int64]float64, targetsNum map[int64]int, updatedSet Set.Set, updatedMaster Set.Set, updatedMirror Set.Set, exchangeBuffer []*PRPair) (bool, map[int][]*PRPair, int64) {
	for _, msg := range exchangeBuffer {
		diffVal[msg.ID] = msg.PRValue
		updatedSet.Add(msg.ID)
	}

	nextUpdated := Set.NewSet()

	var iterationNum int64 = 0
	maxerr := 0.0

	for u := range updatedSet {
		accVal[u] += diffVal[u]
		delete(diffVal, u)
	}


	for u := range updatedSet {
		pr := alpha * accVal[u] + 1 - alpha
		if math.Abs(prVal[u] - pr) > eps {
			maxerr = math.Max(maxerr, math.Abs(prVal[u] - pr))
			iterationNum += int64(len(g.GetTargets(u)))
			for v := range g.GetTargets(u) {
				nextUpdated.Add(v)
				diffVal[v] += (pr - prVal[u]) / float64(targetsNum[u])
				if g.IsMirror(v) {
					updatedMirror.Add(v)
				}
				if g.IsMaster(v) {
					updatedMaster.Add(v)
				}
			}
		}
		prVal[u] = pr
	}
	log.Printf("max error:%v\n", maxerr)

	updatedSet.Clear()
	for u := range nextUpdated {
		updatedSet.Add(u)
	}
	nextUpdated.Clear()

	messageMap := make(map[int][]*PRPair)
	mirrorMap := g.GetMirrors()
	for v := range updatedMirror {
		workerId := mirrorMap[v]
		if _, ok := messageMap[workerId]; !ok {
			messageMap[workerId] = make([]*PRPair, 0)
		}
		messageMap[workerId] = append(messageMap[workerId], &PRPair{ID:v,PRValue:diffVal[v]})
	}

	return len(messageMap) != 0, messageMap, iterationNum
}