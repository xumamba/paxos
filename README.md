# paxos
Implementation of Paxos algorithm in Go

# Paxos Overview
&emsp;&emsp;&emsp;**Paxos算法**是[莱斯利·兰伯特](https://zh.wikipedia.org/wiki/%E8%8E%B1%E6%96%AF%E5%88%A9%C2%B7%E5%85%B0%E6%B3%A2%E7%89%B9)
 于1990年提出的一种基于消息传递、具有高度容错特性且拥有完整数学证明的共识算法。
 
&emsp;&emsp;&emsp;当今ZooKeeper使用的Zab、ETCD中使用的Raft以及Chubby和Boxwood均是基于Paxos算法设计的，正如Google在的Chubby中的描述：
"all working protocols for asynchronous consensus we have so far encountered have Paxos at their core."

# 算法解决的问题
&emsp;&emsp;&emsp;paxos算法要解决的问题是分布式一致性的问题，即在一个分布式系统中，各个节点（进程）如何对某一个或者多个值达成一个统一的共识。

应用场景：
- 集群选主
- 资源互斥访问
- 一致性问题
- 其它

# 算法实现

## 角色划分

- Proposer: 提案者，主动发起提议。
- Acceptor: 决策者，被动接收Proposer的提议消息，并将提议结果告知给Learner
- Learner:  接收者，被动接收来自Acceptor的提议结果消息。

```go
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
```

在实际应用中，一个服务器节点往往同时扮演Proposer、Acceptor、Learner三个角色。

```go
// Node 服务器节点
type Node struct {
	localAddr  string   // 本节点地址
	othersAddr []string // 其它节点地址

	proposer IProposer
	acceptor IAcceptor
	learner  ILearner
}
```

## 协议两阶段

Paxos算法达成一次共识的过程主要分为以下两个阶段：

1. Prepare阶段：
    - Proposer生成自己**唯一且递增**的ProposeID向所有Acceptor发送提议申请;
    - Acceptor接收各个Proposer发来的ProposeID,选择**最大**的ProposeID使其Proposer通过提议申请，同时判断是否告知其已有的提议内容；
    - Proposer接收到**半数以上**Acceptor的提议申请通过的回复之后才开始准备发起第二阶段请求。

2. Accept阶段：
    - 提议申请通过的Proposer生成提议内容并向所有Acceptor发送提议内容请求；
    - Acceptor接收到Proposer提议内容，先校验请求方是否为有资格的提议者，接收合法的提议值，持久化提议结果（以便后续集群动态伸缩），并将提议结果告知所有Learner。
    - Proposer接收到**半数以上**Acceptor的提议内容申请通过的回复之后，结束此次共识过程。

```go
func (p *Proposer) twoPhaseCommit() {
	acceptorAddr := p.getAcceptorAddr()
	for p.proposeResult == nil {
		// 第一阶段：向所有决策者发送提议请求
		prepareMsg := p.generatePrepareMsg()
		var prepareSuccessCount, prepareFailedCount int64 // 节点投票成功、失败 的数量
		prepareSuccessChan := make(chan struct{})         // 第一阶段成功的信道
		prepareFailedChan := make(chan struct{})          // 第一阶段失败的信道

		for _, acceptor := range acceptorAddr {
			go func(addr string, prepareMsg PrepareMsg) {
				defer func() {
					if atomic.LoadInt64(&prepareSuccessCount) >= p.getClusterSize() {
						prepareSuccessChan <- struct{}{}
						return
					}
					if atomic.LoadInt64(&prepareFailedCount) >= p.getClusterSize() {
						prepareFailedChan <- struct{}{}
						return
					}
				}()

				promiseMsg, err := p.sendPrepare(addr, &prepareMsg)
				// 此处添加容错机制 构建容错模型。（消息丢失、重复、延迟、网络分区等处理）
				if err != nil || promiseMsg.ProposeID != prepareMsg.ProposeID || addr != promiseMsg.AcceptorAddr {
					atomic.AddInt64(&prepareSuccessCount, 1)
				}
				if promiseMsg.StatusCode == successCode {
					atomic.AddInt64(&prepareSuccessCount, 1)
				} else {
					atomic.AddInt64(&prepareFailedCount, 1)
				}
				// 已存在其它提议内容
				if promiseMsg.AcceptedID > 0 {
					p.updateProposeValue(promiseMsg)
				}
			}(acceptor, prepareMsg)
		}

		select {
		case <-time.After(200 * time.Millisecond):
			continue
		case <-prepareFailedChan:
			continue
		case <-prepareSuccessChan:
			// 可以执行第二阶段，向所有acceptor发送提议内容了。
			log.Printf("Proposer[%d]:提议请求成功：%+v", p.id, p)
		}

		// 第二阶段：向所有决策者发送提议内容
		var acceptSuccessCount, acceptFailedCount int64
		acceptSuccessChan := make(chan struct{}) // 第二阶段成功的信道
		acceptFailedChan := make(chan struct{})  // 第二阶段失败的信道
		acceptMsg := p.generateAcceptMsg()
		for _, acceptor := range acceptorAddr {
			go func(addr string, acceptMsg AcceptMsg) {
				defer func() {
					if atomic.LoadInt64(&acceptSuccessCount) >= p.getClusterSize() {
						acceptSuccessChan <- struct{}{}
						return
					}
					if atomic.LoadInt64(&acceptFailedCount) >= p.getClusterSize() {
						acceptFailedChan <- struct{}{}
						return
					}
				}()

				acceptedMsg, err := p.sendAccept(addr, &acceptMsg)
				if err != nil || acceptedMsg.ProposeID != acceptMsg.ProposeID || addr != acceptedMsg.AcceptorAddr {
					atomic.AddInt64(&acceptFailedCount, 1)
					return
				}
				if acceptedMsg.StatusCode == successCode {
					atomic.AddInt64(&acceptSuccessCount, 1)
				} else {
					atomic.AddInt64(&acceptFailedCount, 1)
				}
			}(acceptor, acceptMsg)
		}

		select {
		case <-time.After(200 * time.Millisecond):
			continue
		case <-acceptFailedChan:
			continue
		case <-acceptSuccessChan:
			p.proposeResult = p.proposeValue
			return
		}
	}
}
```


## 容错模型

- 只要大多数Acceptor正常运行，共识最终就能达成；
- 分布式系统下消息异步发送、消息丢失、重复、延迟、网络分区等处理；
- 集群动态伸缩后，结果不丢失；
- 不能解决拜占庭错误（消息被篡改）
```go
// 只要大多数节点正常运行即可
defer func() {
	if atomic.LoadInt64(&prepareSuccessCount) >= p.getClusterSize() {
		prepareSuccessChan <- struct{}{}
			return
	}
	if atomic.LoadInt64(&prepareFailedCount) >= p.getClusterSize() {
		prepareFailedChan <- struct{}{}
			return
    }
}()
... 
...
// 消息丢失、重复、延迟、网络分区等处理
if err != nil || promiseMsg.ProposeID != prepareMsg.ProposeID || addr != promiseMsg.AcceptorAddr {
		atomic.AddInt64(&prepareSuccessCount, 1)
}
...
...
// 容错模型，避免Proposer申请通过第一阶段后挂掉的情况。
if args.ProposeID >= a.getMaxProposeID() {
	a.acceptedID = args.ProposeID
	a.acceptedValue = args.ProposeValue
	reply.StatusCode = successCode
	chosenResult := &ChosenMsg{ChosenValue: a.acceptedValue}
	// 此处持久化提议结果，以便后续集群动态伸缩。
	for _, learnerAddr := range a.getLearnersAddr() {
		err := callRPC(learnerAddr, "Learner.ReceiveChosen", chosenResult, &EmptyMsg{})
		if err != nil {
			// error handle
		}
	}
}
...
```

# 效率与活锁问题

效率问题：
- 一次共识过程至少需要两次网络请求；
- 需要达成多个值时就需要执行多次Paxos算法。

活锁问题：
- 两个Proposer交替执行第一阶段Prepare成功，但第二阶段Accept失败。

# Multi Paxos

>Multi Paxos通常是指一类共识算法，不是精确的指定某个共识算法，它基于Basic Paxos，进而改进了上述效率与活锁问题。

- 先从所有Proposer中选择一个Leader，由唯一的Leader发送提议请求（解决活锁问题）;
- 集群只有一个Leader(Proposer)发起共识提议，所以可以省略第一阶段过程（提高效率）;
- Leader宕机的情况只需要重新选举即可；
- 如果集群出现脑裂也是安全的，可视为Basic Paxos;
- 需要达成多个值时，只执行一次Prepare阶段选出Leader，再由此Leader连续发出对应值的Accept请求(应为每次Accept绑定唯一递增的Instance ID)即可。

