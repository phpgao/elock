 
# ELocker: Etcd 分布式锁库

ELocker 是一个使用 etcd 作为后端实现的分布式锁库，用于在分布式系统中进行资源同步。

## 功能特性

- **分布式锁**：ELocker 提供了一个基于 etcd 的分布式锁实现。
- **锁监控**: 自动监控和释放超时的锁。
- **上下文支持**：所有的锁操作都支持上下文（context），允许在操作中实现超时和取消功能。
- **灵活的配置**: 提供多种配置选项，包括最大持有时间、监控间隔和成功回调等。

## 快速开始

### 安装

使用 `go get` 命令安装 ELocker：

```bash
go get github.com/phpgao/elock
```

### 使用

创建一个新的 ELocker 实例，并使用它来进行锁操作：

```go
package main

import (
	"context"
	"time"
	"github.com/your_username/elock"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	// 创建 etcd 客户端
	client, err := clientv3.NewFromURL("localhost:2379")
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// 创建 ELocker 实例
	locker := elock.NewELocker(client)

	// 尝试获取锁
	err = locker.Lock(context.Background(), "my-key", 10)
	if err != nil {
		panic(err)
	}

	// 执行需要同步的操作
	// ...

	// 释放锁
	err = locker.Unlock(context.Background(), "my-key")
	if err != nil {
		panic(err)
	}
}
```

获取锁并执行任务：

```go
duration, err := elocker.RunJobInLock(context.Background(), "resource-key", 10, myJob)
if err != nil {
log.Printf("Error acquiring lock: %s", err)
}
log.Printf("Job ran for %s", duration)
```

## API 参考

### `NewELocker(client *clientv3.Client, opts ...Opt) *ELocker`

创建一个新的 ELocker 实例。

### `Lock(ctx context.Context, key string, ttl int) error`

尝试获取一个锁。如果锁已经被其他实例持有，则返回错误。

### `Unlock(ctx context.Context, key string) error`

释放一个锁。如果锁不存在，则返回错误。

### `RunFuncInLock(ctx context.Context, key string, ttl int, fn func()) (time.Duration, error)`

在持有锁的状态下执行一个函数。函数执行完成后自动释放锁。

### `RunJobInLock(ctx context.Context, key string, ttl int, job Job) (time.Duration, error) {`

在持有锁的状态下执行一个job。函数执行完成后自动释放锁。

### `Close() error`

关闭 ELocker 客户端。

## 高级选项

ELocker 提供了一些配置选项，可以在创建实例时通过选项函数进行设置。

### `WithMaxHeldTime(maxHeldTime time.Duration) Opt`

设置锁的最大持有时间。

### `WithMonitorTicks(monitorTicks time.Duration) Opt`

设置监控协程检查锁状态的时间间隔。

### `WithTimeoutCallback(timeoutFunc func(string, time.Duration)) Opt`

设置一个回调函数，当锁因为超时而被自动释放时调用。

### `WithRandomSleep(min, max time.Duration) Opt`

设置一个回调函数，在RunInLock中获取到锁后随机等待，避免退出过快。

### `WithPrefix(prefix string) Opt`

设置key在etcd中的前缀

### `WithSuccessCallback(successCallback func(string)) Opt `

设置获取锁成功后的回调，可以打印日志

## 贡献

欢迎通过 GitHub 提交问题和拉取请求。

## 许可证

ELocker 采用 MIT 许可证。请查看 `LICENSE` 文件获取更多信息。
