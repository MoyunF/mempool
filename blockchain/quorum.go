package blockchain

import (
	"fmt"
	"time"

	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/types"
)

type Vote struct {
	types.View
	Voter     identity.NodeID
	BlockID   crypto.Identifier
	Timestamp time.Time
	crypto.Signature
}

type Stable struct {
	AckInGroup     []identity.NodeID //组内ack
	AckOutGroup    []identity.NodeID //组外ack
	MicroblockID   crypto.Identifier //微块hash
	Sender         identity.NodeID   //发送方
	GroupId        int               //区块所在的组
	MbCreationTime time.Time         //微块的创建时间
}

type Ack struct {
	Receiver     identity.NodeID //ack的发送者
	MicroblockID crypto.Identifier
	crypto.Signature
	OutGroup bool //是否为组外节点回复
}

type QC struct {
	Leader  identity.NodeID
	View    types.View
	BlockID crypto.Identifier
	Signers []identity.NodeID
	crypto.AggSig
	crypto.Signature
}

type Quorum struct {
	total int
	votes map[crypto.Identifier]map[identity.NodeID]*Vote
}

func MakeAck(receiver identity.NodeID, id crypto.Identifier) *Ack {
	sig, err := crypto.PrivSign(crypto.IDToByte(id), receiver, nil)
	if err != nil {
		log.Fatalf("[%v] has an error when signing an ack", receiver)
		return nil
	}
	return &Ack{
		Receiver:     receiver,
		MicroblockID: id,
		Signature:    sig,
	}
}

func MakeVote(view types.View, voter identity.NodeID, id crypto.Identifier) *Vote {
	// TODO: uncomment the following
	sig, err := crypto.PrivSign(crypto.IDToByte(id), voter, nil)
	if err != nil {
		log.Fatalf("[%v] has an error when signing a vote", voter)
		return nil
	}
	return &Vote{
		View:      view,
		Voter:     voter,
		BlockID:   id,
		Signature: sig,
	}
}

func NewQuorum(total int) *Quorum {
	return &Quorum{
		total: total,
		votes: make(map[crypto.Identifier]map[identity.NodeID]*Vote),
	}
}

// Add adds id to quorum ack records
func (q *Quorum) Add(vote *Vote) (bool, *QC) {
	if q.superMajority(vote.BlockID) {
		return false, nil
	}
	_, exist := q.votes[vote.BlockID]
	if !exist {
		//	first time of receiving the vote for this block
		q.votes[vote.BlockID] = make(map[identity.NodeID]*Vote)
	}
	q.votes[vote.BlockID][vote.Voter] = vote
	if q.superMajority(vote.BlockID) {
		aggSig, signers, err := q.getSigs(vote.BlockID)
		if err != nil {
			//log.Warningf("cannot generate a valid qc, view: %v, block id: %x: %w", vote.View, vote.BlockID, err)
		}
		qc := &QC{
			View:    vote.View,
			BlockID: vote.BlockID,
			AggSig:  aggSig,
			Signers: signers,
		}
		return true, qc
	}
	return false, nil
}

// Super majority quorum satisfied
func (q *Quorum) superMajority(blockID crypto.Identifier) bool {
	return q.size(blockID) > q.total*2/3
}

// Size returns ack size for the block
func (q *Quorum) size(blockID crypto.Identifier) int {
	return len(q.votes[blockID])
}

func (q *Quorum) getSigs(blockID crypto.Identifier) (crypto.AggSig, []identity.NodeID, error) {
	var sigs crypto.AggSig
	var signers []identity.NodeID
	_, exists := q.votes[blockID]
	if !exists {
		return nil, nil, fmt.Errorf("sigs does not exist, id: %x", blockID)
	}
	for _, vote := range q.votes[blockID] {
		sigs = append(sigs, vote.Signature)
		signers = append(signers, vote.Voter)
	}

	return sigs, signers, nil
}
