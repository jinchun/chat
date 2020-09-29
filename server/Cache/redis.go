package Cache

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

type Redis struct {
}

var rdb *redis.Client
var ctx context.Context

func init() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:32769",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	ctx = context.Background()

	log.Println("redis init...")
}

func (r *Redis) Set(key string, value interface{}, expr time.Duration) error {
	if err := rdb.Set(ctx, key, value, expr).Err(); err != nil {
		return err
	}
	return nil
}

func (r *Redis) Get(key string) (interface{}, error) {
	value, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (r *Redis) LPush(key string, values ...interface{}) error {
	if err := rdb.LPush(ctx, key, values).Err(); err != nil {
		return err
	}
	return nil
}

func (r *Redis) LPop(key string) (string, error) {
	value, err := rdb.LPop(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return value, nil
}

func (r *Redis) LRange(key string, start, stop int64) ([]string, error) {
	value, err := rdb.LRange(ctx, key, start, stop).Result()
	if err != nil {
		return []string{}, err
	}
	return value, nil
}

func (r *Redis) HMSet(key string, values ...interface{}) error {
	if err := rdb.HMSet(ctx, key, values).Err(); err != nil {
		return err
	}
	return nil
}

func (r *Redis) HMGet(key string, fields ...string) ([]interface{}, error) {
	value, err := rdb.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (r *Redis) HGet(key string, field string) (string, error) {
	value, err := rdb.HGet(ctx, key, field).Result()
	if err != nil {
		return "", err
	}
	return value, nil
}

func (r *Redis) SAdd(key string, members ...interface{}) error {
	if err := rdb.SAdd(ctx, key, members...).Err(); err != nil {
		return err
	}
	return nil
}

func (r *Redis) SMembers(key string) ([]string, error) {
	value, err := rdb.SMembers(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (r *Redis) SIsMember(key string, member interface{}) bool {
	value, err := rdb.SIsMember(ctx, key, member).Result()
	if err != nil {
		return false
	}
	return value
}

func (r *Redis) SRem(key string, members ...interface{}) error {
	if err := rdb.SRem(ctx, key, members...).Err(); err != nil {
		return err
	}
	return nil
}
