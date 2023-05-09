package execute

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/log"
)

/*
	处理已经提交的区块
*/

var startForCoop chan struct{} //是否可以开启协作处理
type GroupNum int

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
	lock             sync.Mutex
}

var executor *Executor = nil
var mu sync.Mutex

func NewExecutor() *Executor {
	//单例模式
	mu.Lock()
	defer mu.Unlock()
	if executor == nil {
		executor = new(Executor)
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

//假的区块执行逻辑
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

//计算交易确认时延 ms
func (e *Executor) Delay() float64 {
	e.lock.Lock()
	e.lock.Unlock()
	return float64(
		float64(e.delayTotal.Milliseconds()) /
			float64(e.executedTxsTotal),
	)
}
