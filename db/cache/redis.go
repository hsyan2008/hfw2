package cache

import (
	"github.com/hsyan2008/hfw/configs"
	"github.com/hsyan2008/hfw/redis"
)

type RedisCache struct {
	redisIns *redis.Client
	key      string
}

func NewRedisCache(redisConfig configs.RedisConfig) (rc *RedisCache, err error) {
	redisIns, err := redis.New(redisConfig)

	if err != nil {
		return
	}

	return &RedisCache{
		redisIns: redisIns,
		key:      "Xorm_Cache",
	}, nil
}

func (rc *RedisCache) Get(key string) (value interface{}, err error) {
	if rc == nil {
		return
	}
	_, err = rc.redisIns.HGet(&value, rc.key, key)

	return
}

func (rc *RedisCache) Put(key string, value interface{}) (err error) {
	if rc == nil {
		return
	}

	return rc.redisIns.HSet(rc.key, key, value)
}

func (rc *RedisCache) Del(key string) (err error) {
	if rc == nil {
		return
	}

	_, err = rc.redisIns.HDel(rc.key, key)

	return
}

func (rc *RedisCache) IsExist(key string) (isExist bool) {
	if rc == nil {
		return
	}

	isExist, _ = rc.redisIns.HExists(rc.key, key)

	return
}

func (rc *RedisCache) Incr(key string, delta int64) (err error) {
	if rc == nil {
		return
	}

	_, err = rc.redisIns.HIncrBy(rc.key, key, delta)

	return
}

func (rc *RedisCache) Decr(key string, delta int64) (err error) {
	if rc == nil {
		return
	}

	_, err = rc.redisIns.HIncrBy(rc.key, key, -delta)

	return
}

func (rc *RedisCache) ClearAll() (err error) {
	if rc == nil {
		return
	}

	_, err = rc.redisIns.Del(rc.key)

	return
}
