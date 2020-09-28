package Database

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type GORM struct {
	DB *gorm.DB
}

func (g *GORM) Init() {
	dsn := "root:12345678@tcp(127.0.0.1:32768)/chat?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database：" + err.Error())
	}

	g.DB = db
}
