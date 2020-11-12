package paxos
/**
 * @DateTime   : 2020/11/12
 * @Author     : xumamba
 * @Description:
 **/

import (
	"fmt"
	"sync"
	"testing"
)

func initializeCluster(size int) ([]*Learner, []*Acceptor, []*Proposer) {
	learnerPort, acceptorPort := 8000, 9000

	learners := make([]*Learner, 0, size)
	learnersAddr := make([]string, 0, size)
	for i := 0; i < size; i++ {
		learner := &Learner{
			localAddr: fmt.Sprintf("127.0.0.1:%d", learnerPort),
		}
		learner.Service()

		learners = append(learners, learner)
		learnersAddr = append(learnersAddr, learner.localAddr)
		learnerPort++
	}

	acceptors := make([]*Acceptor, 0, size)
	acceptorsAddr := make([]string, 0, size)
	for i := 0; i < size; i++ {
		acceptor := &Acceptor{
			localAddr:    fmt.Sprintf("127.0.0.1:%d", acceptorPort),
			learnersAddr: learnersAddr,
		}
		acceptor.Service()

		acceptors = append(acceptors, acceptor)
		acceptorsAddr = append(acceptorsAddr, acceptor.localAddr)
		acceptorPort++
	}

	proposers := make([]*Proposer, 0, size)
	for i := 1; i <= size; i++ {
		proposer := &Proposer{
			id:           i,
			acceptorAddr: acceptorsAddr,
		}

		proposers = append(proposers, proposer)
	}

	return learners, acceptors, proposers
}

func closeCluster(acceptors []*Acceptor, learners []*Learner) {
	for _, a := range acceptors {
		a.CloseService()
	}
	for _, l := range learners {
		l.CloseService()
	}
}

func TestClusterMode(t *testing.T) {
	learners, acceptors, proposers := initializeCluster(5)
	defer closeCluster(acceptors, learners)

	wg := &sync.WaitGroup{}

	for proposeValue, proposer := range proposers{
		wg.Add(1)
		go func(id int, proposer *Proposer) {
			defer wg.Done()
			proposer.Proposal(fmt.Sprintf("候选人:%d", id))
			return
		}(proposeValue, proposer)
	}
	wg.Wait()

	var chosenResult interface{}
	for _, l := range learners{
		if chosenResult == nil{
			chosenResult = l.chosenValue
		}else if l.chosenValue != chosenResult{
			t.Fatal()
		}
	}
	t.Logf("提案最终结果：%+v", chosenResult)
}