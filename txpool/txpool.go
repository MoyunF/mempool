package txpool

import (
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/db"
	"github.com/gitferry/bamboo/log"
	"github.com/gitferry/bamboo/message"
	"github.com/gitferry/bamboo/node"
)

type Txpool struct {
	node         node.Node
	Transactions []*message.Transaction //交易
	Poolsize     int                    //交易池大小
	Len          int                    //当前交易数量
	mu           sync.Mutex
	FetchSignal  chan interface{}
}

func NewTxpool(node node.Node) *Txpool {
	txpool := &Txpool{
		node:         node,
		Transactions: make([]*message.Transaction, 0),
		Poolsize:     config.GetConfig().Poolsize,
		Len:          0,
		FetchSignal:  make(chan interface{}, config.GetConfig().Poolsize),
	}
	return txpool
}

//添加nums笔交易
func (pool *Txpool) AddTx(nums int) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	payloadSize := config.Configuration.PayloadSize
	for i := 0; i < nums; i++ {
		if pool.Len == pool.Poolsize {
			log.Warningf("txpool is full")
			pool.FetchSignal <- struct{}{}
			break
		}
		pool.Len += 1
		tx := pool.generateTx(payloadSize, strconv.Itoa(pool.Len+i))
		pool.Transactions = append(pool.Transactions, tx)
		pool.FetchSignal <- struct{}{}
	}
	//log.Debugf("%+v", pool.Transactions)
}

//取nums笔交易，如果不满足就取出所有交易
func (pool *Txpool) FetchTx(nums int) []*message.Transaction {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	result := make([]*message.Transaction, 0)
	if pool.Len < nums {
		result = append(result, pool.Transactions...)
		pool.Transactions = make([]*message.Transaction, 0) //清空交易池
		pool.Len = 0
	} else {
		result = append(result, pool.Transactions[:nums]...)
		pool.Transactions = pool.Transactions[nums:]
		pool.Len = len(pool.Transactions)
	}
	return result
}

func (pool *Txpool) TxLen() int {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return pool.Len
}

// func (pool *Txpool) observePool() {
// 	pool.mu.Lock()
// 	defer pool.mu.Unlock()

// 	for {
// 		if pool.Len > config.GetConfig().MSize {

// 		}
// 	}
// }

func (pool *Txpool) generateTx(payloadSize int, id string) *message.Transaction {
	//生成交易
	value := make([]byte, payloadSize)
	rand.Read(value)
	cmd := db.Command{
		Value:     value,
		Key:       1,
		ClientID:  pool.node.ID(),
		CommandID: 1,
	}
	tx := &message.Transaction{
		Command:   cmd,
		Timestamp: time.Now(),
		ID:        id,
		NodeID:    pool.node.ID(),
	}
	return tx
}
