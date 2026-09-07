package metastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLStore 从一张关系型表里读路由，期望的表结构（列名可以通过配置
// 自定义，见 MySQLConfig，这里是默认建议的 DDL）：
//
//	CREATE TABLE mount_routes (
//	    segment VARCHAR(255) NOT NULL PRIMARY KEY,
//	    backend VARCHAR(1024) NOT NULL,
//	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
//	);
//
// segment 即路由 key（虚拟路径的顶层分段），backend 是真实后端目录。
// updated_at 不是必须的，只是方便运维在数据库侧看变更历史，本 Store
// 实现不读取它。
type MySQLStore struct {
	db            *sql.DB
	table         string
	segmentColumn string
	backendColumn string
}

// MySQLConfig 是构造 MySQLStore 所需的参数。
type MySQLConfig struct {
	// DSN 是 database/sql 标准的数据源名，例如：
	// "user:pass@tcp(127.0.0.1:3306)/jfsrouter?parseTime=true"
	DSN string

	// Table/SegmentColumn/BackendColumn 允许复用已有的表结构，
	// 不强制要求表名/列名必须叫 mount_routes/segment/backend。
	Table         string
	SegmentColumn string
	BackendColumn string

	// MaxOpenConns/MaxIdleConns 控制连接池大小。路由层的查询量本身
	// 很低（大部分请求会命中客户端缓存），默认给一个偏小的连接池即可，
	// 避免和业务系统抢占数据库连接数配额。
	MaxOpenConns int
	MaxIdleConns int
}

// NewMySQLStore 建立数据库连接池并做一次 Ping 探活。
func NewMySQLStore(ctx context.Context, cfg MySQLConfig) (*MySQLStore, error) {
	if cfg.DSN == "" {
		return nil, errors.New("mysql metastore 配置缺少 dsn")
	}
	table := cfg.Table
	if table == "" {
		table = "mount_routes"
	}
	segCol := cfg.SegmentColumn
	if segCol == "" {
		segCol = "segment"
	}
	backendCol := cfg.BackendColumn
	if backendCol == "" {
		backendCol = "backend"
	}

	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("打开 MySQL 连接失败: %w", err)
	}
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 4
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 2
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}

	return &MySQLStore{
		db:            db,
		table:         table,
		segmentColumn: segCol,
		backendColumn: backendCol,
	}, nil
}

func (s *MySQLStore) Get(ctx context.Context, segment string) (Route, bool, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ? LIMIT 1",
		s.backendColumn, s.table, s.segmentColumn)
	var backend string
	err := s.db.QueryRowContext(ctx, query, segment).Scan(&backend)
	if errors.Is(err, sql.ErrNoRows) {
		return Route{}, false, nil
	}
	if err != nil {
		return Route{}, false, fmt.Errorf("查询路由 %q 失败: %w", segment, err)
	}
	return Route{Backend: backend}, true, nil
}

func (s *MySQLStore) List(ctx context.Context) (map[string]Route, error) {
	query := fmt.Sprintf("SELECT %s, %s FROM %s", s.segmentColumn, s.backendColumn, s.table)
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询全部路由失败: %w", err)
	}
	defer rows.Close()

	out := make(map[string]Route)
	for rows.Next() {
		var seg, backend string
		if err := rows.Scan(&seg, &backend); err != nil {
			return nil, fmt.Errorf("扫描路由行失败: %w", err)
		}
		out[seg] = Route{Backend: backend}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历路由结果失败: %w", err)
	}
	return out, nil
}

func (s *MySQLStore) Close() error {
	return s.db.Close()
}
