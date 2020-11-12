package paxos

/**
 * @DateTime   : 2020/11/11
 * @Author     : xumamba
 * @Description: acceptor决策者模型
 **/

import (
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
)

type Acceptor struct {
	locker sync.RWMutex

	localAddr     string      // 本节点地址
	learnersAddr  []string    // Learners节点地址
	maxProposeID  float32     // 收到的最大提议标识
	acceptedID    float32     // 暂定决策的提议标识
	acceptedValue interface{} // 暂定决策的提议值

	listener net.Listener
}

// ReceivePrepare 处理第一阶段提议请求
func (a *Acceptor) ReceivePrepare(args *PrepareMsg, reply *PromiseMsg) error {
	log.Printf("Acceptor[%s]:收到第一阶段提议请求：%+v",a.getLocalAddr(), args)
	reply.ProposeID = args.ProposeID
	reply.AcceptorAddr = a.getLocalAddr()
	if args.ProposeID > a.maxProposeID {
		a.maxProposeID = args.ProposeID
		reply.StatusCode = successCode
		if a.acceptedID > 0 && a.acceptedValue != nil {
			reply.AcceptedID = a.acceptedID
			reply.AcceptedValue = a.acceptedValue
		}
	}
	return nil
}

// ReceiveAccepted 处理第二阶段提议内容
func (a *Acceptor) ReceiveAccepted(args *AcceptMsg, reply *AcceptedMsg) error {
	log.Printf("Acceptor[%s]:收到第二阶段提议请求：%+v",a.getLocalAddr(), args)
	reply.ProposeID = args.ProposeID
	reply.AcceptorAddr = a.getLocalAddr()
	if args.ProposeID == a.maxProposeID {
		a.acceptedID = args.ProposeID
		a.acceptedValue = args.ProposeValue
		reply.StatusCode = successCode
		chosenResult := &ChosenMsg{ChosenValue: a.acceptedValue}
		for _, learnerAddr := range a.getLearnersAddr() {
			err := callRPC(learnerAddr, "Learner.ReceiveChosen", chosenResult, &EmptyMsg{})
			if err != nil {
				// error handle
			}
		}
	}
	return nil
}

// getLocalAddr
func (a *Acceptor) getLocalAddr() string {
	a.locker.RLock()
	localAddr := a.localAddr
	a.locker.RUnlock()
	return localAddr
}

// getLearnersAddr 获取Learners节点地址
func (a *Acceptor) getLearnersAddr() []string {
	a.locker.RLock()
	lAddr := a.learnersAddr
	a.locker.RUnlock()
	return lAddr
}

func (a *Acceptor) Service() {
	server := rpc.NewServer()
	err := server.Register(a)
	errHandle(err)
	a.listener, err = net.Listen("tcp", a.localAddr)
	errHandle(err)
	go func() {
		for {
			conn, err := a.listener.Accept()
			if err != nil {
				continue
			}
			// 模拟不稳定网络下的数据丢失
			if isUnstableNetwork && rand.Intn(100) < 20 {
				err := conn.Close()
				errHandle(err)
				continue
			}
			go server.ServeConn(conn)
		}
	}()
}

func (a *Acceptor) CloseService() {
	err := a.listener.Close()
	errHandle(err)
}
