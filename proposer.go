package paxos

/**
 * @DateTime   : 2020/11/10
 * @Author     : xumamba
 * @Description: proposer提议者模型
 **/

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type Proposer struct {
	id     int // 提议者ID
	locker sync.RWMutex

	acceptorAddr []string // acceptor决策者地址

	proposeID           float32     // 提议号
	historyMaxProposeID float32     // 历史最高提案号
	proposeValue        interface{} // 提议值
	proposeResult       interface{} // 提议结果

}

func (p *Proposer) Proposal(value interface{}) interface{} {
	p.proposeValue = value
	// 两阶段提交
	p.twoPhaseCommit()
	return p.proposeResult
}

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

// getAcceptorAddr 获得集群各节点地址
func (p *Proposer) getAcceptorAddr() []string {
	p.locker.RLock()
	addr := p.acceptorAddr
	p.locker.RUnlock()
	return addr
}

// generatePrepareMsg 生成提议请求信息
func (p *Proposer) generatePrepareMsg() PrepareMsg {
	p.locker.Lock()
	defer p.locker.Unlock()
	proposeID := generateProposeID(p.id, p.proposeID)
	p.proposeID = proposeID
	return PrepareMsg{ProposeID: proposeID}
}

// setAccepted 根据初达成的共识结果更新提议内容
func (p *Proposer) updateProposeValue(promiseMsg *PromiseMsg) {
	p.locker.Lock()
	if promiseMsg.AcceptedID > p.historyMaxProposeID {
		p.proposeValue = promiseMsg.AcceptedValue
	}
	p.locker.Unlock()
}

// generateAcceptMsg 生成提议内容请求信息
func (p *Proposer) generateAcceptMsg() AcceptMsg {
	p.locker.Lock()
	acceptMsg := AcceptMsg{
		ProposeID:    p.proposeID,
		ProposeValue: p.proposeValue,
	}
	p.locker.Unlock()
	return acceptMsg
}

// sendPrepare 发送提议请求
func (p *Proposer) sendPrepare(addr string, prepareMsg *PrepareMsg) (*PromiseMsg, error) {
	promiseMsg := new(PromiseMsg)
	err := callRPC(addr, "Acceptor.ReceivePrepare", prepareMsg, promiseMsg)
	return promiseMsg, err
}

// sendAccept 发送提议内容
func (p *Proposer) sendAccept(addr string, acceptMsg *AcceptMsg) (*AcceptedMsg, error) {
	acceptedMsg := new(AcceptedMsg)
	err := callRPC(addr, "Acceptor.ReceiveAccepted", acceptMsg, acceptedMsg)
	return acceptedMsg, err
}

// getClusterSize 获得集群投票节点数量（一般计算集群内所有节点数量的一半再加1）
func (p *Proposer) getClusterSize() int64 {
	p.locker.RLock()
	size := int64(len(p.acceptorAddr)/2 + 1)
	p.locker.RUnlock()
	return size
}
