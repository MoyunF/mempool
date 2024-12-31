package monitor

import (
	"sync"
	"time"

	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/log"
)

type MonitorManager struct {
	//结果收集
	stableNumList    []int                               `json:"stable_num_list"`     //每秒stable的微块
	executedNumList  []int                               `json:"executed_num_list"`   //每秒执行成功的微块数
	committedNumList []int                               `json:"committed_num_list"`  //每秒被共识提交的微块数
	receiveTxNumList []int                               `json:"receive_tx_num_list"` //每秒已经接收的交易数 （到达的，包含已经被取出的）
	poolTxNumList    []int                               `json:"pool_tx_num_list"`    //每秒交易池中剩余的交易
	stableTime       map[crypto.Identifier]time.Duration `json:"stable_time"`         //每个微块被stable的用时
	executeTime      map[crypto.Identifier]time.Duration `json:"execute_time"`        //每个微块执行成功的用时，此处只记录当前节点执行的mb用时，想获得全部mb的用时，需要将所有节点的记录取并集
	committedTime    map[crypto.Identifier]time.Duration `json:"committed_time"`      //每个微块被共识提交的用时
}

var monitorManager *MonitorManager = nil
var mu sync.Mutex

func NewMonitorManager() *MonitorManager {
	//单例模式
	mu.Lock()
	defer mu.Unlock()
	if monitorManager == nil {
		log.Debugf("初始化监控器")
		monitorManager = new(MonitorManager)
		//结果收集器
		monitorManager.stableNumList = make([]int, 0)                            //每秒stable的微块数
		monitorManager.executedNumList = make([]int, 0)                          //每秒执行成功的微块数
		monitorManager.committedNumList = make([]int, 0)                         //每秒被共识提交的微块数
		monitorManager.receiveTxNumList = make([]int, 0)                         //每秒已经接收的交易数 （到达的，包含已经被取出的）
		monitorManager.poolTxNumList = make([]int, 0)                            //每秒交易池中剩余的交易
		monitorManager.stableTime = make(map[crypto.Identifier]time.Duration)    //每个微块被stable的用时
		monitorManager.executeTime = make(map[crypto.Identifier]time.Duration)   //每个微块执行成功的用时
		monitorManager.committedTime = make(map[crypto.Identifier]time.Duration) //每个微块被共识提交的用时
		log.Debugf("监控器初始化成功 --- %v", monitorManager)
		return monitorManager
	}
	return monitorManager
}

// stable的mb数量
func (m *MonitorManager) CollectStableNum(stable int) {
	mu.Lock()
	defer mu.Unlock()
	m.stableNumList = append(m.stableNumList, stable)
}

// 执行完成的mb数
func (m *MonitorManager) CollectExecutedNum(executed int) {
	mu.Lock()
	defer mu.Unlock()
	m.executedNumList = append(m.executedNumList, executed)
}

// 完成提交的mb数
func (m *MonitorManager) CollectCommittedNum(committed int) {
	mu.Lock()
	defer mu.Unlock()
	m.committedNumList = append(m.committedNumList, committed)
}

// 累计接收到的交易数
func (m *MonitorManager) CollectReceiveTxNum(receiveNum int) {
	mu.Lock()
	defer mu.Unlock()
	m.receiveTxNumList = append(m.receiveTxNumList, receiveNum)
}

// 交易池中剩的交易数
func (m *MonitorManager) CollectePoolTxNum(poolTxNum int) {
	mu.Lock()
	defer mu.Unlock()
	m.poolTxNumList = append(m.receiveTxNumList, poolTxNum)
}

// mb stable所用时间
func (m *MonitorManager) CollectStableTime(mbHash crypto.Identifier, duration time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	m.stableTime[mbHash] = duration
}

// mb 执行成功所用时间
func (m *MonitorManager) CollectExecuteTime(mbHash crypto.Identifier, duration time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	m.executeTime[mbHash] = duration
}

// mb committed所用时间
func (m *MonitorManager) CollectCommitteddTime(mbHash crypto.Identifier, duration time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	m.committedTime[mbHash] = duration
}

// stable的mb数量
func (m *MonitorManager) GetStableNumList() []int {
	mu.Lock()
	defer mu.Unlock()
	return m.stableNumList
}

// 执行完成的mb数
func (m *MonitorManager) GetExecutedNumList() []int {
	mu.Lock()
	defer mu.Unlock()
	return m.executedNumList
}

// 完成提交的mb数
func (m *MonitorManager) GetCommittedNumList() []int {
	mu.Lock()
	defer mu.Unlock()
	return m.committedNumList
}

// 累计接收到的交易数
func (m *MonitorManager) GetReceiveTxNumList() []int {
	mu.Lock()
	defer mu.Unlock()
	return m.receiveTxNumList
}

// 交易池中剩的交易数
func (m *MonitorManager) GetPoolTxNumList() []int {
	mu.Lock()
	defer mu.Unlock()
	return m.poolTxNumList
}

// mb stable所用时间
func (m *MonitorManager) GetStableTime(mbHash crypto.Identifier, duration time.Duration) map[crypto.Identifier]time.Duration {
	mu.Lock()
	defer mu.Unlock()
	return m.stableTime
}

// mb 执行成功所用时间
func (m *MonitorManager) GetExecuteTime(mbHash crypto.Identifier, duration time.Duration) map[crypto.Identifier]time.Duration {
	mu.Lock()
	defer mu.Unlock()
	return m.executeTime
}

// mb committed所用时间
func (m *MonitorManager) GetCommitteddTime(mbHash crypto.Identifier, duration time.Duration) map[crypto.Identifier]time.Duration {
	mu.Lock()
	defer mu.Unlock()
	return m.committedTime
}

// 将结果输出到本地文件中
func (m *MonitorManager) SaveResult() {

}
