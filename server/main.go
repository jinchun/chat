package main

import (
	proto "chatDemo/proto"
	"chatDemo/server/Chat"
	"google.golang.org/grpc"
	"log"
	"net"
)

func main() {
	log.Println("service start...")
	server := grpc.NewServer()
	streamer := &Chat.Stream{}
	proto.RegisterChatServer(server, streamer)
	streamer.Room.FlushRoom("")

	address, err := net.Listen("tcp", ":3020")
	if err != nil {
		panic(err)
	}
	log.Println("service started.")
	go func() {
		log.Println("consume started.")
		err := streamer.Consume()
		if err != nil {
			log.Println("consume err:", err)
		}
		defer streamer.Queue.Close()
	}()
	if err := server.Serve(address); err != nil {
		panic(err)
	}

}
