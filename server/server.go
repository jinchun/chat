package main

import (
	proto "chatDemo/proto"
	"google.golang.org/grpc"
	"io"
	"log"
	"net"
	"time"
)

type client struct {
	Name   string
	Stream proto.Chat_BidStreamServer
}

type Streamer struct {
	Clients map[string]*client
}

func (s *Streamer) BidStream(stream proto.Chat_BidStreamServer) error {
	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			log.Println("ctx done.")
			return ctx.Err()
		default:
			recv, err := stream.Recv()
			if err == io.EOF {
				log.Println("io EOF.")
				return nil
			}
			if err != nil {
				log.Println("recv err:", err)
				return err
			}
			if s.Clients == nil {
				s.Clients = make(map[string]*client)

			}
			if s.Clients[recv.Name] == nil {
				s.Clients[recv.Name] = &client{
					Name:   recv.Name,
					Stream: stream,
				}
			}
			switch recv.Event {
			case "login": //TODO
				if err := stream.Send(&proto.Response{Name: recv.Name, Time: time.Now().Unix(), Content: "login is successful."}); err != nil {
					return err
				}
			default:
				switch recv.Content {
				case "whoami":
					if err := stream.Send(&proto.Response{Name: recv.Name, Time: time.Now().Unix(), Content: "my name is " + recv.Name}); err != nil {
						return err
					}
					return nil
				default:
					log.Printf("[recv]: %s", recv.Content)

					for _, client := range s.Clients {
						if err := client.Stream.Send(&proto.Response{Name: recv.Name, Time: time.Now().Unix(), Content: recv.Content}); err != nil {
							return err
						}
					}
				}
			}
		}
	}
}

func main() {
	log.Println("Start...")
	server := grpc.NewServer()

	proto.RegisterChatServer(server, &Streamer{})

	address, err := net.Listen("tcp", ":3020")
	if err != nil {
		panic(err)
	}

	if err := server.Serve(address); err != nil {
		panic(err)
	}
}
