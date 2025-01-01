package monitor

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/log"
)

/*
	目前已支持的数据：
		1.stable、committed、executed的数量随时间变化图， total = tps * time tps是斜率，可以计算
		2.stable、committed、exiceted的平均时延，mb的时延分布情况
		3.result被接受的平均用时 mbhash:time
		4.mb从创建到接收的用时，不包括后面投票

*/

type MonitorManager struct {
	//结果收集
	StableNumList    []int               `json:"stable_num_list"`     //每秒stable的微块
	ExecutedNumList  []int               `json:"executed_num_list"`   //每秒执行成功的微块数
	CommittedNumList []int               `json:"committed_num_list"`  //每秒被共识提交的微块数
	ReceiveTxNumList []int               `json:"receive_tx_num_list"` //每秒已经接收的交易数 （到达的，包含已经被取出的）
	PoolTxNumList    []int               `json:"pool_tx_num_list"`    //每秒交易池中剩余的交易
	StableTime       map[string]string   `json:"stable_time"`         //每个微块被stable的用时
	ExecuteTime      map[string]string   `json:"execute_time"`        //每个微块执行成功的用时，此处只记录当前节点执行的mb用时，想获得全部mb的用时，需要将所有节点的记录取并集
	CommittedTime    map[string]string   `json:"committed_time"`      //每个微块被共识提交的用时
	ResultTime       map[string][]string `json:"result_time"`         //每个需要协作的微块收到执行完成的用时,所用时间为列表，代表来自不同节点的执行结果
	MbReceivedTime   map[string]string   `json:"mb_received_time"`    //每个微块被节点接受的用时
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
		monitorManager.StableNumList = make([]int, 0)           //每秒stable的微块数
		monitorManager.ExecutedNumList = make([]int, 0)         //每秒执行成功的微块数
		monitorManager.CommittedNumList = make([]int, 0)        //每秒被共识提交的微块数
		monitorManager.ReceiveTxNumList = make([]int, 0)        //每秒已经接收的交易数 （到达的，包含已经被取出的）
		monitorManager.PoolTxNumList = make([]int, 0)           //每秒交易池中剩余的交易
		monitorManager.StableTime = make(map[string]string)     //每个微块被stable的用时
		monitorManager.ExecuteTime = make(map[string]string)    //每个微块执行成功的用时
		monitorManager.CommittedTime = make(map[string]string)  //每个微块被共识提交的用时
		monitorManager.ResultTime = make(map[string][]string)   //微块对应result被接受的用时
		monitorManager.MbReceivedTime = make(map[string]string) //mb被接收的用时
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
	m.StableTime[m.idToString(mbHash)] = m.durationToMs(duration)
}

// mb 执行成功所用时间
func (m *MonitorManager) CollectExecuteTime(mbHash crypto.Identifier, duration time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	m.ExecuteTime[m.idToString(mbHash)] = m.durationToMs(duration)
}

// mb committed所用时间
func (m *MonitorManager) CollectCommitteddTime(mbHash crypto.Identifier, duration time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	m.CommittedTime[m.idToString(mbHash)] = m.durationToMs(duration)
}

// f+1个执行结果的所用时间
func (m *MonitorManager) CollectResultTime(mbHash crypto.Identifier, duration time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	if list, exist := m.ResultTime[m.idToString(mbHash)]; exist {
		log.Debugf("CollectResultTime() --- mb:[%x] exist len:[%v], list[%v]", mbHash, len(list), list)
		list = append(list, m.durationToMs(duration))
		//由于append时数组触发扩容，产生了新的数组因此需要将修改后的切片与map重新绑定
		m.ResultTime[m.idToString(mbHash)] = list
	} else {
		log.Debugf("CollectResultTime() --- new list mb:[%x]", mbHash)
		timeList := make([]string, 0)
		timeList = append(timeList, m.durationToMs(duration))
		m.ResultTime[m.idToString(mbHash)] = timeList
	}
}

// mb创建到节点接收的用时
func (m *MonitorManager) CollectMbReceiveTime(mbHash crypto.Identifier, duration time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	m.MbReceivedTime[m.idToString(mbHash)] = m.durationToMs(duration)
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
func (m *MonitorManager) GetStableTime() map[string]string {
	mu.Lock()
	defer mu.Unlock()
	return m.StableTime
}

// mb 执行成功所用时间
func (m *MonitorManager) GetExecuteTime() map[string]string {
	mu.Lock()
	defer mu.Unlock()
	return m.ExecuteTime
}

// mb committed所用时间
func (m *MonitorManager) GetCommitteddTime() map[string]string {
	mu.Lock()
	defer mu.Unlock()
	return m.CommittedTime
}

// f+1个执行结果的用时
func (m *MonitorManager) GetResultTime() map[string][]string {
	mu.Lock()
	defer mu.Unlock()
	return m.ResultTime
}

// mb创建到接受的用时
func (m *MonitorManager) GetMbReceiveTime() map[string]string {
	mu.Lock()
	defer mu.Unlock()
	return m.MbReceivedTime
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

//将duration格式化为ms字符串
func (m *MonitorManager) durationToMs(duration time.Duration) string {
	// 强制转换为毫秒
	milliseconds := float64(duration) / float64(time.Millisecond)

	// 打印毫秒数
	return fmt.Sprintf("%.3f", milliseconds)
}
