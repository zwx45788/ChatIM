// pkg/database/mysql.go
package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// InitDB 初始化数据库连接
func InitDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 配置连接池参数
	db.SetMaxOpenConns(100)                 // 最大100个连接
	db.SetMaxIdleConns(20)                  // 保持20个空闲连接
	db.SetConnMaxLifetime(time.Hour)        // 1小时后回收
	db.SetConnMaxIdleTime(10 * time.Minute) // 空闲连接10分钟后回收

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection established successfully")
	log.Printf("✅ Connection pool: MaxOpen=100, MaxIdle=20, MaxLifetime=1h, MaxIdleTime=10m")
	return db, nil
}
