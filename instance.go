package elock

import (
	"context"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	instance *ELocker
	once     sync.Once
)

func GetELockerInstance(client *clientv3.Client, opts ...Opt) *ELocker {
	once.Do(func() {
		instance = NewELocker(client, opts...)
	})
	return instance
}

func RunJobInLock(ctx context.Context, key string, ttl int, job Job) (time.Duration, error) {
	return instance.RunJobInLock(ctx, key, ttl, job)
}

func RunFuncInLock(ctx context.Context, key string, ttl int, fn func()) (time.Duration, error) {
	return instance.RunFuncInLock(ctx, key, ttl, fn)
}
