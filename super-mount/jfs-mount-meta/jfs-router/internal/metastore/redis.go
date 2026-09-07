package metastore

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisStore 用一个 Redis Hash 存储整张路由表：
//
//	HSET <key> tenantA /mnt/.jfs-tenantA
//	HSET <key> tenantB /mnt/.jfs-tenantB
//
// 字段名（field）即路由 key（虚拟路径的顶层分段），值就是 backend 目录。
// 选择单个 Hash 而不是"每个 tenant 一个独立 key"，是因为：
//  1. List()（用于 `ls` 挂载根目录）可以用一次 HGETALL 完成，不需要
//     SCAN 匹配前缀（SCAN 在 key 数量大、且和其他业务共用一个 Redis
//     实例时，对生产环境不够友好）。
//  2. Get() 是 O(1) 的 HGET，不受 Hash 里其他字段数量影响。
//
// 如果你的团队已经有更复杂的路由元数据模型（比如每个 tenant 还要挂
// 额外属性），可以参照本文件的结构自行扩展一个新的 Store 实现，
// 路由层完全不需要感知这些差异。
type RedisStore struct {
	client *redis.Client
	key    string
}

// RedisConfig 是构造 RedisStore 所需的连接参数。
type RedisConfig struct {
	Addr     string // 例如 "127.0.0.1:6379"
	Password string
	DB       int
	Key      string // 存路由表的 Hash key，例如 "jfs-router:mounts"
}

// NewRedisStore 建立到 Redis 的连接并做一次 PING 探活，确保配置错误
// （地址不通、密码不对）在启动阶段就暴露，而不是等到第一次 FUSE 请求
// 打过去才发现连不上。
func NewRedisStore(ctx context.Context, cfg RedisConfig) (*RedisStore, error) {
	if cfg.Key == "" {
		return nil, errors.New("redis metastore 配置缺少 key（存路由表的 Hash key）")
	}
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("连接 Redis %q 失败: %w", cfg.Addr, err)
	}
	return &RedisStore{client: client, key: cfg.Key}, nil
}

func (s *RedisStore) Get(ctx context.Context, segment string) (Route, bool, error) {
	backend, err := s.client.HGet(ctx, s.key, segment).Result()
	if errors.Is(err, redis.Nil) {
		return Route{}, false, nil
	}
	if err != nil {
		return Route{}, false, fmt.Errorf("HGET %s %s 失败: %w", s.key, segment, err)
	}
	return Route{Backend: backend}, true, nil
}

func (s *RedisStore) List(ctx context.Context) (map[string]Route, error) {
	m, err := s.client.HGetAll(ctx, s.key).Result()
	if err != nil {
		return nil, fmt.Errorf("HGETALL %s 失败: %w", s.key, err)
	}
	out := make(map[string]Route, len(m))
	for seg, backend := range m {
		out[seg] = Route{Backend: backend}
	}
	return out, nil
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}
