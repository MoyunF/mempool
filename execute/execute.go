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
	"github.com/gitferry/bamboo/monitor"
	"github.com/gitferry/bamboo/node"
)

/*
	处理已经提交的区块
*/

var startForCoop chan struct{} //是否可以开启协作处理
type GroupNum int
type mbHash crypto.Identifier

// 解析comman
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
	gm                  *group.GroupManager
	mbPending           mbList
	lock                sync.Mutex
	MbReceive           chan []*blockchain.MicroBlock
	MissReceive         chan *blockchain.MicroBlock
	executeReady        chan interface{} //是否可以尝试执行
	ReceiveResult       chan *ExecuteResult
	ResultBuffer        map[mbHash]*ExecuteResult //保存执行成功的结果
	monitor             *monitor.MonitorManager
}

type ExecuteResult struct {
	PropsalId identity.NodeID  //广播者的id
	Sig       crypto.Signature //广播者的签名
	Mb        mbHash           //执行成功的块hash
	No        int              //微块顺序
	Result    string           //区块执行后的状态

	buildTime   time.Time //生成结果的时间
	receiveTime time.Time //收到结果的时间
	enableTime  time.Time //结果的时间
}

// 区块执行队列
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
			e.AddMbToExecute(mb)
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

// 添加微块到执行队列中
func (e *Executor) AddMbToExecute(mb []*blockchain.MicroBlock) {
	e.lock.Lock()
	defer e.lock.Unlock()

	if config.GetConfig().BroadcastByGroup == true {

		e.mbPending.mbs = append(e.mbPending.mbs, mb...)
		log.Debugf("AddMbToExecute() --- 有%v个微块添加给执行队列，添加后的队列长度：%v", len(mb), len(e.mbPending.mbs))

		done_index := -1
		for index, v := range e.mbPending.mbs {
			if e.CheckResult(v) == true {
				done_index = index
				continue
			}
		}
		if done_index != -1 {
			log.Debugf("AddMbToExecute() --- 处理一下之前缓存的result")
			for i := 0; i <= done_index; i++ {
				e.updateState(e.mbPending.mbs[0])
				e.mbPending.mbs = e.mbPending.mbs[1:]
			}
		}
	} else {
		e.mbPending.mbs = append(e.mbPending.mbs, mb...)
		log.Debugf("AddMbToExecute() --- 添加mb到执行队列,队列长度：%v", len(e.mbPending.mbs))
	}
	e.ExecuteThread()
}

func (e *Executor) ExecuteThread() { //表示是有一个mb被成功执行
	e.ShowQueueStatus_Sample()
	if config.GetConfig().BroadcastByGroup == true {
		successNum := 0
		for curIndex, mb := range e.mbPending.mbs {
			if e.gm.IsInMyGroup(mb.GroupId) {
				if mb.IsFake == true {
					//请求重传
					missStableRequest := message.MissingStableMBRequest{
						RequesterID: e.node.ID(), //本人id
						MbID:        mb.Hash,
					}
					requestNode := make([]identity.NodeID, 0)
					log.Debugf("ExecuteThread() --- [%v] 需要执行mb:%x,但是缺少明文,向组内节点要,执行组:%v", e.node.ID(), mb.Hash, mb.GroupId)
					//TODO:requestNode不全
					requestNode = append(requestNode)
					e.node.MulticastQuorum(requestNode, missStableRequest)
					break
				} else {
					log.Debugf("ExecuteThread() --- [%v] 需要执行mb:%x,包含明文,开始执行,执行组:%v", e.node.ID(), mb.Hash, mb.GroupId)
					e.Execute(mb)
					//广播
					executeResult := e.generateExecuteResult(mb)
					e.stateBroadcastByGroup(curIndex, executeResult)
					successNum += 1
				}
			} else {
				log.Debugf("ExecuteThread() ---[%v] 当前队头任务%x是%v分组负责，无需执行", e.node.ID(), mb.Hash, mb.GroupId)
				break
			}
		}
		e.mbPending.mbs = e.mbPending.mbs[successNum:]
		log.Debugf("ExecuteThread() --- [%v] 共执行了%v个微块，当前执行队列长度", e.node.ID(), successNum, len(e.mbPending.mbs))
	} else if config.GetConfig().BroadcastBySample == true {
		//采样模式
		successNum := 0
		for curIndex, mb := range e.mbPending.mbs {
			if mb.IsFake == true {
				if _, ok := mb.GenerateNodeList[e.node.ID()]; ok {
					//如果需要我执行，但是我没有，我会要一下缺失块
					missStableRequest := message.MissingStableMBRequest{
						RequesterID: e.node.ID(), //本人id
						MbID:        mb.Hash,
					}
					log.Debugf("ExecuteThread() --- [%v] 采样模式，需要执行mb:%x,但是缺少明文,向存储明文的节点要，生成mb的节点列表:%+v", e.node.ID(), mb.Hash, mb.GenerateNodeList)
					e.node.BroadcastByGroup(missStableRequest, mb.GenerateNodeList)
					break
				} else {
					log.Debugf("ExecuteThread() --- [%v] 采样模式，mb:%x 的明文不是由当前节点存储的,生成mb的节点列表:%+v", e.node.ID(), mb.Hash, mb.GenerateNodeList)
				}
			} else {
				log.Debugf("ExecuteThread() --- [%v] 采样模式，mb:%x,包含明文,开始执行,生成mb的节点列表:%+v", e.node.ID(), mb.Hash, mb.GenerateNodeList)
				e.Execute(mb)
				//广播
				executeResult := e.generateExecuteResult(mb)
				e.stateBroadcastBySample(curIndex, executeResult)
				successNum += 1
			}
		}
		e.mbPending.mbs = e.mbPending.mbs[successNum:]
		log.Debugf("ExecuteThread() --- [%v] 采样模式, 共执行了%v个微块，当前执行队列长度", e.node.ID(), successNum, len(e.mbPending.mbs))
	} else {
		//不分组
		for _, mb := range e.mbPending.mbs {
			if mb.IsFake == true {
				//请求重传
				break
			} else {
				e.Execute(mb)
				e.mbPending.mbs = e.mbPending.mbs[1:]
			}
		}
	}
}

// 收到微块result
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

	log.Debugf("HandleResult() --- [%v] 收到来自%v的执行成功，执行的mb是%x,目前一共有个%v个执行成功", e.node.ID(), result.PropsalId, result.Mb, len(e.mbPending.done[result.No]))
	if high_done_index != 0 {
		//又可以更新的
		log.Debugf("HandleResult() --- [%v] 执行队列%v以及之前的都被执行成功了", e.node.ID(), high_done_index)
		e.mbReady(high_done_index)
	}
}

// 判断是微块是否已经被执行过
func (e *Executor) CheckResult(mb *blockchain.MicroBlock) bool {
	if len(e.mbPending.done[mb.CommittedNo]) >= config.GetConfig().Q {
		log.Debugf("HandleResult() ---[%v] 微块 [%x ]添加到队列之前就被执行了", e.node.ID(), mb.Hash)
		return true
	}
	return false
}

// 收到丢失块
func (e *Executor) HandleMiss(mb *blockchain.MicroBlock) {
	e.lock.Lock()
	defer e.lock.Unlock()
	found := false
	for _, mbp := range e.mbPending.mbs {
		if mbp.Hash == mb.Hash && mbp.IsFake == true {
			//找到了阻塞块
			*mbp = *mb //深拷贝
			found = true
			log.Debugf("HandleResult() --- [%v] 收到被提交但是本地没有存储的mb[%x]，从节点[%v]", e.node.ID(), mb.ProposalID, mbp.Hash)
		}
	}
	log.Debugf("HandleResult() --- [%v] 收到了一个丢失的区块mb [%x]从节点[%v]， 但这个小块没有被提交过", e.node.ID(), mb.ProposalID, mb.Hash)
	if found {
		e.ExecuteThread()
	}
}

// mbHash对应的微块就绪了
func (e *Executor) mbReady(commitNo int) {
	end_index := -1
	for index, mb := range e.mbPending.mbs {
		if mb.CommittedNo == commitNo {
			end_index = index
		}
	}

	if end_index == -1 {
		log.Debugf("HandleResult() --- [%v] 收到执行成功，但是对应的微块还没到，当前队列长度%v", e.node.ID(), len(e.mbPending.mbs))
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
	log.Debugf("HandleResult() ---[%v]根据收到的执行结果，更新了%v个mb的状态,当前队列长度%v", e.node.ID(), end_index+1, len(e.mbPending.mbs))
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
		executor.monitor = monitor.NewMonitorManager()
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

// 仿真区块执行逻辑
func (e *Executor) Execute(mb *blockchain.MicroBlock) {
	e.monitor.CollectExecuteTime(mb.Hash, time.Now().Sub(mb.CreateTimeStamp))
	log.Debugf("Execute() : [%v] execute mb [%x]", e.node.ID(), mb.Hash)
	for _, transaction := range mb.Txns {
		e.state["1"] += 1
		e.state["2"] += 1
		e.executedTxsTotal += 1
		e.executedTxsForQuery += 1
		e.delayTotal += time.Now().Sub(transaction.Timestamp)
		e.delayTotalForQuery += time.Now().Sub(transaction.Timestamp)
	}
}

// 更新被其他人执行的状态
func (e *Executor) updateState(mb *blockchain.MicroBlock) {

	log.Debugf("updateState() : [%v] 更新mb [%x] 后执行的状态", e.node.ID(), mb.Hash)
	for _, transaction := range mb.Txns {
		e.executedTxsTotal += 1
		e.delayTotal += time.Now().Sub(transaction.Timestamp)
		// log.Debugf("execute delay = %v ms", time.Now().Sub(transaction.Timestamp).Milliseconds())
		// log.Debugf("totaldelay = %v ms", e.delayTotal.Milliseconds())
	}
}

// 计算交易确认时延 ms
func (e *Executor) DelayForQuery() float64 {
	e.lock.Lock()
	e.lock.Unlock()
	return float64(
		float64(e.delayTotalForQuery.Milliseconds()) /
			float64(e.executedTxsForQuery),
	)
}

// 计算交易确认时延 ms
func (e *Executor) TotalNum() int {
	e.lock.Lock()
	defer e.lock.Unlock()
	return e.executedTxsTotal
}

// 计算交易确认时延 ms
func (e *Executor) TotalNumForQuery() int {
	e.lock.Lock()
	defer e.lock.Unlock()
	return e.executedTxsForQuery
}

// 清空数据，为下次查询准备
func (e *Executor) Reset() {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.delayTotalForQuery = 0
	e.executedTxsForQuery = 0
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
		log.Resultf("mb:%v, group: %v,done:%v, mb'hash:%x", v.CommittedNo, v.GroupId, len(e.mbPending.done[v.CommittedNo]), v.Hash)
	}
}

func (e *Executor) ShowQueueStatus_Sample() {
	// e.lock.Lock()
	// defer e.lock.Unlock()

	i := 1
	for _, v := range e.mbPending.mbs {
		log.Resultf("当前执行队列第%v个: mb:%v, GenerateList: %+v,done:%v, mb'hash:%x", i, v.CommittedNo, v.GenerateNodeList, len(e.mbPending.done[v.CommittedNo]), v.Hash)
		i++
		if i >= 10 {
			break
		}
	}
}

func (e *Executor) generateExecuteResult(mb *blockchain.MicroBlock) *ExecuteResult {
	sig, err := crypto.PrivSign(mb.Hash[:], e.node.ID(), nil)
	if err != nil {
		log.Debugf("generateExecuteResult() ---[%v]对结果签名失败", e.node.ID())
	} else {
		fakestate := ""
		for i := 0; i < 200; i++ {
			fakestate += "0x112312912480:32"
		}
		result := &ExecuteResult{
			PropsalId: e.node.ID(),
			Sig:       sig,
			Mb:        mbHash(mb.Hash),
			Result:    fakestate,
			No:        mb.CommittedNo,
			buildTime: time.Now(),
		}
		//广播
		log.Debugf("generateExecuteResult() ---[%v] mb [%x]执行后的结果已经获取", e.node.ID(), mb.Hash)
		return result
	}
	log.Debugf("generateExecuteResult() ---[%v] mb [%x]执行后的结果获取失败", e.node.ID(), mb.Hash)
	return &ExecuteResult{}
}

// 采用分组方案时，执行状态广播操作，根据mb的存储情况，判断需要向哪些节点广播执行结果
func (e *Executor) stateBroadcastByGroup(curIndex int, executeResult *ExecuteResult) {
	if curIndex != len(e.mbPending.mbs)-1 {
		targetMember := make(map[identity.NodeID]struct{})
		//不是最后一个微块
		currentMb := e.mbPending.mbs[curIndex]
		nextMb := e.mbPending.mbs[curIndex+1]
		currentGroup := currentMb.GroupId
		nextGroup := nextMb.GroupId
		currentMemberList := e.gm.GetGroupListByGroupId(currentGroup)
		nextMemberList := e.gm.GetGroupListByGroupId(nextGroup)

		for member := range nextMemberList {
			//log.Debugf("stateBroadcastByGroup() --- Checking member: %+v in currentMemberList: %+v", member, currentMemberList)
			if _, ok := currentMemberList[member]; !ok {
				//如果下一个微块中的节点，没有存储当前区块，则需要把结果广播给它
				targetMember[member] = struct{}{}
				log.Debugf("stateBroadcastByGroup() --- [%v]，精细化广播执行结果，目标节点[%v],当前mb:%x", e.node.ID().Node(), member, currentMb.Hash)
			}
		}
		log.Debugf("stateBroadcastByGroup() --- [%v]，精细化广播执行结果，目标节点列表[%+v],当前mb:%x", e.node.ID().Node(), targetMember, currentMb.Hash)
		e.node.BroadcastByGroup(executeResult, targetMember)
	} else {
		currentMb := e.mbPending.mbs[curIndex]
		//如果是队列中最后一个块，广播
		log.Debugf("stateBroadcastByGroup() --- [%v]，当前是mb是队列中的最后一个块，因此广播给全员,当前mb:%x", e.node.ID().Node(), currentMb.Hash)
		e.node.Broadcast(executeResult)
	}

}

// 采用采样方案时，执行状态广播操作，根据mb的存储情况，判断需要向哪些节点广播执行结果
func (e *Executor) stateBroadcastBySample(curIndex int, executeResult *ExecuteResult) {
	if curIndex != len(e.mbPending.mbs)-1 {
		targetMember := make(map[identity.NodeID]struct{})
		//不是最后一个微块
		currentMb := e.mbPending.mbs[curIndex]
		nextMb := e.mbPending.mbs[curIndex+1]
		currentMemberList := currentMb.GenerateNodeList
		nextMemberList := nextMb.GenerateNodeList

		for member := range nextMemberList {
			log.Debugf("stateBroadcastBySample() --- Checking member: %+v in currentMemberList: %+v", member, currentMemberList)
			if _, ok := currentMemberList[member]; !ok {
				//如果下一个微块中的节点，没有存储当前区块，则需要把结果广播给它
				targetMember[member] = struct{}{}
				log.Debugf("stateBroadcastBySample() --- [%v]，精细化广播执行结果，目标节点[%v],当前mb:%x", e.node.ID().Node(), member, currentMb.Hash)
			}
		}
		log.Debugf("stateBroadcastBySample() --- [%v]，精细化广播执行结果，目标节点列表[%+v],当前mb:%x", e.node.ID().Node(), targetMember, currentMb.Hash)
		e.node.BroadcastByGroup(executeResult, targetMember)
	} else {
		currentMb := e.mbPending.mbs[curIndex]
		//如果是队列中最后一个块，广播
		log.Debugf("stateBroadcastBySample() --- [%v]，当前是mb是队列中的最后一个块，因此广播给全员,当前mb:%x", e.node.ID().Node(), currentMb.Hash)
		e.node.Broadcast(executeResult)
	}

}
