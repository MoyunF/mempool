package mempool

import (
	"container/list"
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
	"github.com/gitferry/bamboo/utils"
)

type AckMem struct {
	stableMicroblocks  *list.List
	txnList            *list.List
	microblockMap      map[crypto.Identifier]*blockchain.MicroBlock
	pendingMicroblocks map[crypto.Identifier]*PendingMicroblock
	ackBuffer          map[crypto.Identifier]map[identity.NodeID]crypto.Signature //number of the ack received before mb arrived
	stableMBs          map[crypto.Identifier]struct{}                             //keeps track of stable microblocks
	StableBuffer       map[crypto.Identifier]blockchain.Stable                    //保存mb的stable信息
	bsize              int                                                        // number of microblocks in a proposal
	msize              int                                                        // byte size of transactions in a microblock
	memsize            int                                                        // number of microblocks in mempool
	currSize           int
	threshhold         int // number of acks needed for a stable microblock
	totalTx            int64
	TotalStableMbs     int           //截至目前一共stable的微块个数
	TotalStableDelay   time.Duration //目前一共使用的时间
	mu                 sync.Mutex
	gm                 *group.GroupManager
	node               node.Node
}

type PendingMicroblock struct {
	microblock  *blockchain.MicroBlock
	ackMap      map[identity.NodeID]struct{} // who has sent acks
	ackNum      int
	AckInGroup  []identity.NodeID //组内ack
	AckOutGroup []identity.NodeID //组外ack
	//增加ack来自哪些节点
}

// NewAckMem creates a new naive mempool
func NewAckMem(n node.Node, gm *group.GroupManager) *AckMem {
	ack := &AckMem{
		bsize:              config.GetConfig().BSize,
		msize:              config.GetConfig().MSize,
		memsize:            config.GetConfig().MemSize,
		threshhold:         config.GetConfig().Q,
		stableMicroblocks:  list.New(),
		microblockMap:      make(map[crypto.Identifier]*blockchain.MicroBlock),
		pendingMicroblocks: make(map[crypto.Identifier]*PendingMicroblock),
		ackBuffer:          make(map[crypto.Identifier]map[identity.NodeID]crypto.Signature),
		stableMBs:          make(map[crypto.Identifier]struct{}),
		StableBuffer:       make(map[crypto.Identifier]blockchain.Stable),
		currSize:           0,
		txnList:            list.New(),
		gm:                 gm,
		node:               n,
	}
	return ack
}

// AddTxn adds a transaction and returns a microblock if msize is reached
// then the contained transactions should be deleted
func (am *AckMem) AddTxn(txn *message.Transaction) (bool, *blockchain.MicroBlock) {
	// mempool is full
	if am.RemainingTx() >= int64(am.memsize) {
		log.Warningf("mempool's tx list is full")
		return false, nil
	}
	if am.RemainingMB() >= int64(am.memsize) {
		log.Warningf("mempool's mb is full")
		return false, nil
	}
	am.totalTx++

	// get the size of the structure. txn is the pointer.
	tranSize := utils.SizeOf(txn)
	totalSize := tranSize + am.currSize

	if tranSize > am.msize {
		return false, nil
	}

	if totalSize > am.msize {
		//do not add the curr trans, and generate a microBlock
		//set the currSize to curr trans, since it is the only one does not add to the microblock
		var id crypto.Identifier
		am.currSize = tranSize
		log.Debugf("totalSize :%v > am.msize:%v generate mb", totalSize, am.msize)
		newBlock := blockchain.NewMicroblock(id, am.makeTxnSlice())
		am.txnList.PushBack(txn)
		return true, newBlock

	} else if totalSize == am.msize {
		//add the curr trans, and generate a microBlock
		var id crypto.Identifier
		allTxn := append(am.makeTxnSlice(), txn)
		am.currSize = 0
		return true, blockchain.NewMicroblock(id, allTxn)

	} else {
		//
		am.txnList.PushBack(txn)
		am.currSize = totalSize
		return false, nil
	}
}

//生成微块
func (am *AckMem) GenerateMb(txs []*message.Transaction) (bool, *blockchain.MicroBlock) {
	if am.RemainingMB() >= int64(am.memsize) {
		log.Warningf("mempool's mb is full")
		return false, nil
	}
	var id crypto.Identifier
	return true, blockchain.NewMicroblock(id, txs)
}

// AddMicroblock adds a microblock into a FIFO queue
// return an err if the queue is full (memsize)
func (am *AckMem) AddMicroblock(mb *blockchain.MicroBlock) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	//if am.microblocks.Len() >= am.memsize {
	//	return errors.New("the memory queue is full")
	//}
	_, exists := am.microblockMap[mb.Hash]
	if exists {
		return nil
	}
	//pm容器，用来判断谁给这个微块发送了ack
	pm := &PendingMicroblock{
		microblock:  mb,
		ackMap:      make(map[identity.NodeID]struct{}),
		ackNum:      0,
		AckInGroup:  make([]identity.NodeID, 0),
		AckOutGroup: make([]identity.NodeID, 0),
	}
	pm.ackMap[mb.Sender] = struct{}{}
	if am.gm.IsInGroup(mb.GroupId, am.node.ID()) {
		//如果微块是自己组内的
		pm.ackNum++
		pm.AckInGroup = append(pm.AckInGroup, mb.Sender)
	} else {
		pm.AckOutGroup = append(pm.AckOutGroup, mb.Sender)
	}
	am.microblockMap[mb.Hash] = mb
	log.Debugf("收到了mb：%v ,mb's fake : %v", mb.Hash, mb.IsFake)

	//check if there are some acks of this microblock arrived before
	buffer, received := am.ackBuffer[mb.Hash]
	if received {
		// if so, add these ack to the pendingblocks
		for id, _ := range buffer {
			//am.pendingMicroblocks[mb.Hash].ackMap[ack] = struct{}{}
			pm.ackMap[id] = struct{}{}
			pm.ackNum++
			pm.AckInGroup = append(pm.AckInGroup, id)
		}
		if pm.ackNum >= am.threshhold {
			if _, exists = am.stableMBs[mb.Hash]; !exists {
				am.stableMicroblocks.PushBack(mb)
				am.stableMBs[mb.Hash] = struct{}{}
				am.TotalStableMbs++
				am.TotalStableDelay += time.Now().Sub(mb.Timestamp)
				delete(am.pendingMicroblocks, mb.Hash)
				log.Debugf("microblock id: %x becomes stable from buffer", mb.Hash)
			}
		} else {
			am.pendingMicroblocks[mb.Hash] = pm
		}
	} else {
		am.pendingMicroblocks[mb.Hash] = pm
	}
	return nil
}

// AddAck adds an ack and push a microblock into the stableMicroblocks queue if it receives enough acks
func (am *AckMem) AddAck(ack *blockchain.Ack) {
	am.mu.Lock()
	defer am.mu.Unlock()
	log.Debugf("receive ack for mb id:%v", ack.MicroblockID)
	target, received := am.pendingMicroblocks[ack.MicroblockID]
	//check if the ack arrives before the microblock
	if received {
		if config.GetConfig().BroadcastByGroup == true {
			//分组
			if !ack.OutGroup {
				log.Debugf("收到组内节点%v的ack", ack.Receiver)
				target.ackMap[ack.Receiver] = struct{}{}
				target.ackNum++
				target.AckInGroup = append(target.AckInGroup, ack.Receiver)
				log.Debugf("ack nums is %v", target.ackNum)
			} else {
				log.Debugf("收到组外节点%v的ack", ack.Receiver)
				target.ackMap[ack.Receiver] = struct{}{}
				target.AckOutGroup = append(target.AckOutGroup, ack.Receiver)
			}
		} else {
			//不分组
			target.ackMap[ack.Receiver] = struct{}{}
			target.ackNum++
			log.Debugf("ack nums is %v", target.ackNum)
			target.AckInGroup = append(target.AckInGroup, ack.Receiver)
		}
		if target.ackNum >= am.threshhold {
			if _, exists := am.stableMBs[target.microblock.Hash]; !exists {
				am.stableMicroblocks.PushBack(target.microblock)
				am.stableMBs[target.microblock.Hash] = struct{}{}
				am.TotalStableMbs++
				am.TotalStableDelay += time.Now().Sub(target.microblock.Timestamp)
				//log.Debugf("push a stableMb id:%v", target.microblock)
				delete(am.pendingMicroblocks, ack.MicroblockID)
				//构建stable信息
				stable := blockchain.Stable{
					AckInGroup:     make([]identity.NodeID, len(target.AckInGroup)),
					AckOutGroup:    make([]identity.NodeID, len(target.AckOutGroup)),
					MicroblockID:   target.microblock.Hash,
					Sender:         am.node.ID(),
					GroupId:        target.microblock.GroupId,
					MbCreationTime: target.microblock.Timestamp,
				}
				am.StableBuffer[target.microblock.Hash] = stable //保存stable信息
				copy(stable.AckInGroup, target.AckInGroup)
				copy(stable.AckOutGroup, target.AckOutGroup)
				log.Debugf("stable准备广播: %+v", stable)
				am.node.Broadcast(stable)
			}
		}
	} else {
		//ack arrives before microblock, record the number of ack received before microblock
		//let the addMicrobslock do the rest.
		if ack.OutGroup != true {
			//组内的回复才奏效
			log.Debugf("receive ack for mb : %v before mb", ack.MicroblockID)
			_, exist := am.ackBuffer[ack.MicroblockID]
			if exist {
				am.ackBuffer[ack.MicroblockID][ack.Receiver] = ack.Signature
			} else {
				temp := make(map[identity.NodeID]crypto.Signature, 0)
				temp[ack.Receiver] = ack.Signature
				am.ackBuffer[ack.MicroblockID] = temp
			}
		}
	}
}

func (am *AckMem) AddStable(stable *blockchain.Stable) {
	am.mu.Lock()
	defer am.mu.Unlock()
	log.Debugf("receive stable for mb's hash:%v from:%v", stable.MicroblockID, stable.Sender)
	if _, ok := am.stableMBs[stable.MicroblockID]; ok {
		//如果自己已经确认过了
		log.Debugf("has been stabel by itself")
		return
	}
	//保存stable信息
	am.StableBuffer[stable.MicroblockID] = *stable

	target, received := am.pendingMicroblocks[stable.MicroblockID]
	//check if the stable arrives before the microblock
	if received {
		am.stableMicroblocks.PushBack(target.microblock)
		am.stableMBs[target.microblock.Hash] = struct{}{}
		am.TotalStableMbs++
		am.TotalStableDelay += time.Now().Sub(target.microblock.Timestamp)

		log.Debugf("receive stale from %v", stable.Sender)
		delete(am.pendingMicroblocks, stable.MicroblockID)
	}
	_, exist := am.microblockMap[stable.MicroblockID]
	if !exist {
		//收到了stable但是没有收到微块
		if am.gm.IsInGroup(stable.GroupId, am.node.ID()) {
			log.Debugf("收到了一个自己组的stable，但是没有对应的微块")
		} else {
			log.Debugf("收到了其他组的stable，所以构建一个fake块")
		}
		mb := &blockchain.MicroBlock{
			IsFake:    true,
			Hash:      stable.MicroblockID,
			GroupId:   stable.GroupId,
			Timestamp: stable.MbCreationTime,
		}
		am.microblockMap[mb.Hash] = mb
		am.stableMicroblocks.PushBack(mb)
		am.stableMBs[mb.Hash] = struct{}{}

		am.TotalStableMbs++
		am.TotalStableDelay += time.Now().Sub(mb.Timestamp)

		am.microblockMap[mb.Hash] = mb //变量逃逸
	}
}

func (am *AckMem) HandleMissingStableMb(mb *blockchain.MicroBlock) {
	am.mu.Lock()
	defer am.mu.Unlock()
	log.Debugf("内存池收到丢失的stable块")
	am.microblockMap[mb.Hash] = mb
}

// GeneratePayload generates a list of microblocks according to bsize
// if the remaining microblocks is less than bsize then return all
func (am *AckMem) GeneratePayload() *blockchain.Payload {
	var batchSize int
	am.mu.Lock()
	defer am.mu.Unlock()
	if config.Configuration.Onlyworker == true {
		for {
			time.Sleep(100 * time.Millisecond)
		}
	}
	sigMap := make(map[crypto.Identifier]map[identity.NodeID]crypto.Signature, 0)
	log.Debugf("generatePayload,stable mb is %v", am.stableMicroblocks.Len())
	if am.stableMicroblocks.Len() >= am.bsize {
		batchSize = am.bsize
	} else {
		// if config.GetConfig().BroadcastByGroup == true {
		// 	time.Sleep(200 * time.Millisecond)
		// }
		batchSize = am.stableMicroblocks.Len()
	}
	// for {
	// 	if am.stableMicroblocks.Len() >= am.bsize/2 {
	// 		batchSize = am.bsize
	// 		break
	// 	}
	// }
	microblockList := make([]*blockchain.MicroBlock, 0)
	ackNodeList := make([]map[identity.NodeID]struct{}, 0)

	for i := 0; i < batchSize; i++ {
		mb := am.front()
		if mb == nil {
			break
		}
		log.Debugf("microblock id: %x is deleted from mempool when proposing", mb.Hash)
		microblockList = append(microblockList, mb)
		ackNodeList = append(ackNodeList, am.GenerateAckNodeList(mb))

		sigs := make(map[identity.NodeID]crypto.Signature, 0)
		count := 0
		for id, sig := range am.ackBuffer[mb.Hash] {
			count++
			sigs[id] = sig
			if count == config.Configuration.Q {
				break
			}
		}
		sigMap[mb.Hash] = sigs
	}
	log.Debugf("generate payload, len: %v", len(microblockList))
	return blockchain.NewPayload(microblockList, sigMap, ackNodeList)
}

func (am *AckMem) GenerateAckNodeList(mb *blockchain.MicroBlock) map[identity.NodeID]struct{} {
	stable, ok := am.StableBuffer[mb.Hash]
	result := make(map[identity.NodeID]struct{}, 0)
	if ok {
		for _, v := range stable.AckInGroup {
			result[v] = struct{}{}
		}
	} else {
		log.Debugf("没找到stable信息")
	}
	return result
}

// CheckExistence checks if the referred microblocks in the proposal exists
// in the mempool and return missing ones if there's any
// return true if there's no missing transactions
func (am *AckMem) CheckExistence(p *blockchain.Proposal) (bool, []crypto.Identifier) {
	id := make([]crypto.Identifier, 0)
	return false, id
}

// RemoveMicroblock removes reffered microblocks from the mempool
func (am *AckMem) RemoveMicroblock(id crypto.Identifier) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	_, exists := am.microblockMap[id]
	if exists {
		delete(am.microblockMap, id)
	}
	_, exists = am.stableMBs[id]
	if exists {
		delete(am.stableMBs, id)
	}
	return nil
}

// FindMicroblock finds a reffered microblock
func (am *AckMem) FindMicroblock(id crypto.Identifier) (bool, *blockchain.MicroBlock) {
	am.mu.Lock()
	defer am.mu.Unlock()
	mb, found := am.microblockMap[id]
	if found {
		if mb.IsFake == false {
			return false, nil
		}
	}
	return found, mb
}

// FillProposal pulls microblocks from the mempool and build a pending block,
// a pending block should include the proposal, micorblocks that already exist,
// and a missing list if there's any
//接受Proposal后，会通过fillProposal来获取丢失的微块
func (am *AckMem) FillProposal(p *blockchain.Proposal) *blockchain.PendingBlock {
	am.mu.Lock()
	defer am.mu.Unlock()
	existingBlocks := make([]*blockchain.MicroBlock, 0)
	missingBlocks := make(map[crypto.Identifier]struct{}, 0)
	for _, id := range p.HashList {
		found := false
		_, exists := am.pendingMicroblocks[id]
		if exists {
			found = true
			existingBlocks = append(existingBlocks, am.pendingMicroblocks[id].microblock)
			delete(am.pendingMicroblocks, id)
			//log.Debugf("microblock id: %x is deleted from pending when filling", id)
		}
		for e := am.stableMicroblocks.Front(); e != nil; e = e.Next() {
			// do something with e.Value
			mb := e.Value.(*blockchain.MicroBlock)
			if mb.Hash == id {
				existingBlocks = append(existingBlocks, mb)
				found = true
				am.stableMicroblocks.Remove(e)
				//log.Debugf("microblock id: %x is deleted from stable when filling", mb.Hash)
				break
			}
		}

		if !found {
			missingBlocks[id] = struct{}{}
		}
	}
	return blockchain.NewPendingBlock(p, missingBlocks, existingBlocks)

}

func (am *AckMem) FetchMB(p *blockchain.Proposal) *blockchain.PendingBlock {
	am.mu.Lock()
	defer am.mu.Unlock()
	existingBlocks := make([]*blockchain.MicroBlock, 0)
	missingBlocks := make(map[crypto.Identifier]struct{}, 0)
	for index, id := range p.HashList {
		//把相关额pending和stable都删掉
		_, exists := am.pendingMicroblocks[id]
		if exists {
			delete(am.pendingMicroblocks, id)
			log.Debugf("microblock id: %x is deleted from pending when filling", id)
		}
		for e := am.stableMicroblocks.Front(); e != nil; e = e.Next() {
			// do something with e.Value
			mb := e.Value.(*blockchain.MicroBlock)
			if mb.Hash == id {
				am.stableMicroblocks.Remove(e)
				log.Debugf("microblock id: %x is deleted from stable when filling", mb.Hash)
				break
			}
		}

		mb, ok := am.microblockMap[id]
		if ok {
			log.Debugf("FetchMb，在本地找到%v，mb's fake:%v", id, mb.IsFake)
			existingBlocks = append(existingBlocks, mb)
		} else {
			log.Debugf("Porposal中的mb id: %x 我没有", id)
			mb := &blockchain.MicroBlock{
				IsFake:    true,
				Hash:      id,
				GroupId:   p.GroupList[index],
				Timestamp: p.MbTime[index],
			}
			existingBlocks = append(existingBlocks, mb)
		}
	}
	return blockchain.NewPendingBlock(p, missingBlocks, existingBlocks)
}

// FillProposal pulls microblocks from the mempool and build a pending block,
// a pending block should include the proposal, micorblocks that already exist,
// and a missing list if there's any
//payload包含收到的和没收到的mb
func (am *AckMem) FillProposalFromGroup(p *blockchain.Proposal) *blockchain.PendingBlock {
	am.mu.Lock()
	defer am.mu.Unlock()
	existingBlocks := make([]*blockchain.MicroBlock, 0)
	missingBlocks := make(map[crypto.Identifier]struct{}, 0)
	for _, id := range p.HashList {
		found := false
		_, exists := am.pendingMicroblocks[id]
		if exists {
			found = true
			existingBlocks = append(existingBlocks, am.pendingMicroblocks[id].microblock)
			delete(am.pendingMicroblocks, id)
			//log.Debugf("microblock id: %x is deleted from pending when filling", id)
		}
		for e := am.stableMicroblocks.Front(); e != nil; e = e.Next() {
			// do something with e.Value
			mb := e.Value.(*blockchain.MicroBlock)
			if mb.Hash == id {
				existingBlocks = append(existingBlocks, mb)
				found = true
				am.stableMicroblocks.Remove(e)
				//log.Debugf("microblock id: %x is deleted from stable when filling", mb.Hash)
				break
			}
		}
		if !found {
			//没找到的加一个假微块进来
			// pendingMb :=
		}
	}
	return blockchain.NewPendingBlock(p, missingBlocks, existingBlocks)
}

// FillProposal pulls microblocks from the mempool and build a pending block,
// a pending block should include the proposal, micorblocks that already exist,
// and a missing list if there's any
//只获取自己组的区块
func (am *AckMem) FillProposalByGroup(p *blockchain.Proposal) *blockchain.PendingBlock {
	am.mu.Lock()
	defer am.mu.Unlock()
	existingBlocks := make([]*blockchain.MicroBlock, 0)
	missingBlocks := make(map[crypto.Identifier]struct{}, 0)
	for _, id := range p.HashList {
		found := false
		_, exists := am.pendingMicroblocks[id]
		if exists {
			found = true
			existingBlocks = append(existingBlocks, am.pendingMicroblocks[id].microblock)
			delete(am.pendingMicroblocks, id)
			//log.Debugf("microblock id: %x is deleted from pending when filling", id)
		}
		for e := am.stableMicroblocks.Front(); e != nil; e = e.Next() {
			// do something with e.Value
			mb := e.Value.(*blockchain.MicroBlock)
			if mb.Hash == id {
				existingBlocks = append(existingBlocks, mb)
				found = true
				am.stableMicroblocks.Remove(e)
				//log.Debugf("microblock id: %x is deleted from stable when filling", mb.Hash)
				break
			}
		}
		if !found {
			// missingBlocks[id] = struct{}{}
		}
	}
	return blockchain.NewPendingBlock(p, missingBlocks, existingBlocks)
}

func (am *AckMem) IsStable(id crypto.Identifier) bool {
	am.mu.Lock()
	defer am.mu.Unlock()
	_, exists := am.stableMBs[id]
	if exists {
		return true
	}
	return false
}

func (am *AckMem) TotalTx() int64 {
	return am.totalTx
}

func (am *AckMem) RemainingTx() int64 {
	return int64(am.txnList.Len())
}

func (am *AckMem) TotalMB() int64 {
	am.mu.Lock()
	defer am.mu.Unlock()
	return int64(len(am.microblockMap))
}

func (am *AckMem) TotalStableMb() int {
	am.mu.Lock()
	defer am.mu.Unlock()
	return am.TotalStableMbs
}

func (am *AckMem) TotalStableTime() time.Duration {
	am.mu.Lock()
	defer am.mu.Unlock()
	return am.TotalStableDelay
}

func (am *AckMem) StableMB() int64 {
	am.mu.Lock()
	defer am.mu.Unlock()
	return int64(am.stableMicroblocks.Len())
}

func (am *AckMem) PendingMB() int64 {
	am.mu.Lock()
	defer am.mu.Unlock()
	return int64(len(am.pendingMicroblocks))
}

func (am *AckMem) RemainingMB() int64 {
	am.mu.Lock()
	defer am.mu.Unlock()
	return int64(len(am.pendingMicroblocks) + am.stableMicroblocks.Len())
}

func (am *AckMem) AckList(id crypto.Identifier) []identity.NodeID {
	am.mu.Lock()
	defer am.mu.Unlock()
	pmb, exists := am.pendingMicroblocks[id]
	if exists {
		nodes := make([]identity.NodeID, 0, len(pmb.ackMap))
		for k, _ := range pmb.ackMap {
			nodes = append(nodes, k)
		}
		return nodes
	}
	return nil
}

func (am *AckMem) front() *blockchain.MicroBlock {
	if am.stableMicroblocks.Len() == 0 {
		return nil
	}
	ele := am.stableMicroblocks.Front()
	val, ok := ele.Value.(*blockchain.MicroBlock)
	if !ok {
		return nil
	}
	am.stableMicroblocks.Remove(ele)
	return val
}

func (am *AckMem) makeTxnSlice() []*message.Transaction {
	allTxn := make([]*message.Transaction, 0)
	for am.txnList.Len() > 0 {
		e := am.txnList.Front()
		allTxn = append(allTxn, e.Value.(*message.Transaction))
		am.txnList.Remove(e)
	}
	return allTxn
}
