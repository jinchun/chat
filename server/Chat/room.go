package Chat

import (
	"chatDemo/server/Cache"
	"fmt"
	"log"
)

type Room struct {
	*Stream
	User
	Cache Cache.Redis
}

type Member struct {
	Name     string
	Status   int8
	JoinedAt int64
}

const defaultRoom = "defaultRoom"

func (r *Room) MapKey(roomName string) string {
	return fmt.Sprintf("room_%s", roomName)
}

func (r *Room) getRoomName(orgRoomName string) string {
	if orgRoomName == "" {
		return defaultRoom
	}
	return orgRoomName
}

func (r *Room) JoinRoom(roomName string, fromUser string) {
	log.Printf("%s 进入了 %s", fromUser, roomName)

	if err := r.Cache.SAdd(r.MapKey(roomName), fromUser); err != nil {
		log.Println("join room failed.", err.Error())
		return
	}
}

func (r *Room) LeftRoom(roomName string, fromUser string) bool {
	if r.Cache.SIsMember(r.MapKey(roomName), fromUser) {
		log.Printf("%s 离开了 %s", fromUser, roomName)

		if err := r.Cache.SRem(r.MapKey(roomName), fromUser); err != nil {
			log.Println("left room failed.", err.Error())
			return false
		}
		return true
	}
	return false
}

func (r *Room) FlushRoom(roomName string) error {
	log.Println("reset room data...")
	roomMapKey := r.MapKey(r.getRoomName(roomName))
	members, err := r.Cache.SMembers(roomMapKey)
	if err != nil {
		return err
	}
	if err := r.Cache.SRem(roomMapKey, members); err != nil {
		return err
	}
	return nil
}
