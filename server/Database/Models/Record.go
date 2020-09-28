package Models

import (
	"time"
)

type Record struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	Room      string    `json:"room"`
	FromUser  string    `json:"from_user"`
	ToUser    string    `json:"to_user"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
