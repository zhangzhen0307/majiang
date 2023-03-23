package main

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var db = newDb()

func newDb() *gorm.DB {
	dsn := Config.MysqlConfig.Dsn
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("mysql open failed")
	}
	return db
}

func GetDb() *gorm.DB {
	return db
}

var red = newRedis()

func newRedis() *redis.Client {
	var red = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	return red
}

func GetRedis() *redis.Client {
	return red
}
