package execute

import (
	"sync"
	"time"

	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/group"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/node"
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
	node                node.Node
	state               map[string]int //世界状态
	executedTxsTotal    int            //成功的交易总数
	executedTxsForQuery int            //每次查询清零，计算两次查询之间的数
	delayTotal          time.Duration  //所有交易的执行时间
	delayTotalForQuery  time.Duration  //每次查询清零
	mbPending           mbList
	gm                  *group.GroupManager
	lock                sync.Mutex
	MbReceive           chan []*blockchain.MicroBlock
	MissReceive         chan *blockchain.MicroBlock
	executeReady        chan interface{} //是否可以尝试执行
	ReceiveResult       chan *ExecuteResult
	ResultBuffer        map[mbHash]*ExecuteResult //保存执行成功的结果
}

type ExecuteResult struct {
	PropsalId identity.NodeID  //广播者的id
	Sig       crypto.Signature //广播者的签名
	Mb        mbHash           //执行成功的块hash
	No        int              //微块顺序
	Result    string           //区块执行后的状态
}

//区块执行队列
type mbList struct {
	mbs   []*blockchain.MicroBlock                     //执行队列
	done  map[int]map[identity.NodeID]crypto.Signature //mb key：微块的顺序从1开始 value：确认的人的签名
	table map[mbHash]*blockchain.MicroBlock            //mb在list中的位置
}

func (e *Executor) HandleMB() {
	for {
		select {
		case mb := <-e.MbReceive:
			//接受执行
			if config.GetConfig().BroadcastByGroup == true {
				e.AddMbToExecute(mb)
			} else {
				//e.ExecuteForSerial(mb)
			}
		case mb := <-e.MissReceive:
			//收到丢失的区块
			e.HandleMiss(mb)
		case result := <-e.ReceiveResult:
			e.HandleResult(result)
		default:
			continue
		}
	}
}

//添加微块到执行队列中
func (e *Executor) AddMbToExecute(mb []*blockchain.MicroBlock) {
	e.lock.Lock()
	defer e.lock.Unlock()

	log.Debugf("有%v个微块添加给执行队列", len(mb))

	e.mbPending.mbs = append(e.mbPending.mbs, mb...)
	log.Debugf("添加mb到执行队列,队列长度：%v", len(e.mbPending.mbs))

	done_index := -1
	for index, v := range e.mbPending.mbs {
		if e.CheckResult(v) == true {
			done_index = index
			continue
		}
	}
	if done_index != -1 {
		log.Debugf("处理一下之前缓存的result")
		for i := 0; i <= done_index; i++ {
			e.updateState(e.mbPending.mbs[0])
			e.mbPending.mbs = e.mbPending.mbs[1:]
		}
	}
	// for _, mb := range e.mbPending.mbs {
	// 	if !mb.IsFake && e.gm.IsInMyGroup(mb.GroupId) {
	// 		log.Debugf("在执行组中 组：%v", mb.GroupId)
	// 		e.ExecuteAndBroadcast(mb)
	// 		e.mbPending.mbs = e.mbPending.mbs[1:]
	// 	}
	// }
	e.ExecuteThread()
}

func (e *Executor) ExecuteThread() { //表示是有一个mb被成功执行
	var lastmb *blockchain.MicroBlock = nil
	e.ShowQueueStatus()
	for _, mb := range e.mbPending.mbs {
		if e.gm.IsInMyGroup(mb.GroupId) {
			if mb.IsFake == true {
				//请求重传
				missStableRequest := message.MissingStableMBRequest{
					RequesterID: e.node.ID(), //本人id
					MbID:        mb.Hash,
				}
				requestNode := make([]identity.NodeID, 0)

				log.Debugf("当前需要我执行的mb:%v,没有，向组内节点要", mb.Hash)
				requestNode = append(requestNode)
				e.node.MulticastQuorum(requestNode, missStableRequest)
				break
			} else {
				log.Debugf("在执行组中 组：%v", mb.GroupId)
				lastmb = mb
				e.Execute(mb)
				e.mbPending.mbs = e.mbPending.mbs[1:]
			}
		} else {
			log.Debugf("当前队头任务%v是%v分组负责，无法执行", mb.Hash, mb.GroupId)
			break
		}
	}
	if lastmb != nil {
		sig, err := crypto.PrivSign(lastmb.Hash[:], e.node.ID(), nil)
		if err != nil {
			log.Debugf("对结果签名失败")
		} else {
			fakestate := ""
			for i := 0; i < 200; i++ {
				fakestate += "0x112312912480:32"
			}
			result := &ExecuteResult{PropsalId: e.node.ID(), Sig: sig,
				Mb:     mbHash(lastmb.Hash),
				Result: fakestate,
				No:     lastmb.CommittedNo,
			}
			//广播
			log.Debugf("区块%v结果执行完成，广播给其他节点", result.Mb)
			//e.node.MulticastQuorum2(e.gm.NotInGroup(lastmb.GroupId), result)
			e.node.Broadcast2(result)
		}
	}
}

//收到微块result
func (e *Executor) HandleResult(result *ExecuteResult) {
	e.lock.Lock()
	defer e.lock.Unlock()
	high_done_index := 0
	for i := 1; i <= result.No; i++ {
		if record, eixst := e.mbPending.done[i]; eixst {
			record[result.PropsalId] = result.Sig
		} else {
			e.mbPending.done[i] = make(map[identity.NodeID]crypto.Signature)
			e.mbPending.done[i][result.PropsalId] = result.Sig
		}

		if len(e.mbPending.done[i]) >= config.GetConfig().Q {
			//>= f+1个成功
			//收到f+1个执行结果
			high_done_index = i
		}
	}

	log.Debugf("收到来自%v的执行成功，执行的mb是%v,目前一共有个%v个执行成功", result.PropsalId, result.Mb, len(e.mbPending.done[result.No]))
	if high_done_index != 0 {
		//又可以更新的
		log.Debugf("执行队列%v以及之前的都被执行成功了", high_done_index)
		e.mbReady(high_done_index)
	}
}

//判断是微块是否已经被执行过
func (e *Executor) CheckResult(mb *blockchain.MicroBlock) bool {
	if len(e.mbPending.done[mb.CommittedNo]) >= config.GetConfig().Q {
		log.Debugf("微块添加到队列之前就被执行了")
		return true
	}
	return false
}

//收到丢失块
func (e *Executor) HandleMiss(mb *blockchain.MicroBlock) {
	e.lock.Lock()
	defer e.lock.Unlock()
	log.Debugf("receive miss mb")
	found := false
	for _, mbp := range e.mbPending.mbs {
		if mbp.Hash == mb.Hash && mbp.IsFake == true {
			//找到了阻塞块
			*mbp = *mb //深拷贝
			found = true
			log.Debugf("find miss stable %+v", mbp)
		}
	}
	log.Debugf("not find miss stable")
	if found == true {
		e.ExecuteThread()
	}
}

//mbHash对应的微块就绪了
func (e *Executor) mbReady(commitNo int) {
	end_index := -1
	for index, mb := range e.mbPending.mbs {
		if mb.CommittedNo == commitNo {
			end_index = index
		}
	}

	if end_index == -1 {
		log.Debugf("收到执行成功，但是对应的微块还没到，当前队列长度%v", len(e.mbPending.mbs))
	}

	for i := 0; i <= end_index; i++ {
		e.updateState(e.mbPending.mbs[0])
		e.mbPending.mbs = e.mbPending.mbs[1:] //丢失队头
	}

	// for _, mb := range e.mbPending.mbs {
	// 	if !mb.IsFake && e.gm.IsInMyGroup(mb.GroupId) {
	// 		e.ExecuteAndBroadcast(mb)
	// 		e.mbPending.mbs = e.mbPending.mbs[1:]
	// 	}
	// }
	log.Debugf("执行成功，执行了%v个任务,当前队列长度%v", end_index+1, len(e.mbPending.mbs))
	e.ExecuteThread()
}

var executor *Executor = nil
var mu sync.Mutex

func NewExecutor(node node.Node) *Executor {
	//单例模式
	mu.Lock()
	defer mu.Unlock()
	if executor == nil {
		log.Debugf("初始化执行器")
		executor = new(Executor)
		executor.gm = group.NewGroupManager(node.ID())
		executor.state = make(map[string]int)
		executor.node = node
		executor.mbPending = mbList{
			mbs:   make([]*blockchain.MicroBlock, 0),
			done:  make(map[int]map[identity.NodeID]crypto.Signature),
			table: make(map[mbHash]*blockchain.MicroBlock),
		}
		executor.MbReceive = make(chan []*blockchain.MicroBlock, 10000)
		executor.MissReceive = make(chan *blockchain.MicroBlock, 10000)
		executor.ReceiveResult = make(chan *ExecuteResult, 10000)
		executor.ResultBuffer = make(map[mbHash]*ExecuteResult, 10000)
		return executor
	}
	return executor
}

func (e *Executor) ExecuteForSerial(mb *blockchain.MicroBlock) {
	//用于串行执行
	e.rawExecute(mb)
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

	log.Debugf("execute mb")
	for _, transaction := range mb.Txns {

		log.Debugf("len : %v", len(transaction.Command.Value))
		log.Debugf("body: %v len : %v", transaction.Command.Value, len(transaction.Command.Value))
		e.state["1"] += 1
		e.state["2"] += 1
		e.executedTxsTotal += 1
		e.executedTxsForQuery += 1
		e.delayTotal += time.Now().Sub(transaction.Timestamp)
		e.delayTotalForQuery += time.Now().Sub(transaction.Timestamp)
		log.Debugf("execute delay = %v ms", time.Now().Sub(transaction.Timestamp).Milliseconds())
		log.Debugf("totaldelay = %v ms", e.delayTotal.Milliseconds())

	}
}

//仿真区块执行逻辑
func (e *Executor) Execute(mb *blockchain.MicroBlock) {

	log.Debugf("execute mb")
	log.Debugf("fake tx, len : %v", len(mb.Txns[0].Command.Value))
	for _, transaction := range mb.Txns {

		//time.Sleep(10 * time.Millisecond)
		e.state["1"] += 1
		e.state["2"] += 1
		e.executedTxsTotal += 1
		e.executedTxsForQuery += 1
		e.delayTotal += time.Now().Sub(transaction.Timestamp)
		e.delayTotalForQuery += time.Now().Sub(transaction.Timestamp)
	}
}

//更新被其他人执行的状态
func (e *Executor) updateState(mb *blockchain.MicroBlock) {

	log.Debugf("receive state upload mb")
	for _, transaction := range mb.Txns {
		e.executedTxsTotal += 1
		e.delayTotal += time.Now().Sub(transaction.Timestamp)
		// log.Debugf("execute delay = %v ms", time.Now().Sub(transaction.Timestamp).Milliseconds())
		// log.Debugf("totaldelay = %v ms", e.delayTotal.Milliseconds())
	}
}

//计算交易确认时延 ms
func (e *Executor) DelayForQuery() float64 {
	e.lock.Lock()
	e.lock.Unlock()
	return float64(
		float64(e.delayTotalForQuery.Milliseconds()) /
			float64(e.executedTxsForQuery),
	)
}

//计算交易确认时延 ms
func (e *Executor) TotalNum() int {
	e.lock.Lock()
	defer e.lock.Unlock()
	return e.executedTxsTotal
}

//计算交易确认时延 ms
func (e *Executor) TotalNumForQuery() int {
	e.lock.Lock()
	defer e.lock.Unlock()
	return e.executedTxsForQuery
}

//清空数据，为下次查询准备
func (e *Executor) Reset() {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.delayTotalForQuery = 0
	e.executedTxsForQuery = 0
}

//是否准备好执行当前mb,wait Id为前一个微块的id
func (e *Executor) readyForExecute(waitId int) {

}

func (e *Executor) ShowQueueStatus() {
	// e.lock.Lock()
	// defer e.lock.Unlock()

	for _, v := range e.mbPending.mbs {
		// log.Debugf("is fake:%v", v.IsFake)
		// log.Debugf("group:%v", v.GroupId)
		// log.Debugf("is In my groop:%v", e.gm.IsInMyGroup(v.GroupId))
		// if _, ok := e.mbPending.done[mbHash(v.Hash)]; ok {
		// 	log.Debugf("have receive %v done", len(e.mbPending.done[mbHash(v.Hash)]))
		// }
		log.Resultf("mb:%+v, group: %v,done:%v, mb'hash:%v", v.CommittedNo, v.GroupId, len(e.mbPending.done[v.CommittedNo]), v.Hash)
	}
}
