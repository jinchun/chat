package Chat

import (
	"fmt"
)

type User struct {
}

func (u *User) MapKey(name string) string {
	return fmt.Sprintf("member_%s", name)
}
