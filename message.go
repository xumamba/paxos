package paxos

/**
 * @DateTime   : 2020/11/10
 * @Author     : xumamba
 * @Description: 两阶段提交，RPC消息内容
 **/

const (
	failCode    int8 = 1
	successCode int8 = 2
)

// PrepareMsg 第一阶段，提议请求
type PrepareMsg struct {
	ProposeID float32
}

// PromiseMsg 第一阶段，提议应允
type PromiseMsg struct {
	ProposeID    float32
	AcceptorAddr string

	AcceptedID    float32
	AcceptedValue interface{}
	StatusCode    int8
}

// AcceptMsg 第二阶段，发起提议内容
type AcceptMsg struct {
	ProposeID    float32
	ProposeValue interface{}
}

// AcceptedMsg 第二阶段，提议共识内容
type AcceptedMsg struct {
	ProposeID    float32
	AcceptorAddr string
	ProposeValue interface{}
	StatusCode   int8
}

// ChosenMsg 提案通过，向所有Learner发送提案结果
type ChosenMsg struct {
	ChosenValue interface{}
}

// EmptyMsg
type EmptyMsg struct {

}



