package main

import (
	proto "chatDemo/proto"
	"chatDemo/server/Rabbit"
	"chatDemo/server/Streamer"
	"google.golang.org/grpc"
	"log"
	"net"
)

func main() {
	log.Println("Start...")

	server := grpc.NewServer()

	streamer := &Streamer.Streamer{Rabbit: &Rabbit.Rabbit{}}

	proto.RegisterChatServer(server, streamer)

	streamer.Rabbit.Con()
	go func() {
		err := streamer.Consume()
		if err != nil {
			log.Println(err)
		}
	}()

	address, err := net.Listen("tcp", ":3020")
	if err != nil {
		panic(err)
	}

	if err := server.Serve(address); err != nil {
		panic(err)
	}

}
