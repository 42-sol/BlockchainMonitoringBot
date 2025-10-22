package databaseconnector

import (
	"log"
	"os"

	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username string `gorm:"unique;not null"`
	ChatId   uint   `gorm:"default:null"`
}

type Node struct {
	gorm.Model
	Key             string `gorm:"unique;not null"`
	Uri             string `gorm:"unique;not null"`
	IsReportedToday bool
}

var database *gorm.DB

func ResolveDbConnection() (*gorm.DB, error) {
	sqliteDatabase := os.Getenv("SQLITE_DB") + ".db"

	db, err := gorm.Open(gormlite.Open(sqliteDatabase), &gorm.Config{})
	if err != nil {
		log.Println(err)
		return nil, err
	}

	//if !db.Migrator().HasTable("users") {
	err = db.AutoMigrate(&User{}, &Node{})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	//}

	database = db

	return db, nil
}

func GetDBInstance() *gorm.DB {
	return database
}
