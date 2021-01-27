package chat

import (
	"fmt"
	"log"
	"strconv"

	"github.com/nano-kit/go-nano"
	"github.com/nano-kit/go-nano/component"
	"github.com/nano-kit/go-nano/examples/cluster/protocol"
	"github.com/nano-kit/go-nano/session"
	"github.com/pingcap/errors"
)

type RoomService struct {
	component.Base
	group *nano.Group
}

func newRoomService() *RoomService {
	return &RoomService{
		group: nano.NewGroup("all-users"),
	}
}

func (rs *RoomService) JoinRoom(s *session.Session, msg *protocol.JoinRoomRequest) error {
	uid := strconv.FormatInt(msg.MasterUID, 10)
	if err := s.Bind(uid); err != nil {
		return errors.Trace(err)
	}

	broadcast := &protocol.NewUserBroadcast{
		Content: fmt.Sprintf("User user join: %v", msg.Nickname),
	}
	if err := rs.group.Broadcast("onNewUser", broadcast); err != nil {
		return errors.Trace(err)
	}
	return rs.group.Add(s)
}

type SyncMessage struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func (rs *RoomService) SyncMessage(s *session.Session, msg *SyncMessage) error {
	uid, _ := strconv.ParseInt(s.UID(), 10, 64)

	// Send an Notify to master server to stats
	if err := s.Notify("TopicService.Stats", &protocol.MasterStats{UID: uid}); err != nil {
		return errors.Trace(err)
	}

	// Sync message to all members in this room
	return rs.group.Broadcast("onMessage", msg)
}

func (rs *RoomService) userDisconnected(s *session.Session) {
	if err := rs.group.Leave(s); err != nil {
		log.Println("Remove user from group failed", s.UID(), err)
		return
	}
	log.Println("User session disconnected", s.UID())
}
