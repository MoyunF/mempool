package monitor

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"sync"
	"time"

	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/log"
)

type MonitorManager struct {
	//结果收集
	StableNumList    []int             `json:"stable_num_list"`     //每秒stable的微块
	ExecutedNumList  []int             `json:"executed_num_list"`   //每秒执行成功的微块数
	CommittedNumList []int             `json:"committed_num_list"`  //每秒被共识提交的微块数
	ReceiveTxNumList []int             `json:"receive_tx_num_list"` //每秒已经接收的交易数 （到达的，包含已经被取出的）
	PoolTxNumList    []int             `json:"pool_tx_num_list"`    //每秒交易池中剩余的交易
	StableTime       map[string]string `json:"stable_time"`         //每个微块被stable的用时
	ExecuteTime      map[string]string `json:"execute_time"`        //每个微块执行成功的用时，此处只记录当前节点执行的mb用时，想获得全部mb的用时，需要将所有节点的记录取并集
	CommittedTime    map[string]string `json:"committed_time"`      //每个微块被共识提交的用时
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
		monitorManager.StableNumList = make([]int, 0)          //每秒stable的微块数
		monitorManager.ExecutedNumList = make([]int, 0)        //每秒执行成功的微块数
		monitorManager.CommittedNumList = make([]int, 0)       //每秒被共识提交的微块数
		monitorManager.ReceiveTxNumList = make([]int, 0)       //每秒已经接收的交易数 （到达的，包含已经被取出的）
		monitorManager.PoolTxNumList = make([]int, 0)          //每秒交易池中剩余的交易
		monitorManager.StableTime = make(map[string]string)    //每个微块被stable的用时
		monitorManager.ExecuteTime = make(map[string]string)   //每个微块执行成功的用时
		monitorManager.CommittedTime = make(map[string]string) //每个微块被共识提交的用时
		log.Debugf("监控器初始化成功 --- %v", monitorManager)
		return monitorManager
	}
	return monitorManager
}

// stable的mb数量
func (m *MonitorManager) CollectStableNum(stable int) {
	mu.Lock()
	defer mu.Unlock()
	m.StableNumList = append(m.StableNumList, stable)
}

// 执行完成的mb数
func (m *MonitorManager) CollectExecutedNum(executed int) {
	mu.Lock()
	defer mu.Unlock()
	m.ExecutedNumList = append(m.ExecutedNumList, executed)
}

// 完成提交的mb数
func (m *MonitorManager) CollectCommittedNum(committed int) {
	mu.Lock()
	defer mu.Unlock()
	m.CommittedNumList = append(m.CommittedNumList, committed)
}

// 累计接收到的交易数
func (m *MonitorManager) CollectReceiveTxNum(receiveNum int) {
	mu.Lock()
	defer mu.Unlock()
	m.ReceiveTxNumList = append(m.ReceiveTxNumList, receiveNum)
}

// 交易池中剩的交易数
func (m *MonitorManager) CollectePoolTxNum(poolTxNum int) {
	mu.Lock()
	defer mu.Unlock()
	m.PoolTxNumList = append(m.ReceiveTxNumList, poolTxNum)
}

// mb stable所用时间
func (m *MonitorManager) CollectStableTime(mbHash crypto.Identifier, duration time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	m.StableTime[m.idToString(mbHash)] = duration.String()
}

// mb 执行成功所用时间
func (m *MonitorManager) CollectExecuteTime(mbHash crypto.Identifier, duration time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	m.ExecuteTime[m.idToString(mbHash)] = duration.String()
}

// mb committed所用时间
func (m *MonitorManager) CollectCommitteddTime(mbHash crypto.Identifier, duration time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	m.CommittedTime[m.idToString(mbHash)] = duration.String()
}

// stable的mb数量
func (m *MonitorManager) GetStableNumList() []int {
	mu.Lock()
	defer mu.Unlock()
	return m.StableNumList
}

// 执行完成的mb数
func (m *MonitorManager) GetExecutedNumList() []int {
	mu.Lock()
	defer mu.Unlock()
	return m.ExecutedNumList
}

// 完成提交的mb数
func (m *MonitorManager) GetCommittedNumList() []int {
	mu.Lock()
	defer mu.Unlock()
	return m.CommittedNumList
}

// 累计接收到的交易数
func (m *MonitorManager) GetReceiveTxNumList() []int {
	mu.Lock()
	defer mu.Unlock()
	return m.ReceiveTxNumList
}

// 交易池中剩的交易数
func (m *MonitorManager) GetPoolTxNumList() []int {
	mu.Lock()
	defer mu.Unlock()
	return m.PoolTxNumList
}

// mb stable所用时间
func (m *MonitorManager) GetStableTime(mbHash crypto.Identifier, duration time.Duration) map[string]string {
	mu.Lock()
	defer mu.Unlock()
	return m.StableTime
}

// mb 执行成功所用时间
func (m *MonitorManager) GetExecuteTime(mbHash crypto.Identifier, duration time.Duration) map[string]string {
	mu.Lock()
	defer mu.Unlock()
	return m.ExecuteTime
}

// mb committed所用时间
func (m *MonitorManager) GetCommitteddTime(mbHash crypto.Identifier, duration time.Duration) map[string]string {
	mu.Lock()
	defer mu.Unlock()
	return m.CommittedTime
}

//hash转为16进制的字符串形式
func (m *MonitorManager) idToString(hash crypto.Identifier) string {
	return hex.EncodeToString(hash[:])
}

// 将结果输出到本地文件中
func (m *MonitorManager) SaveResult(filePath string) error {
	mu.Lock()
	defer mu.Unlock()
	log.Infof("%+v", monitorManager)
	// 将结构体序列化为 JSON
	// 将结构体序列化为 JSON
	data, err := json.MarshalIndent(monitorManager, "", "  ")
	if err != nil {
		log.Errorf("failed to marshal MonitorManager to JSON: %w", err)
		return err
	}

	// 将 JSON 数据写入文件
	err = ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		log.Errorf("failed to write JSON to file: %w", err)
		return err
	}

	log.Debugf("监控数据已保存到 %s", filePath)
	return nil
}
