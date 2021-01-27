package master

import (
	"log"
	"strconv"
	"strings"

	"github.com/nano-kit/go-nano/component"
	"github.com/nano-kit/go-nano/examples/cluster/protocol"
	"github.com/nano-kit/go-nano/session"
	"github.com/pingcap/errors"
)

type User struct {
	session  *session.Session
	nickname string
	gateID   int64
	masterID int64
	balance  int64
	message  int
}

type TopicService struct {
	component.Base
	nextUID int64
	users   map[int64]*User
}

func newTopicService() *TopicService {
	return &TopicService{
		users: map[int64]*User{},
	}
}

type ExistsMembersResponse struct {
	Members string `json:"members"`
}

func (ts *TopicService) NewUser(s *session.Session, msg *protocol.NewUserRequest) error {
	ts.nextUID++
	uid := ts.nextUID
	uidstr := strconv.FormatInt(uid, 10)
	if err := s.Bind(uidstr); err != nil {
		return errors.Trace(err)
	}

	var members []string
	for _, u := range ts.users {
		members = append(members, u.nickname)
	}
	err := s.Push("onMembers", &ExistsMembersResponse{Members: strings.Join(members, ",")})
	if err != nil {
		return errors.Trace(err)
	}

	user := &User{
		session:  s,
		nickname: msg.Nickname,
		gateID:   msg.GateUID,
		masterID: uid,
		balance:  1000,
	}
	ts.users[uid] = user

	chat := &protocol.JoinRoomRequest{
		Nickname:  msg.Nickname,
		GateUID:   msg.GateUID,
		MasterUID: uid,
	}
	return s.Notify("RoomService.JoinRoom", chat)
}

type UserBalanceResponse struct {
	CurrentBalance int64 `json:"currentBalance"`
}

func (ts *TopicService) Stats(s *session.Session, msg *protocol.MasterStats) error {
	// It's OK to use map without lock because of this service running in main thread
	user, found := ts.users[msg.UID]
	if !found {
		return errors.Errorf("User not found: %v", msg.UID)
	}
	user.message++
	user.balance--
	return s.Push("onBalance", &UserBalanceResponse{user.balance})
}

func (ts *TopicService) userDisconnected(s *session.Session) {
	uid := s.UID()
	uidint, _ := strconv.ParseInt(uid, 10, 64)
	delete(ts.users, uidint)
	log.Println("User session disconnected", s.UID())
}
