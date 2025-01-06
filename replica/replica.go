package replica

import (
	"encoding/gob"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/execute"
	"github.com/gitferry/bamboo/group"
	"github.com/gitferry/bamboo/kafka"
	"github.com/gitferry/bamboo/limiter"
	"github.com/gitferry/bamboo/txpool"
	"github.com/gitferry/bamboo/utils"
	"github.com/kelindar/bitmap"

	"go.uber.org/atomic"

	"github.com/gitferry/bamboo/blockchain"
	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/election"
	"github.com/gitferry/bamboo/hotstuff"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/mempool"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/monitor"
	"github.com/gitferry/bamboo/node"
	"github.com/gitferry/bamboo/pacemaker"
	"github.com/gitferry/bamboo/types"

	"net/http"
	_ "net/http/pprof" // 导入 pprof 包，它会自动初始化 pprof 路由
)

type Replica struct {
	node.Node
	Safety
	election.Election
	sm      mempool.SharedMempool
	pm      *pacemaker.Pacemaker
	ex      *execute.Executor
	gm      *group.GroupManager
	Pool    *txpool.Txpool
	monitor *monitor.MonitorManager
	/*for group by lxx*/

	kafkaProducer *kafka.KafkaProducer

	//estimator       *Estimator
	start           chan bool // signal to start the node
	isStarted       atomic.Bool
	isByz           bool
	timer           *time.Timer // timeout for each view
	committedBlocks chan *blockchain.Block
	forkedBlocks    chan *blockchain.Block
	eventChan       chan interface{}
	mbBroadcast     chan interface{}
	/* for monitoring node statistics */
	thrus                     string
	lastViewTime              time.Time
	startTime                 time.Time
	tmpTime                   time.Time
	voteStart                 time.Time
	totalCreateDuration       time.Duration
	totalProcessDuration      time.Duration
	totalProposeDuration      time.Duration
	totalDisseminationTime    time.Duration
	totaRealDissminationTime  time.Duration //收到的完整微块的时间
	totalDelay                time.Duration
	totalRoundTime            time.Duration
	totalVoteTime             time.Duration
	totalSlowDisemminationDur time.Duration
	totalStableTime           time.Duration
	totalSlowMBs              int
	totalBlockSize            int
	totalMicroblocks          int
	totalProposedMBs          int
	totalRealMBS              int //完整微块
	missingMicroblocks        int
	receivedNo                int
	roundNo                   int
	voteNo                    int
	totalCommittedTx          int
	latencyNo                 int
	proposedNo                int
	processedNo               int
	committedNo               int
	totalHops                 int
	totalCommittedMBs         int
	totalRedundantMBs         int
	totalReceivedTxs          int
	txNoInMB                  int
	commitMbNo                int //提交的微块序号
	CommitedMb                map[crypto.Identifier]struct{}
	missingCounts             map[identity.NodeID]int
	pendingBlockMap           map[crypto.Identifier]*blockchain.PendingBlock
	missingMBs                map[crypto.Identifier]crypto.Identifier // microblock hash to proposal hash
	receivedMBs               map[crypto.Identifier]struct{}
	selfMBChan                chan blockchain.MicroBlock
	otherMBChan               chan blockchain.MicroBlock
	poolChan                  chan interface{}
	limiter                   *limiter.Bucket
	mbSentNodes               map[crypto.Identifier]bitmap.Bitmap

	hasMiss chan bool //是否收到的微块已经被执行了

	totalTx int //提交的总交易数
	result  string
}

// NewReplica creates a new replica instance
func NewReplica(id identity.NodeID, alg string, isByz bool) *Replica {
	r := new(Replica)
	r.Node = node.NewNode(id, isByz)
	if isByz {
		log.Infof("[%v] is Byzantine", r.ID())
	}
	if config.GetConfig().Master == "0" {
		r.Election = election.NewRotation(config.GetConfig().N())
	} else {
		r.Election = election.NewStatic(config.GetConfig().Master)
	}
	r.isByz = isByz
	r.pm = pacemaker.NewPacemaker(config.GetConfig().N())
	//增加执行器
	r.ex = execute.NewExecutor(r.Node)
	//增加分组器
	r.gm = group.NewGroupManager(r.ID())
	//交易池
	r.Pool = txpool.NewTxpool(r.Node)
	//监控器
	r.monitor = monitor.NewMonitorManager()
	//限制微块广播，用来控制最多可以广播多少微块
	r.mbBroadcast = make(chan interface{}, config.GetConfig().Mb_broadcast)
	//r.estimator = NewEstimator()
	r.CommitedMb = make(map[crypto.Identifier]struct{})
	r.start = make(chan bool)
	r.eventChan = make(chan interface{})
	r.poolChan = make(chan interface{}, config.GetConfig().Poolsize)
	r.committedBlocks = make(chan *blockchain.Block, 100)
	r.forkedBlocks = make(chan *blockchain.Block, 100)
	r.pendingBlockMap = make(map[crypto.Identifier]*blockchain.PendingBlock)
	r.missingMBs = make(map[crypto.Identifier]crypto.Identifier)
	r.receivedMBs = make(map[crypto.Identifier]struct{})
	r.missingCounts = make(map[identity.NodeID]int)
	r.selfMBChan = make(chan blockchain.MicroBlock, 1024)
	r.otherMBChan = make(chan blockchain.MicroBlock, 1024)
	r.mbSentNodes = make(map[crypto.Identifier]bitmap.Bitmap)
	r.limiter = limiter.NewBucket(time.Duration(config.Configuration.FillInterval)*time.Millisecond, int64(config.Configuration.Capacity))
	//消息队列
	if config.GetConfig().MessageQueue.Enable {
		r.kafkaProducer, _ = kafka.NewKafkaProducer(config.GetConfig().MessageQueue.Address, config.GetConfig().MessageQueue.Topic)
	}
	memType := config.GetConfig().MemType
	switch memType {
	// case "naive":
	// 	r.sm = mempool.NewNaiveMem()
	//case "time":
	//	r.sm = mempool.NewTimemem()
	case "ack":
		r.sm = mempool.NewAckMem(r.Node, r.gm)
	}
	r.Register(blockchain.Proposal{}, r.HandleProposal)
	r.Register(blockchain.MicroBlock{}, r.HandleMicroblock)
	r.Register(blockchain.Vote{}, r.HandleVote)
	r.Register(pacemaker.TMO{}, r.HandleTmo)
	r.Register(message.Transaction{}, r.handleTxn)
	r.Register(message.Query{}, r.handleQuery)
	r.Register(message.MissingMBRequest{}, r.HandleMissingMBRequest)
	r.Register(message.MissingStableMBRequest{}, r.HandleMissingStableMb)
	r.Register(blockchain.Ack{}, r.HandleAck)
	r.Register(execute.ExecuteResult{}, r.handleResult)
	r.Register(blockchain.Stable{}, r.HandleStable)

	gob.Register(blockchain.Proposal{})
	gob.Register(blockchain.MicroBlock{})
	gob.Register(blockchain.Vote{})
	gob.Register(pacemaker.TC{})
	gob.Register(pacemaker.TMO{})
	gob.Register(message.MissingMBRequest{})
	gob.Register(blockchain.Ack{})
	gob.Register(execute.ExecuteResult{})
	gob.Register(blockchain.Stable{})
	gob.Register(message.MissingStableMBRequest{})

	// Is there a better way to reduce the number of parameters?
	switch alg {
	case "hotstuff":
		r.Safety = hotstuff.NewHotStuff(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	//case "tchs":
	//	r.Safety = tchs.NewTchs(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	//case "streamlet":
	//	r.Safety = streamlet.NewStreamlet(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	//case "lbft":
	//	r.Safety = lbft.NewLbft(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	//case "fasthotstuff":
	//	r.Safety = fhs.NewFhs(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	default:
		r.Safety = hotstuff.NewHotStuff(r.Node, r.pm, r.Election, r.committedBlocks, r.forkedBlocks)
	}
	return r
}

/* Message Handlers */

// HandleProposal handles proposals from the leader
// it first checks if the referred microblocks exist in the mempool
// and requests the missing ones
func (r *Replica) HandleProposal(proposal blockchain.Proposal) {

	r.receivedNo++
	r.startSignal()
	r.totalProposeDuration += time.Now().Sub(proposal.Timestamp)
	log.Debugf("HandleProposal() --- [%v] received a proposal from %v, containing %v microblocks, view is %v, id: %x, prevID: %x", r.ID(), proposal.Proposer, len(proposal.HashList), proposal.View, proposal.ID, proposal.PrevID)
	r.totalBlockSize += len(proposal.HashList)
	pendingBlock := r.sm.FetchMB(&proposal)
	block := pendingBlock.CompleteBlock() //看一下有没有缺的
	if block != nil {
		log.Debugf("HandleProposal() --- [%v] a block is ready, view: %v, id: %x", r.ID(), proposal.View, proposal.ID)
		r.eventChan <- *block
		return
	}
}

func (r *Replica) HandleStable(stable blockchain.Stable) {
	log.Debugf("HandleStable() --- [%v] receive a stable from [%v], mb's hash[%x]", r.ID(),
		stable.Sender, stable.MicroblockID)
	r.sm.AddStable(&stable)

	sendThreshold := config.Configuration.Q
	if r.sm.GetStablePerRound() >= sendThreshold {
		log.Debugf("HandleStable() --- [%v] 已经接受了 [%v] 个上一轮的mb，开始下一轮广播", r.ID(), r.sm.GetStablePerRound())
		r.sm.ResetStablePerRound()
		r.mbBroadcast <- struct{}{}
	}
}

// HandleMicroblock handles microblocks from replicas
// it first checks if the relevant proposal is pending
// if so, tries to complete the block
func (r *Replica) HandleMicroblock(mb blockchain.MicroBlock) {
	// r.startSignal()
	// // gossip
	// //if a quorum of acks is not reached, gossip the microblock
	// go func() {
	// 	if config.Configuration.Gossip == true && !mb.IsRequested && r.ID() != config.Configuration.Master {
	// 		mb.Hops++
	// 		if mb.Hops <= config.Configuration.R {
	// 			r.otherMBChan <- mb
	// 		}
	// 	}
	// }()
	// if config.Configuration.LoadBalance && mb.IsForward {
	// 	mb.IsForward = false
	// 	r.Broadcast(mb)
	// 	return
	// }
	// _, ok := r.receivedMBs[mb.Hash]
	// if ok {
	// 	r.totalRedundantMBs++
	// 	return
	// }
	// defer r.kickOff()
	// if config.GetConfig().BroadcastByGroup == true && r.gm.IsInMyGroup(mb.GroupId) {
	// 	r.totaRealDissminationTime += time.Now().Sub(mb.Timestamp)
	// 	r.totalRealMBS++
	// }
	// r.totalDisseminationTime += time.Now().Sub(mb.Timestamp)
	// if mb.Sender.Node() <= config.Configuration.SlowNo {
	// 	r.totalSlowDisemminationDur += time.Now().Sub(mb.Timestamp)
	// 	r.totalSlowMBs++
	// }
	// r.receivedMBs[mb.Hash] = struct{}{}
	// r.totalMicroblocks++
	// mb.FutureTimestamp = time.Now()

	// log.Debugf("HandleMircoblock() --- [%v] received a microblock from [%v], mb's hash: %x", r.ID(), mb.Sender, mb.Hash)
	// // proposalID, exists := r.missingMBs[mb.Hash]
	// if mb.IsRequested {
	// 	//是丢失块,调用丢失处理逻辑 TODO:处理丢失请求的函数
	// 	log.Debugf("HandleMircoblock() --- [%v] a missing mb is found, mb's hash:[%x]", r.ID(), mb.Hash)
	// 	r.sm.HandleMissingStableMb(&mb)
	// 	r.ex.MissReceive <- &mb
	// } else {
	// 	err := r.sm.AddMicroblock(&mb)
	// 	if err != nil {
	// 		log.Errorf("HandleMircoblock() ---[%v] can not add a microblock, mb's hash: %x", r.ID(), mb.Hash)
	// 	}
	// 	// ack
	// 	if !mb.IsRequested && config.Configuration.MemType == "ack" {
	// 		ack := blockchain.MakeAck(r.ID(), mb.Hash)
	// 		if config.GetConfig().BroadcastByGroup == true && !r.gm.IsInMyGroup(mb.GroupId) {
	// 			log.Debugf("HandleMircoblock() --- [%v] recieved a outgroup mb, mb'hash [%x], ignore", r.ID(), mb.Hash)
	// 			ack.OutGroup = true
	// 		}
	// 		if mb.Sender != r.ID() {
	// 			log.Debugf("HandleMircoblock() --- [%v] receive a mb, reply ack to [%v], mb's hash [%x]", r.ID(), mb.Sender, mb.Hash)
	// 			r.Send(mb.Sender, blockchain.MakeAck(r.ID(), mb.Hash))
	// 			r.monitor.CollectMbReceiveTime(mb.Hash, time.Now().Sub(mb.CreateTimeStamp))
	// 		} else {
	// 			r.HandleAck(*ack)
	// 		}
	// 	}
	// }
}

func (r *Replica) HandleMissingMBRequest(mbr message.MissingMBRequest) {
	log.Debugf("[%v] %d missing microblocks request is received from %v, missing mbs are:", r.ID(), len(mbr.MissingMBList), mbr.RequesterID)
	r.missingCounts[mbr.RequesterID] += len(mbr.MissingMBList)
	for _, mbid := range mbr.MissingMBList {
		found, mb := r.sm.FindMicroblock(mbid)
		log.Debugf("[%v] id: %x", r.ID(), mbid)
		if found {
			mb.IsRequested = true
			r.Send(mbr.RequesterID, mb)
		} else {
			log.Errorf("[%v] a requested microblock is not found in mempool, id: %x", r.ID(), mbid)
		}
	}
}

// lxx写的，重传对方没收到的stable块
func (r *Replica) HandleMissingStableMb(mbr message.MissingStableMBRequest) {
	log.Debugf("[%v] missing microblocks request is received from %v, missing mbs are: %x", r.ID(), mbr.RequesterID, mbr.MbID)
	// r.missingCounts[mbr.RequesterID] += len(mbr.MissingMBList)
	// for _, mbid := range mbr.MissingMBList {
	// 	found, mb := r.sm.FindMicroblock(mbid)
	// 	log.Debugf("[%v] id: %x", r.ID(), mbid)
	// 	if found {
	// 		mb.IsRequested = true
	// 		r.Send(mbr.RequesterID, mb)
	// 	} else {
	// 		log.Errorf("[%v] a requested microblock is not found in mempool, id: %x", r.ID(), mbid)
	// 	}
	// }
	found, mb := r.sm.FindMicroblock(mbr.MbID)
	if found {
		mb.IsRequested = true
		r.Send(mbr.RequesterID, mb)
	} else {
		log.Errorf("[%v] a requested microblock is not found in mempool, id: %x", r.ID(), mbr.MbID)
	}
}

func (r *Replica) HandleVote(vote blockchain.Vote) {
	log.Debugf("[%v] received a vote frm %v, blockID is %x", r.ID(), vote.Voter, vote.BlockID)
	if vote.View < r.pm.GetCurView() {
		//log.Warningf("[%v] received a vote has lower view", r.ID())
		return
	}
	r.totalVoteTime += time.Now().Sub(vote.Timestamp)
	r.voteNo++
	//r.startSignal()
	//log.Debugf("[%v] received a vote frm %v, blockID is %x", r.ID(), vote.Voter, vote.BlockID)
	r.eventChan <- vote
}

func (r *Replica) HandleTmo(tmo pacemaker.TMO) {
	log.Debugf("[%v] received a timeout from %v for view %v", r.ID(), tmo.NodeID, tmo.View)
	if tmo.View < r.pm.GetCurView() {
		return
	}
	r.eventChan <- tmo
}

func (r *Replica) HandleAck(ack blockchain.Ack) {
	log.Debugf("HandleAck() --- [%v] received an ack message form [%v] for mb's hash[%x]", r.ID(), ack.Receiver, ack.MicroblockID)
	r.processAcks(&ack)
}

func (r *Replica) handleResult(result execute.ExecuteResult) {
	log.Debugf("handleResult() --- [%v] received result from %v", r.ID(), result.PropsalId)
	r.eventChan <- result
}

var queryMu sync.Mutex

// handleQuery replies a query with the statistics of the node
func (r *Replica) handleQuery(m message.Query) {
	queryMu.Lock()
	defer queryMu.Unlock()
	//realAveProposeTime := float64(r.totalProposeDuration.Milliseconds()) / float64(r.processedNo)
	//aveProcessTime := float64(r.totalProcessDuration.Milliseconds()) / float64(r.processedNo)
	//aveVoteProcessTime := float64(r.totalVoteTime.Milliseconds()) / float64(r.roundNo)
	aveBlockSize := float64(r.totalBlockSize) / float64(r.proposedNo+r.receivedNo)
	//requestRate := float64(r.sm.TotalReceivedTxNo()) / time.Now().Sub(r.startTime).Seconds()
	//committedRate := float64(r.committedNo) / time.Now().Sub(r.startTime).Seconds()
	//aveRoundTime := float64(r.totalRoundTime.Milliseconds()) / float64(r.roundNo)
	//aveProposeTime := aveRoundTime - aveProcessTime - aveVoteProcessTime
	//latency := float64(r.totalDelay.Milliseconds()) / float64(r.latencyNo)
	r.thrus += fmt.Sprintf("Time:%v TxPool:%v StableTPS:%v ,StableDelay:%v, TotalTx:%v TotalExectuedTx:%v Throughput:%v Delay:%v\n",
		time.Now().Sub(r.startTime).Seconds(),
		r.Pool.TxLen(),
		float64(r.sm.TotalStableMb())/time.Now().Sub(r.startTime).Seconds(),
		float64(r.sm.TotalStableTime().Milliseconds())/float64(r.sm.TotalStableMb()),
		r.totalTx,
		r.ex.TotalNum(),
		float64(r.totalCommittedTx)/time.Now().Sub(r.tmpTime).Seconds(), //tps 从收到proposal开始计时
		float64(r.totalDelay.Milliseconds())/float64(r.latencyNo),       //delay 交易从被提出到确认的时间
	)
	r.totalCommittedTx = 0
	r.tmpTime = time.Now()
	r.totalDelay = 0
	r.latencyNo = 0
	r.ex.Reset()
	aveCreationTime := float64(r.totalCreateDuration.Milliseconds()) / float64(r.proposedNo)
	aveTxRate := float64(r.sm.TotalTx()) / time.Now().Sub(r.startTime).Seconds()
	aveRoundTime := float64(r.totalRoundTime.Milliseconds()) / float64(r.roundNo)
	aveHops := float64(r.totalHops) / float64(r.getTotalCommittedBlock())
	aveProposeTime := float64(r.totalProposeDuration.Milliseconds()) / float64(r.receivedNo)
	aveDisseminationTime := float64(r.totalDisseminationTime.Milliseconds()) / float64(r.totalMicroblocks)
	aveRealDissTime := aveDisseminationTime
	if config.GetConfig().BroadcastByGroup == true {
		aveRealDissTime = float64(r.totaRealDissminationTime.Milliseconds()) / float64(r.totalRealMBS)
	}
	aveSlowDisseminationTime := float64(r.totalSlowDisemminationDur.Milliseconds()) / float64(r.totalSlowMBs)
	r.totalSlowDisemminationDur = 0
	r.totalSlowMBs = 0
	aveVoteTime := float64(r.totalVoteTime.Milliseconds()) / float64(r.voteNo)
	mbRate := float64(r.sm.TotalMB()) / time.Now().Sub(r.startTime).Seconds()
	//status := fmt.Sprintf("chain status is: %s\nCommitted rate is %v.\nAve. block size is %v.\nAve. trans. delay is %v ms.\nAve. creation time is %f ms.\nAve. processing time is %v ms.\nAve. vote time is %v ms.\nRequest rate is %f txs/s.\nAve. round time is %f ms.\nLatency is %f ms.\nThroughput is %f txs/s.\n", r.Safety.GetChainStatus(), committedRate, aveBlockSize, aveTransDelay, aveCreateDuration, aveProcessTime, aveVoteProcessTime, requestRate, aveRoundTime, latency, throughput)
	//status := fmt.Sprintf("Ave. actual proposing time is %v ms.\nAve. proposing time is %v ms.\nAve. processing time is %v ms.\nAve. vote time is %v ms.\nAve. block size is %v.\nAve. round time is %v ms.\nLatency is %v ms.\n", realAveProposeTime, aveProposeTime, aveProcessTime, aveVoteProcessTime, aveBlockSize, aveRoundTime, latency)
	status := fmt.Sprintf(" Leader:%v\n Ave Real Time:%v\n. Ave. View Time: %vms\nAve. Propose Time: %vms\nAve. Dissemination Time: %vms, slow dissemination time: %v\nAve. Creation Time: %v, a proposal contains %v microblocks\nAve. Vote Time: %vms\nAve. Tx Rate: %v\nAve. MB Rate: %v, an MB contains %v txs\nRedundant microblocks:%v\nTotal microblocks: %v, Remaining microblocks: %v\nTotal missing microblocks: %v\nTotoal proposed microblocks:%v\nAve. hops:%v\nSend Rate: %v Mbps\nRecv Rate: %v Mbps\nTotal txs: %v, Remaining txs: %v\n, StableMb :%v, PendingMb : %v\n%s\n",
		r.GetCurrentLeader(), aveRealDissTime, aveRoundTime, aveProposeTime, aveDisseminationTime, aveSlowDisseminationTime, aveCreationTime, aveBlockSize, aveVoteTime, aveTxRate, mbRate, r.txNoInMB, r.totalRedundantMBs, r.sm.TotalMB(), r.sm.RemainingMB(), r.missingMicroblocks, r.totalProposedMBs, aveHops, r.SendRate(), r.RecvRate(), r.sm.TotalTx(), r.sm.RemainingTx(), r.sm.StableMB(), r.sm.PendingMB(), r.thrus)
	m.Reply(message.QueryReply{Info: status})
	if config.GetConfig().MessageQueue.Enable {
		log.Debugf("发送到消息队列中")
		r.kafkaProducer.SendMessage(status)
		log.Debugf("发送完成")
	}
}

/*
		每个1s打一下TPS，统计最高值
		时延：计算出每个小块的时延
		区块执行效率统计：
			1. 执行全部区块所用的时间
			2. 交易执行数量随时间的变化曲线 间隔1s
			3. 交易TPS = 执行成功的交易 / 时间
			4. 交易时延 = 交易的总时延 / 交易数

	    每秒钟，发送当前成功执行的 节前时间戳点号 确认阈值 小块编号 小块执行完成时间 当

	 1. 全部小块执行成功后 / （t_最后一个小块的时间戳 - 小块被提交的提交时间）

	 2. 对时间戳进行四舍五入近似 或者 画平滑曲线

	 3. 对2的每个时间戳求TPS，取max

	 4. 对于每一个成功的小块：累加（t_小块的时间戳 - 小块被提交的提交时间）/ 小块数量
*/
func (r *Replica) sendExecutedResult() {

}

// 将结果保存到日志中
func (r *Replica) saveQuery() {
	queryMu.Lock()
	defer queryMu.Unlock()
	//realAveProposeTime := float64(r.totalProposeDuration.Milliseconds()) / float64(r.processedNo)
	//aveProcessTime := float64(r.totalProcessDuration.Milliseconds()) / float64(r.processedNo)
	//aveVoteProcessTime := float64(r.totalVoteTime.Milliseconds()) / float64(r.roundNo)
	aveBlockSize := float64(r.totalBlockSize) / float64(r.proposedNo+r.receivedNo)
	//requestRate := float64(r.sm.TotalReceivedTxNo()) / time.Now().Sub(r.startTime).Seconds()
	//committedRate := float64(r.committedNo) / time.Now().Sub(r.startTime).Seconds()
	//aveRoundTime := float64(r.totalRoundTime.Milliseconds()) / float64(r.roundNo)
	//aveProposeTime := aveRoundTime - aveProcessTime - aveVoteProcessTime
	//latency := float64(r.totalDelay.Milliseconds()) / float64(r.latencyNo)
	r.thrus += fmt.Sprintf("Time:%v TxPool:%v StableMbPerSecond:%v StableDelay:%v, TotalTx:%v TotalExectuedTx:%v Throughput:%v Delay:%v AveTxExecutedDelay:%v\n",
		time.Now().Sub(r.startTime).Seconds(),
		r.Pool.TxLen(),
		float64(r.sm.TotalStableMb())/time.Now().Sub(r.startTime).Seconds(),
		float64(r.sm.TotalStableTime().Milliseconds())/float64(r.sm.TotalStableMb()),
		r.totalTx,
		r.ex.TotalNum(),
		float64(r.totalCommittedTx)/time.Now().Sub(r.tmpTime).Seconds(), //tps 从收到proposal开始计时
		float64(r.totalDelay.Milliseconds())/float64(r.latencyNo),       //delay 交易从被提出到确认的时间
		r.ex.DelayForQuery(), //执行时延
	)
	r.totalCommittedTx = 0
	r.tmpTime = time.Now()
	r.totalDelay = 0
	r.latencyNo = 0
	r.ex.Reset()
	aveCreationTime := float64(r.totalCreateDuration.Milliseconds()) / float64(r.proposedNo)
	aveTxRate := float64(r.sm.TotalTx()) / time.Now().Sub(r.startTime).Seconds()
	aveRoundTime := float64(r.totalRoundTime.Milliseconds()) / float64(r.roundNo)
	aveHops := float64(r.totalHops) / float64(r.getTotalCommittedBlock())
	aveProposeTime := float64(r.totalProposeDuration.Milliseconds()) / float64(r.receivedNo)
	aveDisseminationTime := float64(r.totalDisseminationTime.Milliseconds()) / float64(r.totalMicroblocks)
	aveRealDissTime := aveDisseminationTime
	if config.GetConfig().BroadcastByGroup == true {
		aveRealDissTime = float64(r.totaRealDissminationTime.Milliseconds()) / float64(r.totalRealMBS)
	}
	aveSlowDisseminationTime := float64(r.totalSlowDisemminationDur.Milliseconds()) / float64(r.totalSlowMBs)
	r.totalSlowDisemminationDur = 0
	r.totalSlowMBs = 0
	aveVoteTime := float64(r.totalVoteTime.Milliseconds()) / float64(r.voteNo)
	mbRate := float64(r.sm.TotalMB()) / time.Now().Sub(r.startTime).Seconds()
	//status := fmt.Sprintf("chain status is: %s\nCommitted rate is %v.\nAve. block size is %v.\nAve. trans. delay is %v ms.\nAve. creation time is %f ms.\nAve. processing time is %v ms.\nAve. vote time is %v ms.\nRequest rate is %f txs/s.\nAve. round time is %f ms.\nLatency is %f ms.\nThroughput is %f txs/s.\n", r.Safety.GetChainStatus(), committedRate, aveBlockSize, aveTransDelay, aveCreateDuration, aveProcessTime, aveVoteProcessTime, requestRate, aveRoundTime, latency, throughput)
	//status := fmt.Sprintf("Ave. actual proposing time is %v ms.\nAve. proposing time is %v ms.\nAve. processing time is %v ms.\nAve. vote time is %v ms.\nAve. block size is %v.\nAve. round time is %v ms.\nLatency is %v ms.\n", realAveProposeTime, aveProposeTime, aveProcessTime, aveVoteProcessTime, aveBlockSize, aveRoundTime, latency)
	status := fmt.Sprintf(" Leader:%v\n Ave Real Time:%v\n. Ave. View Time: %vms\nAve. Propose Time: %vms\nAve. Dissemination Time: %vms, slow dissemination time: %v\nAve. Creation Time: %v, a proposal contains %v microblocks\nAve. Vote Time: %vms\nAve. Tx Rate: %v\nAve. MB Rate: %v, an MB contains %v txs\nRedundant microblocks:%v\nTotal microblocks: %v, Remaining microblocks: %v\nTotal missing microblocks: %v\nTotoal proposed microblocks:%v\nAve. hops:%v\nSend Rate: %v Mbps\nRecv Rate: %v Mbps\nTotal txs: %v, Remaining txs: %v\n, StableMb :%v, PendingMb : %v\n%s\n",
		r.GetCurrentLeader(), aveRealDissTime, aveRoundTime, aveProposeTime, aveDisseminationTime, aveSlowDisseminationTime, aveCreationTime, aveBlockSize, aveVoteTime, aveTxRate, mbRate, r.txNoInMB, r.totalRedundantMBs, r.sm.TotalMB(), r.sm.RemainingMB(), r.missingMicroblocks, r.totalProposedMBs, aveHops, r.SendRate(), r.RecvRate(), r.sm.TotalTx(), r.sm.RemainingTx(), r.sm.StableMB(), r.sm.PendingMB(), r.thrus)
	r.result = status
}

// 只有通过客户端发送交易时，才会走这个接口的逻辑，否则observerPool
func (r *Replica) handleTxn(m message.Transaction) {
	r.startSignal()
	log.Debugf("[%v] handleTxn ---  recivie tx TxID:[%v] ForwardNode:[%v] ", r.ID(), m.ID, m.NodeID)
	m.Timestamp = time.Now()
	isbuilt, mb := r.sm.AddTxn(&m)
	if isbuilt {
		log.Debugf("[%v] handleTxn --- built mb done, txs size %v", r.ID(), len(mb.Txns))
		r.txNoInMB = len(mb.Txns)
		mb.Sender = r.ID()
		r.sm.AddMicroblock(mb)
		mb.Timestamp = time.Now()
		r.totalMicroblocks++
		r.totalProposedMBs++
		if config.Configuration.LoadBalance == false {
			if r.isByz && config.Configuration.Strategy == "missing" {
				if config.Configuration.MemType == "naive" {
					r.Send(r.GetCurrentLeader(), mb)
				} else if config.Configuration.MemType == "ack" {
					r.MulticastQuorum(r.randomPick(), mb)

				}
			} else {
				if config.Configuration.BroadcastByGroup == true {
					groupId := mb.GroupId
					groupList := r.gm.GetGroupListByGroupId(groupId)
					log.Debugf("handleTxn() ---[%v] brocadcast mb [%x] to group [%v], group member list [%+v]", r.Node, mb.Hash, groupId, groupList)
					r.BroadcastByGroup(mb, groupList) //N -> 2f+1
				} else {
					r.Broadcast(mb)
				}
			}
		} else {
			mb.Hops++
			r.selfMBChan <- *mb
		}
	}
	r.kickOff()
}

// 后台监控交易池情况，如果交易池数量大于msize，生成一个mb，并广播
func (r *Replica) observePool() {
	payloadsize := config.GetConfig().PayloadSize
	msize := config.GetConfig().MSize
	nums := msize / payloadsize //一个微块包含多少个交易

	go func() {
		for {
			for i := 0; i < config.GetConfig().Mb_broadcast; i++ {
				r.mbBroadcast <- struct{}{}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	for {
		<-r.Pool.FetchSignal
		if r.Pool.TxLen() > nums {
			txs := r.Pool.FetchTx(nums)
			isbuilt, mb := r.sm.GenerateMb(txs)
			if isbuilt {
				//构建微块并且广播
				log.Debugf("ObservePool() --- [%v] built mb from pool, mb has %v txs, mb's hash[%x]", r.ID(), len(mb.Txns), mb.Hash)
				r.txNoInMB = len(mb.Txns)
				mb.Sender = r.ID()
				r.sm.AddMicroblock(mb)
				mb.Timestamp = time.Now()
				r.totalMicroblocks++
				r.totalProposedMBs++

				//限制广播
				<-r.mbBroadcast
				if config.Configuration.LoadBalance == false {
					if r.isByz && config.Configuration.Strategy == "missing" {
						if config.Configuration.MemType == "naive" {
							r.Send(r.GetCurrentLeader(), mb)
						} else if config.Configuration.MemType == "ack" {
							r.MulticastQuorum(r.randomPick(), mb)

						}
					} else {
						if config.Configuration.BroadcastByGroup == true {
							groupId := mb.GroupId
							groupList := r.gm.GetGroupListByGroupId(groupId)
							r.BroadcastByGroup(mb, groupList)
							log.Debugf("ObservePool() ---[%v] brocadcast mb [%x] to group [%v], group member list [%+v]", r.ID(), mb.Hash, groupId, groupList)
						} else if config.Configuration.BroadcastBySample == true {
							threshold := int(config.Configuration.Threshold)
							n := config.Configuration.N()
							//生成threshold个整数，每个数从1到n
							nodeNumber, _ := utils.GenerateUniqueRandomArray(threshold, 1, n)
							targetMember := make(map[identity.NodeID]struct{})
							targetMember[r.ID()] = struct{}{}
							for _, v := range nodeNumber {
								targetMember[identity.NewNodeID(v)] = struct{}{}
							}
							mb.GenerateNodeList = targetMember
							log.Debugf("ObservePool() ---[%v] brocadcast mb [%x] to sample [%v]. mb.GenerateNodeList[%v]", r.ID(), mb.Hash, targetMember, mb.GenerateNodeList)
							r.BroadcastByGroup(mb, targetMember)
						} else {
							log.Debugf("ObservePool() --- [%v] broadcastToAll mb's hash:[%x]", r.ID(), mb.Hash)
							r.Broadcast(mb)
						}
					}
				} else {
					mb.Hops++
					r.selfMBChan <- *mb
				}
			} else {
				log.Warningf("build mb failed")
			}
		}
	}

}

func (r *Replica) benchmark() {
	model := config.GetConfig().Model
	var wg sync.WaitGroup
	if model == "exp1" {
		//交易频繁冷启动
		ticker := time.NewTicker(5 * time.Second)
		timerForEnd := time.NewTimer(time.Duration(config.GetConfig().Time) * time.Second)
		wg.Add(1)
		go func() {
		DNOE:
			for {
				select {
				case <-ticker.C:
					log.Warningf("before add tx, len %v", r.Pool.TxLen())
					r.Pool.AddTx(10000)
					log.Warningf("before add tx, len %v", r.Pool.TxLen())
				case <-timerForEnd.C:
					break DNOE
				default:
					continue
				}
			}
			wg.Done()
		}()
	} else if model == "exp2" {
		//交易小规模持续到达
		ticker := time.NewTicker(1 * time.Second)
		timerForEnd := time.NewTimer(time.Duration(config.GetConfig().Time) * time.Second)
		wg.Add(1)
		go func() {
		DNOE:
			for {
				select {
				case <-ticker.C:
					log.Warningf("benchmark() --- [%v] has been added %v txs", r.ID(), config.GetConfig().TxPerSecond)
					r.Pool.AddTx(config.GetConfig().TxPerSecond)
				case <-timerForEnd.C:
					break DNOE
				default:
					continue
				}
			}
			wg.Done()
		}()
	} else if model == "exp3" {
		//大规模持续到达
		ticker := time.NewTicker(1 * time.Second)
		timerForEnd := time.NewTimer(time.Duration(config.GetConfig().Time) * time.Second)
		wg.Add(1)
		go func() {
		DNOE:
			for {
				select {
				case <-ticker.C:
					log.Warningf("add tx")
					r.Pool.AddTx(10000) //增加1w笔交易
				case <-timerForEnd.C:
					break DNOE
				default:
					continue
				}
			}
			wg.Done()
		}()
	} else if model == "exp4" {
		//压满交易池后开始共识
		ticker := time.NewTicker(3 * time.Second)
		timerForEnd := time.NewTimer(30 * time.Second)
		wg.Add(1)
		go func() {
		DNOE:
			for {
				select {
				case <-ticker.C:
					log.Warningf("add tx")
					r.Pool.AddTx(100000) //增加1w笔交易
				case <-timerForEnd.C:
					break DNOE
				default:
					continue
				}
			}
			wg.Done()
		}()
	} else if model == "onlyConsensus" {
		wg.Add(1)
		//测试场景，不发送交易
		for {
		}
	}

	wg.Wait()
	log.Resultf("exp stop")
}

func (r *Replica) kickOff() {
	// the first leader kicks off the protocol
	if r.pm.GetCurView() == 0 && r.IsLeader(r.ID(), 1) {
		log.Debugf("kickOff() --- [%v] ready to kick off the protocol", r.ID())
		log.Debugf("kickOff() --- [%v] is going to kick off the protocol", r.ID())
		r.pm.AdvanceView(0)
	}
}

func (r *Replica) loadbalance() {
	for {
		select {
		case mb := <-r.selfMBChan:
			if rand.Intn(100) < config.Configuration.ForwardP && r.ID().Node() <= config.Configuration.LoadedIndex {
				mb.IsForward = true
				pick := pickRandomNodes(config.Configuration.N()-1, 1, config.Configuration.LoadedIndex)[0]
				log.Debugf("[%v] is going to forward a mb to %v", r.ID(), pick)
				r.Send(pick, mb)
			} else {
				r.Broadcast(mb)
			}
			//log.Debugf("[%v] is going to gossip a self mb", r.ID())
			//r.MulticastQuorum(r.pickFanoutNodes(&mb), mb)
		default:
		}
	}
}

func (r *Replica) gossip() {
	for {
		tt := r.limiter.Take(int64(config.Configuration.Fanout))
		time.Sleep(tt)
	L:
		for {
			select {
			case mb := <-r.selfMBChan:
				//log.Debugf("[%v] is going to gossip a self mb", r.ID())
				r.MulticastQuorum(r.pickFanoutNodes(&mb), mb)
				break L
			default:
				select {
				case mb := <-r.selfMBChan:
					//log.Debugf("[%v] is going to gossip a self mb", r.ID())
					r.MulticastQuorum(r.pickFanoutNodes(&mb), mb)
					break L
				case mb := <-r.otherMBChan:
					if !r.sm.IsStable(mb.Hash) && mb.Hops <= config.Configuration.R {
						if r.ID().Node() > config.Configuration.SlowNo {
							r.MulticastQuorum(r.pickFanoutNodes(&mb), mb)
							break L
						} else if rand.Intn(100) < config.Configuration.P {
							r.MulticastQuorum(r.pickFanoutNodes(&mb), mb)
							break L
						} else {
							continue
						}
					} else {
						continue
					}
				}
			}
		}
	}
}

func (r *Replica) randomPick() []identity.NodeID {
	n := config.GetConfig().N() - 1 // exluding the master
	f := n/3 + 1
	pick := utils.RandomPick(n, f)
	pickedNode := make([]identity.NodeID, f)
	for i, item := range pick {
		pickedNode[i] = identity.NewNodeID(item + 2)
	}
	return pickedNode
}

// 按组广播
func (r *Replica) pickGroup() []identity.NodeID {
	//取出对应组的人进行广播
	n := config.GetConfig().N() - 1 // exluding the master
	f := 3
	pick := utils.RandomPick(n, f)
	pickedNode := make([]identity.NodeID, f)
	for i, item := range pick {
		pickedNode[i] = identity.NewNodeID(item + 2)
	}
	return pickedNode
}

func pickRandomNodes(n, d, index int) []identity.NodeID {
	pick := utils.RandomPick(n-index, d)
	pickedNode := make([]identity.NodeID, d)
	for i, item := range pick {
		pickedNode[i] = identity.NewNodeID(item + 1 + index)
	}
	return pickedNode
}

func (r *Replica) pickFanoutNodes(mb *blockchain.MicroBlock) []identity.NodeID {
	if !config.Configuration.Opt {
		return utils.PickRandomNodes(nil)
	}
	if bm, exists := r.mbSentNodes[mb.Hash]; exists {
		mb.AddSentNodes(utils.BitmapToNodes(bm))
	}
	mb.AddSentNodes(r.sm.AckList(mb.Hash))
	mb.AddSentNodes([]identity.NodeID{r.ID()})
	sentNodes := mb.FindSentNodes()
	nodes := utils.PickRandomNodes(sentNodes)
	mb.AddSentNodes(nodes)
	r.mbSentNodes[mb.Hash] = mb.Bitmap
	//mb.AddSentNodes(nodes)
	//log.Debugf("[%v] mb %x has received %v, is going to send to %v, %v hops", r.ID(), mb.Hash, sentNodes, nodes, mb.Hops)
	return nodes
}

/* Processors */

var lock sync.Mutex

func (r *Replica) processCommittedBlock(block *blockchain.Block) {
	lock.Lock()
	defer lock.Unlock()
	var txCount int
	deliver := make([]*blockchain.MicroBlock, 0)
	r.totalCommittedMBs += len(block.MicroblockList())
	for _, mb := range block.MicroblockList() {
		r.monitor.CollectCommitteddTime(mb.Hash, time.Now().Sub(mb.CreateTimeStamp))
		if _, exist := r.CommitedMb[mb.Hash]; exist {
			log.Debugf("processCommittedBlock() --- 提交了重复的区块%x", mb.Hash)
			continue
		}
		deliver = append(deliver, mb)
		r.CommitedMb[mb.Hash] = struct{}{}
		r.commitMbNo++
		mb.CommittedNo = r.commitMbNo //从1开始
		txCount += len(mb.Txns)
		for _, txn := range mb.Txns {
			// only record the delay of transactions from the local memory pool
			delay := time.Now().Sub(txn.Timestamp)
			r.totalDelay += delay
			r.latencyNo++
			r.totalCommittedTx++
			r.totalTx++
		}
		r.totalHops += mb.Hops
	}
	r.committedNo++
	log.Debugf("processCommittedBlock() --- [%v] the block is committed, No. of microblocks: %v, No. of tx: %v, view: %v, current view: %v, id: %x",
		r.ID(), len(block.MicroblockList()), txCount, block.View, r.pm.GetCurView(), block.ID)
	r.ex.MbReceive <- deliver //全部交付
}

func (r *Replica) processForkedBlock(block *blockchain.Block) {
	//if block.Proposer == r.Hash() {
	//	for _, txn := range block.payload {
	//		// collect txn back to mem pool
	//		//r.sm.CollectTxn(txn)
	//	}
	//}
	//log.Infof("[%v] the block is forked, No. of transactions: %v, view: %v, current view: %v, id: %x", r.ID(), len(block.payload), block.View, r.pm.GetCurView(), block.ID)
}

func (r *Replica) processNewView(newView types.View) {
	log.Debugf("processNewView() --- [%v] is processing new view: %v, leader is %v", r.ID(), newView, r.FindLeaderFor(newView))
	if !r.IsLeader(r.ID(), newView) {
		return
	}
	//r.proposeBlock(newView)
}

func (r *Replica) processAcks(ack *blockchain.Ack) {
	//if config.Configuration.MemType == "time" {
	//	r.estimator.AddAck(ack)
	if config.Configuration.MemType == "ack" {
		if r.sm.IsStable(ack.MicroblockID) {
			log.Debugf("processAcks() --- [%v] receive ack for a stabled mb: %x, return", r.ID(), ack.MicroblockID)
			return
		}
		if ack.Receiver != r.ID() {
			voteIsVerified, err := crypto.PubVerify(ack.Signature, crypto.IDToByte(ack.MicroblockID), ack.Receiver)
			if err != nil {
				log.Warningf("processAcks() --- [%v] Error in verifying the signature in ack id: %x", r.ID(), ack.MicroblockID)
				return
			}
			if !voteIsVerified {
				log.Warningf("processAcks() --- [%v] received an ack with invalid signature. vote id: %x", r.ID(), ack.MicroblockID)
				return
			}
		}
		r.sm.AddAck(ack)
		// found, _ := r.sm.FindMicroblock(ack.MicroblockID)
		// if !found && r.sm.IsStable(ack.MicroblockID) {
		// 	//没找到ack的微块，这应该是不可能的吧。。
		// 	missingRequest := message.MissingMBRequest{
		// 		RequesterID:   r.ID(),
		// 		MissingMBList: []crypto.Identifier{ack.MicroblockID},
		// 	}
		// 	r.Send(ack.Receiver, missingRequest)
		// 	log.Debugf("[%v] has received enough acks, but not received the microblock id: %x, fetch from %v",
		// 		r.ID(), ack.MicroblockID, ack.Receiver)
		// }
	}
}

func (r *Replica) proposeBlock(view types.View) {
	createStart := time.Now()
	//time.Sleep(time.Duration(config.Configuration.ProposeTime) * time.Millisecond)
	payload := r.sm.GeneratePayload()

	// if we are using time-based shared mempool, wait until all the microblocks are stable
	//if config.Configuration.MemType == "time" {
	//	r.waitUntilStable(payload)
	//}
	log.Debugf("proposeBlock() --- for debug, payload mb time list[%v]", payload.GenerateTimeList())
	proposal := r.Safety.MakeProposal(
		view,
		payload.GenerateHashList(),
		payload.GenerateGroupList(),
		payload.AckNode,
		payload.GenerateTimeList(),
		payload.TxNums,
	)
	log.Debugf("proposeBlock() --- [%v] make and broadcast a proposal for view %v, containing %v microblocks, %v stable mb left, proposal id [%x]",
		proposal.Proposer,
		proposal.View,
		len(proposal.HashList),
		r.sm.RemainingMB(),
		proposal.ID,
	)
	r.totalBlockSize += len(proposal.HashList)
	r.proposedNo++
	createEnd := time.Now()
	createDuration := createEnd.Sub(createStart)
	r.totalCreateDuration += createDuration
	proposal.Timestamp = time.Now()
	r.Broadcast(proposal)
	block := blockchain.BuildBlock(proposal, payload)
	_ = r.Safety.ProcessBlock(block)
	r.voteStart = time.Now()
}

func (r *Replica) verifySigs(sigMap map[crypto.Identifier]map[identity.NodeID]crypto.Signature) bool {
	for mbID, sigs := range sigMap {
		for id, sig := range sigs {
			voteIsVerified, err := crypto.PubVerify(sig, crypto.IDToByte(mbID), id)
			if err != nil {
				log.Warningf("[%v] Error in verifying the signature in ack id: %x", r.ID(), mbID)
				return false
			}
			if !voteIsVerified {
				log.Warningf("[%v] received an ack with invalid signature. vote id: %x", r.ID(), mbID)
				return false
			}
		}
	}
	return true
}

//func (r *Replica) waitUntilStable(payload *blockchain.Payload) {
//	lastItem := payload.LastItem()
//	if lastItem == nil {
//		return
//	}
//	stableTime := r.estimator.PredictStableTime("p")
//	//stableTime := time.Duration(0)
//	wait := lastItem.FutureTimestamp.Sub(time.Now()) - stableTime
//	//log.Debugf("[%v] stable time for a proposal is %v", r.ID(), stableTime)
//	if stableTime < 0 {
//		log.Errorf("[%v] stable time for proposal is less than 0")
//	}
//	if wait > 0 {
//		log.Debugf("[%v] wait for %v until the contained microblocks are stable", r.ID(), wait)
//		time.Sleep(wait)
//	}
//}

func (r *Replica) GetCurrentLeader() identity.NodeID {
	return r.Election.FindLeaderFor(r.pm.GetCurView())
}

// ListenLocalEvent listens new view and timeout events
func (r *Replica) ListenLocalEvent() {
	r.lastViewTime = time.Now()
	r.timer = time.NewTimer(r.pm.GetTimerForView())
	for {
		r.timer.Reset(r.pm.GetTimerForView())
	L:
		for {
			select {
			case view := <-r.pm.EnteringViewEvent():
				if view >= 2 {
					//r.totalVoteTime += time.Now().Sub(r.voteStart)
				}
				// measure round time
				now := time.Now()
				lasts := now.Sub(r.lastViewTime)
				r.totalRoundTime += lasts
				r.roundNo++
				r.lastViewTime = now
				r.eventChan <- view
				log.Debugf("ListenLocalEvent --- [%v] the last view lasts %v milliseconds, current view: %v", r.ID(), lasts.Milliseconds(), view)
				break L
			case <-r.timer.C:
				r.Safety.ProcessLocalTmo(r.pm.GetCurView())
				break L
			}
		}
	}
}

// ListenCommittedBlocks listens committed blocks and forked blocks from the protocols
func (r *Replica) ListenCommittedBlocks() {
	for {
		select {
		case committedBlock := <-r.committedBlocks:
			r.processCommittedBlock(committedBlock)
		case forkedBlock := <-r.forkedBlocks:
			r.processForkedBlock(forkedBlock)
		}
	}
}

//用来监控节点的各项指标 duration：持续的时间  interval ： 每隔多少ms进行一次结果采样

func (r *Replica) startMonitor(duration time.Duration, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	r.monitor.Duration = duration
	r.monitor.Interval = interval
	endTime := time.Now().Add(duration) // 计算结束时间
	log.Infof("startMonitor() --- Monitoring started. Duration: %v s, Interval: %v s", duration.Seconds(), interval.Seconds())

	for now := time.Now(); now.Before(endTime); now = time.Now() {
		select {
		case <-ticker.C:
			//监控指标
			r.collectData()
		}
	}

	log.Infof("startMonitor() --- Monitoring finished")

	var filePath string
	nodesNum := fmt.Sprint(config.GetConfig().N())               //节点数
	threshold := fmt.Sprint(int(config.Configuration.Threshold)) //2f+1
	txPerSecond := fmt.Sprint(config.Configuration.TxPerSecond)  //每秒交易
	id := fmt.Sprint(r.ID())                                     //节点id
	groupNum := fmt.Sprint(config.Configuration.GroupNum)        //分组数
	payloadSize := fmt.Sprint(config.Configuration.PayloadSize)  //交易大侠
	benchTime := fmt.Sprint(config.Configuration.Time)

	now := time.Now()
	timestamp := fmt.Sprintf("%v", now.Format("20060102_150405"))
	//TODO:hotstuff的文件保存路径

	err := os.MkdirAll("./result", os.ModePerm)
	if err != nil {
		log.Fatalf("failed to create directory: %v", err)
	}

	if config.Configuration.BroadcastByGroup {
		filePath = "./result/ID" + id + "-N" + nodesNum + "-Threshold" + threshold + "-Tx" + txPerSecond + "-txSize" + payloadSize + "-BenchTime" + benchTime + "-Gnum" + groupNum + "-group" + "-duration" + duration.String() + "-interval" + interval.String() + "-" + timestamp + ".json"
	} else if config.Configuration.BroadcastBySample {
		filePath = "./result/ID" + id + "-N" + nodesNum + "-Threshold" + threshold + "-Tx" + txPerSecond + "-txSize" + payloadSize + "-BenchTime" + benchTime + "-sample" + "-duration" + duration.String() + "-interval" + interval.String() + "-" + timestamp + ".json"
	} else {
		filePath = "./result/ID" + id + "-N" + nodesNum + "-Threshold" + threshold + "-Tx" + txPerSecond + "-txSize" + payloadSize + "-BenchTime" + benchTime + "-stratus" + "-duration" + duration.String() + "-interval" + interval.String() + "-" + timestamp + ".json"
	}
	r.monitor.SaveResult(filePath)
}

var collectMu sync.Mutex

/**
1. 每秒stable的微块数
2. 每个微块被stable的用时 k，v
3. 每秒执行成功的微块数
4. 每个微块被执行成功的用时 k，v
5. 每秒已经接收的交易数 （到达的，包含已经被取出的）
6. 每秒交易池中剩余的交易
7. 每秒被共识提交的微块数
8. 每个微块被共识提交的用时 k，v


*/

func (r *Replica) collectData() {
	collectMu.Lock()
	defer collectMu.Unlock()
	r.monitor.CollectStableNum(r.sm.TotalStableMb())
	r.monitor.CollectCommittedNum(r.getTotalCommittedBlock())
	r.monitor.CollectExecutedNum(r.ex.TotalNum())
	r.monitor.CollectReceiveTxNum(r.Pool.ReceiveNum())
	r.monitor.CollectePoolTxNum(r.Pool.TxLen())

	r.monitor.CollectStableTPS()
	r.monitor.CollectStableTPSFromBegin()
	r.monitor.CollectStableDelay()

	r.monitor.CollectCommittedTPS()
	r.monitor.CollectCommittedTPSFromBegin()
	r.monitor.CollectCommittedDelay()

	r.monitor.CollectExecutedTPS()
	r.monitor.CollectExecutedTPSFromBegin()
	r.monitor.CollectExecutedDelay()

	log.Infof("collectData() --- [%v] 已收集[%v]的数据", r.ID(), time.Now())
	log.Debugf("collectData() --- [%v]: stableNUM:[%v],committedNum:[%v],executedNum:[%v],receiveTxNum:[%v],poolTxNum:[%v]",
		r.ID(),
		r.monitor.GetStableNumList(),
		r.monitor.GetCommittedNumList(),
		r.monitor.GetExecutedNumList(),
		r.monitor.GetReceiveTxNumList(),
		r.monitor.GetPoolTxNumList(),
	)
}

func (r *Replica) getTotalCommittedBlock() int {
	lock.Lock()
	defer lock.Unlock()
	return r.totalCommittedMBs
}

func (r *Replica) startSignal() {
	if !r.isStarted.Load() {
		r.startTime = time.Now()
		r.tmpTime = time.Now()
		log.Debugf("startSignal() --- [%v] is boosting", r.ID())
		r.isStarted.Store(true)
		r.start <- true
	}
}

// Start starts event loop
func (r *Replica) Start() {
	waitSecond := 15
	log.Infof("Start() --- [%v] wait other nodes start for %v", r.ID(), waitSecond)
	time.Sleep(time.Duration(waitSecond) * time.Second)
	go func() {
		http.ListenAndServe("localhost:6060", nil) // 启动 pprof 服务器
	}()

	go r.Run()
	//go r.gossip()
	go r.loadbalance() //负载均衡用

	//交易执行器
	go r.ex.HandleMB()
	//模拟交易
	go r.benchmark()
	go r.observePool()

	// wait for the start signal
	<-r.start
	log.Infof("Start() --- [%v] start", r.Node.ID())
	go r.ListenLocalEvent()
	go r.ListenCommittedBlocks()

	duration := time.Duration(config.GetConfig().Duration) //监控时间 单位ms
	interval := time.Duration(config.GetConfig().Interval) //监控频率 单位ms
	go r.startMonitor(duration*time.Millisecond, interval*time.Millisecond)

	for r.isStarted.Load() {
		event := <-r.eventChan
		switch v := event.(type) {
		case types.View:
			r.processNewView(v)
		case blockchain.Block:
			startProcessTime := time.Now()
			_ = r.Safety.ProcessBlock(&v)
			r.totalProcessDuration += time.Now().Sub(startProcessTime)
			r.voteStart = time.Now()
			r.processedNo++
		case blockchain.Vote:
			//startProcessTime := time.Now()
			r.Safety.ProcessVote(&v)
			//processingDuration := time.Now().Sub(startProcessTime)
			//r.totalVoteTime += processingDuration
			//r.voteNo++
		case pacemaker.TMO:
			r.Safety.ProcessRemoteTmo(&v)
		case execute.ExecuteResult:
			//收到执行结果
			r.ex.ReceiveResult <- &v
		default:
			log.Errorf("[%v] received an unknown event %v", r.ID(), v)
		}
	}
}
