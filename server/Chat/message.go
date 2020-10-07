package Chat

import (
	proto "chatDemo/proto"
	"chatDemo/server/Cache"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type Message struct {
	Cache Cache.Redis
}

type MsgBody struct {
	Room      string
	FromUser  string
	ToUser    string
	Content   string
	CreatedAt time.Time
}

func (m Message) roomMsgMapKey(roomName string) string {
	return fmt.Sprintf("roomMsg:%s", roomName)
}

func (m Message) offlineMsgMapKey(name string) string {
	return fmt.Sprintf("offlineMsg:%s", name)
}

func (m Message) FetchRecords(roomName string) []MsgBody {
	records, err := m.Cache.LRange(m.roomMsgMapKey(roomName), 0, 20)
	if err != nil {
		log.Println("fetch records failed.", err.Error())
	}
	response := make([]MsgBody, len(records))
	for i, record := range records {
		var _msgBody MsgBody
		if err := json.Unmarshal([]byte(record), &_msgBody); err != nil {
			continue
		}
		response[len(records)-i-1] = _msgBody
	}

	return response
}

func (m Message) SaveRecord(recv *proto.Request) {
	if recv.ToUser != "" {
		return
	}
	msgBody := MsgBody{
		Room:      recv.RoomName,
		FromUser:  recv.FromUser,
		ToUser:    recv.ToUser,
		Content:   recv.Content,
		CreatedAt: time.Now(),
	}

	msgBodyStr, _ := json.Marshal(msgBody)
	if err := m.Cache.LPush(m.roomMsgMapKey(recv.RoomName), string(msgBodyStr)); err != nil {
		log.Println("save record failed.", err.Error())
	}
}

func (m Message) OfflineMsg(recv *proto.Request) {
	msgBody := MsgBody{
		Room:      recv.RoomName,
		FromUser:  recv.FromUser,
		ToUser:    recv.ToUser,
		Content:   recv.Content,
		CreatedAt: time.Now(),
	}
	msgBodyStr, _ := json.Marshal(msgBody)
	if err := m.Cache.LPush(m.offlineMsgMapKey(recv.ToUser), string(msgBodyStr)); err != nil {
		log.Println("save offline message failed.", err.Error())
	}
}

func (m Message) FetchOfflineMsg(name string) []MsgBody {
	var msg []MsgBody
	for {
		_msg, err := m.Cache.LPop(m.offlineMsgMapKey(name))
		if err != nil {
			break
		}
		var _msgBody MsgBody
		if err := json.Unmarshal([]byte(_msg), &_msgBody); err != nil {
			continue
		}
		msg = append(msg, _msgBody)
	}

	return msg
}
