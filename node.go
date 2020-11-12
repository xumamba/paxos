package paxos

/**
* @DateTime   : 2020/11/10
* @Author     : xumamba
* @Description: 在实际应用中，一个服务器节点往往同时扮演Proposer、Acceptor、Learner三个角色
**/

// IProposer 提案者，主动发起提议
type IProposer interface {
	Proposal(value interface{}) interface{} // 发起提议，得到提议结果
}

// IAcceptor 决策者，被动接收Proposer的提议消息，并将提议结果告知给Learner
type IAcceptor interface {
	ReceivePrepare(args *PrepareMsg, reply *PromiseMsg) error  // 第一阶段
	ReceiveAccepted(args *AcceptMsg, reply *AcceptedMsg) error // 第二阶段
}

// ILearner 接收者，被动接收来自Acceptor的提议结果消息
type ILearner interface {
	ReceiveChosen(args *ChosenMsg, reply *EmptyMsg) error // 接收提议结果
}

// Node 服务器节点
type Node struct {
	localAddr  string   // 本节点地址
	othersAddr []string // 其它节点地址

	proposer IProposer
	acceptor IAcceptor
	learner  ILearner
}
