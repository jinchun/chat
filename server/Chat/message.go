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

func (s *Stream) sendTo(name string, recv *proto.Request, msgType int32, content string) error {
	if content == "" {
		content = recv.Content
	}
	if content == "" {
		return nil
	}
	client, err := s.GetClientByUser(name)
	if err != nil {
		log.Println("get client failed,", err.Error())
		return err
	}
	if err := client.Stream.Send(&proto.Response{FromUser: recv.FromUser, Time: time.Now().Unix(), Content: content, MsgType: msgType, RoomName: recv.RoomName}); err != nil {
		log.Println("send error, continue...", err)
		return err
	}

	return nil
}

func (s *Stream) sendToAll(recv *proto.Request, msgType int32, content string) error {
	log.Println("send to all")

	members, err := s.Cache.SMembers(s.Room.MapKey(s.Room.getRoomName(recv.RoomName)))
	if err != nil {
		log.Printf("get room:%s members failed:%v\n", recv.RoomName, err.Error())
		return err
	}

	for _, member := range members {
		if err := s.sendTo(member, recv, msgType, content); err != nil {
			s.Room.LeftRoom(s.Room.getRoomName(recv.RoomName), member)
			s.DelClient(member)
			if err := s.sendToAll(recv, MsgTypeSystem, fmt.Sprintf("%s 离开了房间。", member)); err != nil {
				log.Println("send to all failed.", err.Error())
			}
		}
		log.Println("send to", member)
	}
	return nil
}
