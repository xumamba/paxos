package paxos

import (
	"log"
	"net"
	"net/rpc"
)

/**
* @DateTime   : 2020/11/11
* @Author     : xumamba
* @Description: Learner 只能被动接受提案结果
**/

type Learner struct {
	chosenValue interface{}  // 被比准的提案

	localAddr string
	listener net.Listener
}

func (l *Learner) ReceiveChosen(args *ChosenMsg, reply *EmptyMsg) error {
	log.Printf("Learner[%s]:收到提案结果:%v", l.localAddr, args.ChosenValue)
	l.chosenValue = args.ChosenValue
	return nil
}


func (l *Learner) Service() {
	server := rpc.NewServer()
	err := server.Register(l)
	errHandle(err)
	l.listener, err = net.Listen("tcp", l.localAddr)
	errHandle(err)
	go func() {
		for {
			conn, err := l.listener.Accept()
			if err != nil {
				continue
			}
			go server.ServeConn(conn)
		}
	}()
}

func (l *Learner) CloseService() {
	err := l.listener.Close()
	errHandle(err)
}