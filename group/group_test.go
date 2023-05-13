package group

import (
	"fmt"
	"testing"
)

// one chain
// func TestInitGroup(t *testing.T) {
// 	gm := NewGroupManager()
// 	gm.groupNum = 4
// 	gm.groupList = make([]group, gm.groupNum)
// 	gm.memberNum = 3
// 	start_num := 0
// 	for i := 0; i < gm.groupNum; i++ {
// 		g := group{groupId: i, content: make(map[identity.NodeID]struct{})}
// 		for j := 0; j < gm.memberNum; j++ {
// 			t.Log(identity.NewNodeID((i+start_num)%4 + 1))
// 			g.content[identity.NewNodeID((i+start_num)%4+1)] = struct{}{}
// 			start_num++
// 		}
// 		start_num--
// 		gm.groupList[i] = g
// 		t.Log(g)
// 	}
// }

func TestGenerate(t *testing.T) {
	slice := []int{1, 2, 3}
	for _, v := range slice {

		fmt.Println(slice[0], v, len(slice))
		slice = slice[1:]
	}
}
