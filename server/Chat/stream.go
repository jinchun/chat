package Chat

import (
	proto "chatDemo/proto"
	"chatDemo/server/Cache"
	"chatDemo/server/Queue"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"
)

type Stream struct {
	Clients map[string]*Client
	Queue   *Queue.Rabbit
	Cache   *Cache.Redis
	Room
	User
	Message
}

type Client struct {
	Name   string
	Stream proto.Chat_BidStreamServer
}

const (
	MsgTypePrivate            = 1
	MsgTypeRoom               = 2
	MsgTypeSystem             = 3
	MsgTypeLoggedIn           = 4
	LoginErrorUserHasLoggedIn = "user has logged in"
	RecvErrorUnknown          = "unknown error"
)

func (s *Stream) BidStream(stream proto.Chat_BidStreamServer) error {
	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			log.Println("ctx done.")
			return ctx.Err()
		default:
			body, err := s.prepare(stream)
			if err != nil {
				switch err.Error() {
				case LoginErrorUserHasLoggedIn:
					if err := stream.Send(&proto.Response{
						Content: err.Error(),
						MsgType: MsgTypeLoggedIn,
					}); err != nil {
						log.Println("logged in msg send failed.", err.Error())
					}

				case RecvErrorUnknown:
					return err
				}
			} else {
				s.Queue.Publish(body)
			}
		}
	}
}

func (s *Stream) prepare(stream proto.Chat_BidStreamServer) (string, error) {
	recv, err := stream.Recv()
	if err == io.EOF {
		return "io EOF.", nil
	}
	if err != nil {
		return "prepare recv err:", err
	}

	roomName := s.Room.getRoomName(recv.RoomName)

	if recv.Event == "login" {
		// 检测是否有同名用户在线
		if _, err := s.GetClientByUser(recv.FromUser); err != nil {
			s.Room.JoinRoom(roomName, recv.FromUser)
			s.SaveClient(recv.FromUser, stream)
		} else {
			return LoginErrorUserHasLoggedIn, errors.New(LoginErrorUserHasLoggedIn)
		}
	}

	recvObj := map[string]interface{}{
		"FromUser": recv.FromUser,
		"ToUser":   recv.ToUser,
		"RoomName": roomName,
		"Content":  recv.Content,
		"Event":    recv.Event,
	}

	recvBody, err := json.Marshal(recvObj)
	if err != nil {
		log.Println("prepare failed.", err.Error())
		return RecvErrorUnknown, errors.New(RecvErrorUnknown)
	}
	if string(recvBody) == "" {
		return RecvErrorUnknown, errors.New(RecvErrorUnknown)
	}

	return string(recvBody), nil
}

func (s *Stream) Consume() error {
	deliveries := s.Queue.Consume()
	for delivery := range deliveries {
		var recv = &proto.Request{}
		body := delivery.Body
		if err := json.Unmarshal(body, &recv); err != nil {
			log.Println("json unmarshal failed", err, string(body))
			continue
		}
		switch recv.Event {
		case "login": //TODO
			curUser, err := s.GetClientByUser(recv.FromUser)
			if err != nil {
				log.Println("get client failed.", err.Error())
				return err
			}
			records := s.Message.FetchRecords(s.Room.getRoomName(recv.RoomName))
			for _, record := range records {
				if err := curUser.Stream.Send(&proto.Response{FromUser: record.FromUser, Time: record.CreatedAt.Unix(), Content: record.Content, MsgType: MsgTypeRoom}); err != nil {
					return err
				}
			}

			offlineMsg := s.Message.FetchOfflineMsg(recv.FromUser)
			for _, msg := range offlineMsg {
				if err := curUser.Stream.Send(&proto.Response{FromUser: msg.FromUser, Time: msg.CreatedAt.Unix(), Content: msg.Content, MsgType: MsgTypePrivate}); err != nil {
					return err
				}
			}

			if err := s.SendToAll(recv, MsgTypeSystem, fmt.Sprintf("%s 进入了房间。", recv.FromUser)); err != nil {
				log.Println("send to all failed.", err.Error())
			}
			break
		case "logout":
			if s.Room.LeftRoom(s.Room.getRoomName(recv.RoomName), recv.FromUser) {
				s.DelClient(recv.FromUser)
				if err := s.SendToAll(recv, MsgTypeSystem, fmt.Sprintf("%s 离开了房间。", recv.FromUser)); err != nil {
					log.Println("send to all failed.", err.Error())
				}
			}
			break
		default:
			if recv.Content != "" {
				switch recv.Content {
				case "whoami":
					if err := s.SendTo(recv.FromUser, recv, MsgTypeSystem, recv.FromUser); err != nil {
						return err
					}
					return nil
				default:
					log.Printf("[recv]: %v", recv)
					if recv.ToUser == "" {
						if err := s.SendToAll(recv, MsgTypeRoom, ""); err != nil {
							return err
						}
					} else {
						s.SendToPrivate(recv)
					}

					s.Message.SaveRecord(recv)
				}
			}

		}
	}

	return nil
}

func (s *Stream) SendToPrivate(recv *proto.Request) {
	s.SendTo(recv.FromUser, recv, MsgTypeRoom, recv.Content)
	s.SendTo(recv.ToUser, recv, MsgTypePrivate, recv.Content)
}

func (s *Stream) SaveClient(name string, stream proto.Chat_BidStreamServer) {
	if s.Clients == nil {
		s.Clients = make(map[string]*Client)
	}
	s.Clients[s.User.MapKey(name)] = &Client{
		Name:   name,
		Stream: stream,
	}
}

func (s *Stream) DelClient(name string) {
	delete(s.Clients, s.User.MapKey(name))
}

func (s *Stream) GetClientByUser(name string) (*Client, error) {
	if _, ok := s.Clients[s.User.MapKey(name)]; ok {
		return s.Clients[s.User.MapKey(name)], nil
	}
	return nil, errors.New("client not exists")
}

func (s *Stream) SendTo(name string, recv *proto.Request, msgType int32, content string) error {
	if content == "" {
		content = recv.Content
	}
	if content == "" {
		return nil
	}
	client, err := s.GetClientByUser(name)
	if err != nil {
		log.Println("get client failed, changed to offline message.")
		s.Message.OfflineMsg(recv)
		return nil
	}
	if err := client.Stream.Send(&proto.Response{FromUser: recv.FromUser, Time: time.Now().Unix(), Content: content, MsgType: msgType, RoomName: recv.RoomName}); err != nil {
		log.Println("send error, continue...", err)
		return err
	}

	return nil
}

func (s *Stream) SendToAll(recv *proto.Request, msgType int32, content string) error {
	log.Println("send to all")

	members, err := s.Cache.SMembers(s.Room.MapKey(s.Room.getRoomName(recv.RoomName)))
	if err != nil {
		log.Printf("get room:%s members failed:%v\n", recv.RoomName, err.Error())
		return err
	}

	for _, member := range members {
		if err := s.SendTo(member, recv, msgType, content); err != nil {
			s.Room.LeftRoom(s.Room.getRoomName(recv.RoomName), member)
			s.DelClient(member)
			if err := s.SendToAll(recv, MsgTypeSystem, fmt.Sprintf("%s 离开了房间。", member)); err != nil {
				log.Println("send to all failed.", err.Error())
			}
		}
		log.Println("send to", member)
	}
	return nil
}
