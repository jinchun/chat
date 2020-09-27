package Streamer

import (
	proto "chatDemo/proto"
	"chatDemo/server/Rabbit"
	"encoding/json"
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
}

const defaultRoom = "defaultRoom"

// get all rooms.
func (s *Streamer) GetRooms() map[string]*Room {
	return map[string]*Room{defaultRoom: {
		Name:    defaultRoom,
		Members: nil,
	}}
}

func (s *Streamer) GetMemberByUser(name string) *Client {
	return s.Clients[name]
}

func (s *Streamer) JoinRoom(roomName string, fromUser string, stream proto.Chat_BidStreamServer) {
	if s.Rooms == nil {
		s.Rooms = make(map[string]*Room)
	}
	if s.Rooms[roomName] == nil {
		s.Rooms[roomName] = &Room{}
	}
	if s.Rooms[roomName].Members == nil {
		s.Rooms[roomName].Members = make(map[string]*Member)
	}
	if s.Rooms[roomName].Members[fromUser] == nil {
		s.Rooms[roomName].Members[fromUser] = &Member{
			Name: fromUser,
		}
	}
	if s.Clients == nil {
		s.Clients = make(map[string]*Client)
	}
	if s.Clients[fromUser] == nil {
		s.Clients[fromUser] = &Client{
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
		return "recv err:", err
	}

	roomName := recv.RoomName
	if roomName == "" {
		roomName = defaultRoom
	}

	if recv.Event == "login" {
		s.JoinRoom(roomName, recv.FromUser, stream)
	}

	recvObj := map[string]interface{}{
		"FromUser": recv.FromUser,
		"ToUser":   recv.ToUser,
		"RoomName": recv.RoomName,
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
		curUser := s.GetMemberByUser(recv.FromUser)
		switch recv.Event {
		case "login": //TODO
			if err := curUser.Stream.Send(&proto.Response{FromUser: recv.FromUser, Time: time.Now().Unix(), Content: "login is successful."}); err != nil {
				return err
			}
		default:
			switch recv.Content {
			case "whoami":
				if err := curUser.Stream.Send(&proto.Response{FromUser: recv.FromUser, Time: time.Now().Unix(), Content: "my name is " + recv.FromUser}); err != nil {
					return err
				}
				return nil
			default:
				log.Printf("[recv]: %s", recv.Content)

				for _, client := range s.Clients {
					if err := client.Stream.Send(&proto.Response{FromUser: recv.FromUser, Time: time.Now().Unix(), Content: recv.Content}); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
