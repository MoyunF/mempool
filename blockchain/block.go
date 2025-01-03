package blockchain

import (
	"strconv"
	"time"

	"github.com/gitferry/bamboo/crypto/merkle"
	"github.com/gitferry/bamboo/group"
	"github.com/kelindar/bitmap"

	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/types"
)

type BlockHeader struct {
	types.View
	QC        *QC
	Proposer  identity.NodeID
	Timestamp time.Time
	PrevID    crypto.Identifier
	Sig       crypto.Signature
	ID        crypto.Identifier
	Ts        time.Duration
}

type Block struct {
	BlockHeader
	payload *Payload
}

type Payload struct {
	MicroblockList []*MicroBlock
	SigMap         map[crypto.Identifier]map[identity.NodeID]crypto.Signature
	AckNode        []map[identity.NodeID]struct{} //接收的节点
	TxNums         []int
}

type MicroBlock struct {
	ProposalID      crypto.Identifier
	GroupId         int //add by lxx 代表mb要被发送的执行组是什么
	Hash            crypto.Identifier
	Txns            []*message.Transaction
	Timestamp       time.Time
	FutureTimestamp time.Time
	Sender          identity.NodeID
	IsFake          bool //是否只是个空壳，还没有收到实际内容
	IsRequested     bool
	IsForward       bool
	Bitmap          bitmap.Bitmap
	Hops            int
	CommittedNo     int

	//完全随机
	GenerateNodeList map[identity.NodeID]struct{}

	CreateTimeStamp      time.Time //小块被创建的时间
	SendTimeStamp        time.Time //小块被发送的时间
	ReceiveTimeStamp     time.Time //小块被收到的时间
	CommittedTimeStamp   time.Time //小块被hotstuff提交的时间
	ExecutStratTimeStamp time.Time //小块开始执行的时间
	ExecuteEndTimeStamp  time.Time //小块执行结束的时间

	RouteList []*types.Route //路由的列表
}

type Proposal struct {
	BlockHeader
	HashList  []crypto.Identifier
	TxNums    []int //每个微块中的交易数量，传这个信息是为了方便统计每个提案中有多少个交易
	GroupList []int
	AckNode   []map[identity.NodeID]struct{}
	MbTime    []time.Time
	MbList    []*MicroBlock //hotstuff中直接传小块明文
}

type PendingBlock struct {
	Payload    *Payload // microblocks that already exist
	Proposal   *Proposal
	MissingMap map[crypto.Identifier]struct{} // missing list
}

type rawProposal struct {
	types.View
	QC       *QC
	Proposer identity.NodeID
	Payload  []crypto.Identifier
	PrevID   crypto.Identifier
}

// BuildProposal creates a signed proposal
func BuildProposal(view types.View, qc *QC, prevID crypto.Identifier, payload []crypto.Identifier,
	groupList []int, ackNodeList []map[identity.NodeID]struct{},
	mbTime []time.Time, proposer identity.NodeID, txNums []int, mblist []*MicroBlock,
) *Proposal {
	p := new(Proposal)
	p.View = view
	p.Proposer = proposer
	p.QC = qc
	p.HashList = payload
	p.PrevID = prevID
	p.GroupList = groupList
	p.AckNode = ackNodeList
	p.MbTime = mbTime
	p.makeID(proposer)
	p.TxNums = txNums
	p.MbList = mblist
	return p
}

func NewPayload(microblockList []*MicroBlock, sigs map[crypto.Identifier]map[identity.NodeID]crypto.Signature, ackList []map[identity.NodeID]struct{}, txNums []int) *Payload {
	return &Payload{
		MicroblockList: microblockList,
		SigMap:         sigs,
		AckNode:        ackList,
		TxNums:         txNums,
	}
}

func (b *Block) MicroblockList() []*MicroBlock {
	return b.payload.MicroblockList
}

func (pl *Payload) GenerateHashList() []crypto.Identifier {
	hashList := make([]crypto.Identifier, 0)
	for _, mb := range pl.MicroblockList {
		if mb == nil {
			continue
		}
		hashList = append(hashList, mb.Hash)
	}
	return hashList
}

func (pl *Payload) GenerateGroupList() []int {
	groupList := make([]int, 0)
	for _, mb := range pl.MicroblockList {
		if mb == nil {
			continue
		}
		groupList = append(groupList, mb.GroupId)
	}
	return groupList
}

func (pl *Payload) GenerateTimeList() []time.Time {
	timeList := make([]time.Time, 0)
	for _, mb := range pl.MicroblockList {
		if mb == nil {
			continue
		}
		timeList = append(timeList, mb.Timestamp)
	}
	return timeList
}

func (pl *Payload) addMicroblock(mb *MicroBlock) {
	pl.MicroblockList = append(pl.MicroblockList, mb)
}

func (pl *Payload) LastItem() *MicroBlock {
	if len(pl.MicroblockList) == 0 {
		return nil
	}
	return pl.MicroblockList[len(pl.MicroblockList)-1]
}

func (mb *MicroBlock) FindSentNodes() []identity.NodeID {
	nodes := make([]identity.NodeID, 0)
	mb.Bitmap.Range(func(x uint32) {
		nodes = append(nodes, identity.NodeID(strconv.Itoa(int(x))))
		return
	})

	return nodes
}

func (mb *MicroBlock) AddSentNodes(nodes []identity.NodeID) {
	for _, id := range nodes {
		mb.Bitmap.Set(uint32(id.Node()))
	}
}

// BuildBlock fills microblocks to make a block,
func BuildBlock(proposal *Proposal, payload *Payload) *Block {
	return &Block{
		BlockHeader: proposal.BlockHeader,
		payload:     payload,
	}
}

// // 构建区块，包括一些没有收到的微块
// func BuildBlockWithPending(proposal *Proposal, payload *Payload) *Block {

// 	return &Block{
// 		BlockHeader: proposal.BlockHeader,
// 		payload:     payload,
// 	}
// }

func NewMicroblock(proposalID crypto.Identifier, txnList []*message.Transaction) *MicroBlock {
	mb := new(MicroBlock)
	mb.ProposalID = proposalID
	mb.Txns = txnList
	mb.Timestamp = time.Now()
	mb.CreateTimeStamp = time.Now()
	mb.Hash = mb.hash()                        //根据交易生成Hash，但是好像不会验证hhhh
	mb.GroupId = group.GenerateGroupIdByRand() //为mb增加group id
	mb.RouteList = make([]*types.Route, 0, 10)
	return mb
}

func NewPendingBlock(proposal *Proposal, missingMap map[crypto.Identifier]struct{}, microBlocks []*MicroBlock) *PendingBlock {
	return &PendingBlock{
		Proposal:   proposal,
		MissingMap: missingMap,
		Payload:    &Payload{MicroblockList: microBlocks},
	}
}

func (p *Proposal) makeID(nodeID identity.NodeID) {
	raw := &rawProposal{
		View:     p.View,
		QC:       p.QC,
		Proposer: p.Proposer,
		Payload:  p.HashList,
		PrevID:   p.PrevID,
	}
	p.ID = crypto.MakeID(raw)
	p.Sig, _ = crypto.PrivSign(crypto.IDToByte(p.ID), nodeID, nil)
}

func (mb *MicroBlock) hash() crypto.Identifier {
	hashList := make([][]byte, 0)
	for _, tx := range mb.Txns {
		hashList = append(hashList, crypto.IDToByte(crypto.MakeID(tx)))
	}
	hashList = append(hashList, []byte(mb.Timestamp.String()))
	return crypto.MakeID(merkle.HashFromByteSlices(hashList))
}

func (pd *PendingBlock) AddMicroblock(mb *MicroBlock) *Block {
	_, exists := pd.MissingMap[mb.Hash]
	if exists {
		pd.Payload.addMicroblock(mb)
		delete(pd.MissingMap, mb.Hash)
	}
	if len(pd.MissingMap) == 0 {
		return BuildBlock(pd.Proposal, pd.Payload)
	}
	return nil
}

func (pd *PendingBlock) CompleteBlock() *Block {
	return BuildBlock(pd.Proposal, pd.Payload)
}

func (pd *PendingBlock) MissingCount() int {
	return len(pd.MissingMap)
}

func (pd *PendingBlock) MissingMBList() []crypto.Identifier {
	missingList := make([]crypto.Identifier, 0)
	for k := range pd.MissingMap {
		missingList = append(missingList, k)
	}
	return missingList
}
