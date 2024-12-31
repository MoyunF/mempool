package monitor

import (
	"sync"

	"github.com/gitferry/bamboo/log"
)

type MonitorManager struct {
	totalTime int //持续监控多长时间
	clock     int //间隔多少毫秒获取一次

}

var monitorManager *MonitorManager = nil
var mu sync.Mutex

func NewMonitorManager(totalTime int, clock int) *MonitorManager {
	//单例模式
	mu.Lock()
	defer mu.Unlock()

	if monitorManager == nil {
		log.Debugf("初始化监控器")
		monitorManager = new(MonitorManager)
		monitorManager.totalTime = totalTime
		monitorManager.clock = clock
		log.Debugf("监控器初始化成功 --- %v", monitorManager)
		return monitorManager
	}
	return monitorManager
}
