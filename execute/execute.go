package execute

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/group"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
)

/*
	处理已经提交的区块
*/

var startForCoop chan struct{} //是否可以开启协作处理
type GroupNum int
type mbHash crypto.Identifier

//解析comman
type Tx struct {
	From    string
	To      string
	Payload string
}

type Executor struct {
	state            map[string]int //世界状态
	executedTxsTotal int            //成功的交易总数
	executedTxs      int            //一个区块的数目
	delayTotal       time.Duration  //所有交易的执行时间
	mbPending        mbList
	gm               *group.GroupManager
	lock             sync.Mutex
}

//区块执行队列
type mbList struct {
	mbs   []*blockchain.MicroBlock                        //执行队列
	done  map[mbHash]map[identity.NodeID]crypto.Signature //mb key：mbHash value：确认的人的签名
	table map[mbHash]*blockchain.MicroBlock               //mb在list中的位置
	lock  sync.Mutex
	//	ready chan struct{} //用于唤醒阻塞在队列的节点
}

func (l *mbList) addMb(mb *blockchain.MicroBlock) {
	l.mbs = append(l.mbs, mb)
}

//收到微块result
func (l *mbHash) receiveResult() {

}

//mbHash对应的微块就绪了
func (e *Executor) mbReady(hash mbHash) {
	//执行到ready
	for _, mb := range e.mbPending.mbs {
		e.updateState(mb)                     //先用假的模拟
		e.mbPending.mbs = e.mbPending.mbs[1:] //丢失队头
		if mb.Hash == crypto.Identifier(hash) {
			log.Debugf("已经收到%v组的交易执行完成消息", mb.GroupId)
			break
		}
	}
	for _, mb := range e.mbPending.mbs {
		if e.gm.IsInMyGroup(mb.GroupId) {
			e.fakeExecute(mb)
			e.mbPending.mbs = e.mbPending.mbs[1:]
		}
	}
}

var executor *Executor = nil
var mu sync.Mutex

func NewExecutor() *Executor {
	//单例模式
	mu.Lock()
	defer mu.Unlock()
	if executor == nil {
		executor = new(Executor)
		executor.gm = group.NewGroupManager(identity.NewNodeID(config.GetConfig().N()))
		executor.state = make(map[string]int)
		return executor
	}
	return executor
}

func (e *Executor) ExecuteForSerial(mb *blockchain.MicroBlock) {
	//处理可并发微块
	e.fakeExecute(mb)
}

func (e *Executor) ExecuteForParallel(mb *blockchain.MicroBlock) {
	//处理可并发微块
	e.rawExecute(mb)
}

//协作执行
func (e *Executor) ExecuteForCoop(mb *blockchain.MicroBlock, befor GroupNum, cur GroupNum) {
	if befor == cur-1 {
		//收到前一个分组的执行结果
		//执行
	}
}

//区块执行逻辑
func (e *Executor) rawExecute(mb *blockchain.MicroBlock) {
	e.lock.Lock()
	defer e.lock.Unlock()

	log.Debugf("execute mb")
	for _, transaction := range mb.Txns {
		tx := &Tx{}
		err := json.Unmarshal(transaction.Command.Value, &tx)
		if err != nil {
			//解析成功
			log.Errorf("tx wrong : %v", err)
		} else {
			log.Debugf("len : %v", len(transaction.Command.Value))
			log.Debugf("body: %v len : %v", transaction.Command.Value, len(transaction.Command.Value))
			log.Debugf("from: %v to: %v", tx.From, tx.To)
			e.state[tx.From] += 1
			e.state[tx.To] += 1
			e.executedTxsTotal += 1
			e.delayTotal += time.Now().Sub(transaction.Timestamp)
			log.Debugf("execute delay = %v ms", time.Now().Sub(transaction.Timestamp).Milliseconds())
			log.Debugf("totaldelay = %v ms", e.delayTotal.Milliseconds())

		}

	}
}

//仿真区块执行逻辑
func (e *Executor) fakeExecute(mb *blockchain.MicroBlock) {
	e.lock.Lock()
	defer e.lock.Unlock()

	log.Debugf("execute mb")
	for _, transaction := range mb.Txns {

		log.Debugf("fake tx, len : %v", len(transaction.Command.Value))

		e.state["1"] += 1
		e.state["2"] += 1
		e.executedTxsTotal += 1
		e.delayTotal += time.Now().Sub(transaction.Timestamp)
		log.Debugf("execute delay = %v ms", time.Now().Sub(transaction.Timestamp).Milliseconds())
		log.Debugf("totaldelay = %v ms", e.delayTotal.Milliseconds())
	}
}

//更新其他组的交易状态，不广播
func (e *Executor) updateState(mb *blockchain.MicroBlock) {
	e.lock.Lock()
	defer e.lock.Unlock()

	log.Debugf("execute mb")
	for _, transaction := range mb.Txns {
		log.Debugf("fake tx, len : %v", len(transaction.Command.Value))

		e.state["1"] += 1
		e.state["2"] += 1
		e.executedTxsTotal += 1
		e.delayTotal += time.Now().Sub(transaction.Timestamp)
		log.Debugf("execute delay = %v ms", time.Now().Sub(transaction.Timestamp).Milliseconds())
		log.Debugf("totaldelay = %v ms", e.delayTotal.Milliseconds())
	}
}

//计算交易确认时延 ms
func (e *Executor) Delay() float64 {
	e.lock.Lock()
	e.lock.Unlock()
	return float64(
		float64(e.delayTotal.Milliseconds()) /
			float64(e.executedTxsTotal),
	)
}

//清空数据，为下次查询准备
func (e *Executor) Reset() {
	e.lock.Lock()
	e.lock.Unlock()
	e.delayTotal = 0
	e.executedTxsTotal = 0
}

//是否准备好执行当前mb,wait Id为前一个微块的id
func (e *Executor) readyForExecute(waitId int) {

}
