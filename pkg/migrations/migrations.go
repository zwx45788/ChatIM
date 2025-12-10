package migrations

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RunMigrations 运行所有待执行的迁移
func RunMigrations(db *sql.DB) error {
	// 1. 创建 schema_migrations 表（如果不存在）
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`
	if _, err := db.Exec(createTableSQL); err != nil {
		log.Printf("Failed to create schema_migrations table: %v", err)
		return err
	}

	// 2. 获取所有迁移文件
	migrationFiles, err := getMigrationFiles()
	if err != nil {
		log.Printf("Failed to read migration files: %v", err)
		return err
	}

	// 3. 对每个迁移文件执行
	for _, file := range migrationFiles {
		version := strings.TrimSuffix(filepath.Base(file), ".sql")

		// 检查该迁移是否已执行
		executed, err := isMigrationExecuted(db, version)
		if err != nil {
			log.Printf("Failed to check migration status for %s: %v", version, err)
			return err
		}

		if executed {
			log.Printf("✓ Migration %s already executed, skipping", version)
			continue
		}

		// 执行迁移
		log.Printf("→ Running migration: %s", version)
		if err := executeMigrationFile(db, file); err != nil {
			log.Printf("✗ Migration %s failed: %v", version, err)
			return err
		}

		log.Printf("✓ Migration %s executed successfully", version)
	}

	log.Println("✓ All migrations completed successfully")
	return nil
}

// getMigrationFiles 获取所有 migration SQL 文件
func getMigrationFiles() ([]string, error) {
	var files []string

	// 尝试多个可能的路径
	possiblePaths := []string{
		"./migrations",
		"./pkg/migrations",
		"/root/migrations",
	}

	var migrationsDir string
	for _, path := range possiblePaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			migrationsDir = path
			break
		}
	}

	if migrationsDir == "" {
		log.Println("Warning: migrations directory not found, skipping migrations")
		return files, nil
	}

	entries, err := ioutil.ReadDir(migrationsDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, filepath.Join(migrationsDir, entry.Name()))
		}
	}

	// 按版本号排序
	sort.Strings(files)
	return files, nil
}

// isMigrationExecuted 检查迁移是否已执行
func isMigrationExecuted(db *sql.DB, version string) (bool, error) {
	var exists int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM schema_migrations WHERE version = ?",
		version,
	).Scan(&exists)

	if err != nil {
		// 如果表不存在，返回 false
		if strings.Contains(err.Error(), "no such table") {
			return false, nil
		}
		return false, err
	}

	return exists > 0, nil
}

// executeMigrationFile 执行单个迁移文件
func executeMigrationFile(db *sql.DB, filePath string) error {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %v", filePath, err)
	}

	// 按 ; 分割 SQL 语句（支持多条 SQL）
	statements := strings.Split(string(content), ";")

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// 执行 SQL 语句
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute SQL statement in %s: %v", filePath, err)
		}
	}

	return nil
}
