package paxos
/**
 * @DateTime   : 2020/11/11
 * @Author     : xumamba
 * @Description: 工具集
 **/

import (
	"fmt"
	"net/rpc"
	"strconv"
)

var isUnstableNetwork = true // 模拟不稳定的网络环境

// generateProposeID 生成提议请求标识ID
func generateProposeID(nodeID int, historyMaxPID float32) float32 {
	var strNum string
	if historyMaxPID == 0 {
		strNum = fmt.Sprintf("0.%d", nodeID)
		floatNum, err := strconv.ParseFloat(strNum, 32)
		if err != nil {
			panic(err)
		}
		return float32(floatNum)
	}
	maxPID := int(historyMaxPID) + 1
	strNum = fmt.Sprintf("%d.%d", maxPID, nodeID)
	floatNum, err := strconv.ParseFloat(strNum, 32)
	if err != nil {
		panic(err)
	}
	return float32(floatNum)
}

// callRPC 消息传递
func callRPC(addr, serviceMethod string, args interface{}, reply interface{}) error {
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		return err
	}
	err = client.Call(serviceMethod, args, reply)
	return err
}

// errHandle 错误处理
func errHandle(err error) {
	if err != nil{
		fmt.Println("出错啦：", err)
	}
}