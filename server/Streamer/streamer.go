package Streamer

import (
	proto "chatDemo/proto"
	"chatDemo/server/Database"
	"chatDemo/server/Database/Models"
	"chatDemo/server/Rabbit"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"
)

type Member struct {
	Name     string
	Status   int8
	JoinedAt int64
}

type Client struct {
	Name   string
	Stream proto.Chat_BidStreamServer
}

type Room struct {
	Name    string
	Members map[string]*Member
}

type Streamer struct {
	Rooms   map[string]*Room
	Clients map[string]*Client
	Rabbit  *Rabbit.Rabbit
	GORM    *Database.GORM
}

const defaultRoom = "defaultRoom"

const (
	MsgTypePrivate = 1
	MsgTypeRoom    = 2
	MsgTypeSystem  = 3
)

func (s *Streamer) getMemberByFromUser(name string) *Client {
	return s.Clients[s.memberMapKey(name)]
}

func (s *Streamer) roomMapKey(username string) string {
	return fmt.Sprintf("room_%s", username)
}

func (s *Streamer) memberMapKey(username string) string {
	return fmt.Sprintf("member_%s", username)
}

func (s *Streamer) leftRoom(roomName string, fromUser string) {
	log.Printf("%s 离开了 %s", fromUser, roomName)
	delete(s.Rooms[s.roomMapKey(roomName)].Members, s.memberMapKey(fromUser))
	delete(s.Clients, s.memberMapKey(fromUser))
}

func (s *Streamer) joinRoom(roomName string, fromUser string, stream proto.Chat_BidStreamServer) {
	if s.Rooms == nil {
		s.Rooms = make(map[string]*Room)
	}

	roomKey := s.roomMapKey(roomName)
	memberKey := s.memberMapKey(fromUser)
	if s.Rooms[roomKey] == nil {
		s.Rooms[roomKey] = &Room{Name: roomName}
	}
	if s.Rooms[roomKey].Members == nil {
		s.Rooms[roomKey].Members = make(map[string]*Member)
	}
	if s.Rooms[roomKey].Members[memberKey] == nil {
		s.Rooms[roomKey].Members[memberKey] = &Member{
			Name: fromUser,
		}
	}
	if s.Clients == nil {
		s.Clients = make(map[string]*Client)
	}
	if s.Clients[memberKey] == nil {
		s.Clients[memberKey] = &Client{
			Name:   fromUser,
			Stream: stream,
		}
	}

}

func (s *Streamer) BidStream(stream proto.Chat_BidStreamServer) error {
	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			log.Println("ctx done.")

			return ctx.Err()
		default:
			log.Printf("default.%v %v", s.Rooms[s.roomMapKey(defaultRoom)], s.Clients)
			body, _ := s.PreSub(stream)
			s.Rabbit.Pub(body)
		}
	}
}

func (s *Streamer) PreSub(stream proto.Chat_BidStreamServer) (string, error) {
	recv, err := stream.Recv()
	if err == io.EOF {
		return "io EOF.", nil
	}
	if err != nil {
		return "presub recv err:", err
	}

	roomName := s.getRoomName(recv)

	if recv.Event == "login" {
		s.joinRoom(roomName, recv.FromUser, stream)
	}

	recvObj := map[string]interface{}{
		"FromUser": recv.FromUser,
		"ToUser":   recv.ToUser,
		"RoomName": roomName,
		"Content":  recv.Content,
		"Event":    recv.Event,
	}

	recvBody, _ := json.Marshal(recvObj)

	return string(recvBody), nil
}

func (s *Streamer) Consume() error {
	msgs := s.Rabbit.Consume()
	for d := range msgs {
		var recv = &proto.Request{}
		body := d.Body
		json.Unmarshal(body, &recv)
		curUser := s.getMemberByFromUser(recv.FromUser)
		switch recv.Event {
		case "login": //TODO
			records := s.fetchRecords(s.getRoomName(recv))
			for _, record := range records {
				if err := curUser.Stream.Send(&proto.Response{FromUser: record.FromUser, Time: record.CreatedAt.Unix(), Content: record.Content, MsgType: MsgTypeRoom}); err != nil {
					return err
				}
			}

			s.sendToAll(recv, MsgTypeSystem, fmt.Sprintf("%s 进入了房间。", recv.FromUser))
			break
		case "logout":
			s.leftRoom(s.getRoomName(recv), recv.FromUser)
			s.sendToAll(recv, MsgTypeSystem, fmt.Sprintf("%s 离开了房间。", recv.FromUser))
			break
		default:
			if recv.Content != "" {
				switch recv.Content {
				case "whoami":
					if err := s.sendTo(curUser.Stream, recv, MsgTypeSystem, recv.FromUser); err != nil {
						return err
					}
					return nil
				default:
					log.Printf("[recv]: %v", recv)
					if err := s.sendToAll(recv, MsgTypeRoom, ""); err != nil {
						return err
					}
					s.saveRecord(recv)
				}
			}

		}
	}

	return nil
}

func (s *Streamer) getRoomName(recv *proto.Request) string {
	if recv.RoomName == "" {
		return defaultRoom
	}
	return recv.RoomName
}

func (s *Streamer) sendTo(stream proto.Chat_BidStreamServer, recv *proto.Request, msgType int32, content string) error {
	if content == "" {
		content = recv.Content
	}
	if content == "" {
		return nil
	}
	if err := stream.Send(&proto.Response{FromUser: recv.FromUser, Time: time.Now().Unix(), Content: content, MsgType: msgType, RoomName: recv.RoomName}); err != nil {
		log.Printf("send error, continue...", err)
		return err
	}

	return nil
}

func (s *Streamer) sendToAll(recv *proto.Request, msgType int32, content string) error {
	log.Println("send to all")
	for _, member := range s.Rooms[s.roomMapKey(s.getRoomName(recv))].Members {
		if err := s.sendTo(s.Clients[s.memberMapKey(member.Name)].Stream, recv, msgType, content); err != nil {
			s.leftRoom(s.getRoomName(recv), member.Name)
		}
		log.Println("send to ", member.Name)
	}
	return nil
}

func (s *Streamer) fetchRecords(room string) []Models.Record {
	var records []Models.Record
	if err := s.GORM.DB.Where("room = ?", room).Order("id desc").Limit(20).Find(&records).Error; err != nil {
		log.Printf("record fetch failed:%s\n", err.Error())
	}
	lenth := len(records)
	sortedRecords := make([]Models.Record, lenth)
	for i := range records {
		sortedRecords[i] = records[lenth-1-i]
	}

	return sortedRecords
}

func (s *Streamer) saveRecord(recv *proto.Request) {
	record := Models.Record{
		Room:      recv.RoomName,
		FromUser:  recv.FromUser,
		ToUser:    recv.ToUser,
		Content:   recv.Content,
		CreatedAt: time.Now(),
	}
	if err := s.GORM.DB.Create(&record).Error; err != nil {
		log.Printf("record creation failed:%s\n", err.Error())
	}
}
