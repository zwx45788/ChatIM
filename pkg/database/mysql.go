package database

import (
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql" // 注意这里的匿名导入
)

func InitDB() (*sql.DB, error) {
	dsn := "root:060629@tcp(127.0.0.1:3306)/chatim?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	log.Println("Successfully connected to the MySQL database!")
	return db, nil
}
