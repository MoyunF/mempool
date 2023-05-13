package group

import (
	"math/rand"
	"sync"

	"github.com/gitferry/bamboo/config"
	"github.com/gitferry/bamboo/identity"
	"github.com/gitferry/bamboo/log"
)

/*
	用来管理广播分组
*/

type group struct {
	groupId int
	content map[identity.NodeID]struct{}
}

type GroupManager struct {
	version   int              //当前节点的group分组版本
	groupNum  int              //分组数量
	memberNum int              //每个组多少人
	groupList []group          //分组情况
	myGroup   map[int]struct{} //自己所在的分组
}

var groupManager *GroupManager = nil
var mu sync.Mutex

func NewGroupManager(myId identity.NodeID) *GroupManager {
	//单例模式
	mu.Lock()
	defer mu.Unlock()

	if groupManager == nil {
		groupManager := new(GroupManager)
		groupManager.version = 0
		groupManager.groupNum = config.GetConfig().GroupNum
		groupManager.memberNum = config.GetConfig().MemberNum
		groupManager.groupList = make([]group, groupManager.groupNum)
		groupManager.myGroup = map[int]struct{}{}
		groupManager.initGroup(myId)
		log.Debugf("%v", groupManager)
		return groupManager
	}
	return groupManager
}

//更新分组
func (gm *GroupManager) updateGroup() {

}

//

//初始化分组,按顺序分
func (gm *GroupManager) initGroup(myId identity.NodeID) {
	log.Debugf("初始化名单")
	start_num := 0
	for i := 0; i < gm.groupNum; i++ {
		g := group{groupId: i, content: make(map[identity.NodeID]struct{})}
		for j := 0; j < gm.memberNum; j++ {
			member_id := identity.NewNodeID((i+start_num)%config.GetConfig().N() + 1)
			if member_id == myId {
				gm.myGroup[i] = struct{}{}
				log.Debugf("%v is in group %v", myId, i)
			}
			g.content[member_id] = struct{}{}
			start_num++
		}
		start_num--
		gm.groupList[i] = g
	}
}

//获取分组名单 id从0开始
func (gm *GroupManager) GetGroupListByGroupId(groupNum int) map[identity.NodeID]struct{} {
	return gm.groupList[groupNum].content
}

func (gm *GroupManager) IsInMyGroup(groupId int) bool {
	if _, ok := gm.myGroup[groupId]; ok {
		return true
	}
	return false
}

//随机为mb生成一个分组
func GenerateGroupIdByRand() int {
	return rand.Intn(config.Configuration.GroupNum)
}
