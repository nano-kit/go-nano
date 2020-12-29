package protocol

type NewUserRequest struct {
	Nickname string `json:"nickname"`
	GateUID  int64  `json:"gateUid"`
}

type JoinRoomRequest struct {
	Nickname  string `json:"nickname"`
	GateUID   int64  `json:"gateUid"`
	MasterUID int64  `json:"masterUid"`
}

type MasterStats struct {
	UID int64 `json:"uid"`
}
