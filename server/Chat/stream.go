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

			if err := s.sendToAll(recv, MsgTypeSystem, fmt.Sprintf("%s 进入了房间。", recv.FromUser)); err != nil {
				log.Println("send to all failed.", err.Error())
			}
			break
		case "logout":
			if s.Room.LeftRoom(s.Room.getRoomName(recv.RoomName), recv.FromUser) {
				s.DelClient(recv.FromUser)
				if err := s.sendToAll(recv, MsgTypeSystem, fmt.Sprintf("%s 离开了房间。", recv.FromUser)); err != nil {
					log.Println("send to all failed.", err.Error())
				}
			}
			break
		default:
			if recv.Content != "" {
				switch recv.Content {
				case "whoami":
					if err := s.sendTo(recv.FromUser, recv, MsgTypeSystem, recv.FromUser); err != nil {
						return err
					}
					return nil
				default:
					log.Printf("[recv]: %v", recv)
					if err := s.sendToAll(recv, MsgTypeRoom, ""); err != nil {
						return err
					}
					s.Message.SaveRecord(recv)
				}
			}

		}
	}

	return nil
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
