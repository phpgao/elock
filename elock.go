package elock

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const (
	defaultMaxHeldTime  = 1 * time.Hour
	defaultMonitorTicks = 1 * time.Minute
	defaultPrefix       = "elock"
)

type Job interface {
	Run()
}

type ELocker struct {
	client          *clientv3.Client
	locks           map[string]*lockSession
	monitorTicks    time.Duration
	maxHeldTime     time.Duration
	timeoutCallback func(string, time.Duration)
	randomSleep     func()
	mu              sync.Mutex
	prefix          string
	successCallback func(string)
}

type lockSession struct {
	acquiredAt time.Time
	lock       *concurrency.Mutex
	session    *concurrency.Session
}

func randomSleep(min, max time.Duration) {
	if min == max {
		time.Sleep(min)
		return
	}

	if min > max {
		min, max = max, min
	}
	delta := max - min
	randSleep := time.Duration(rand.Int63n(int64(delta))) + min
	time.Sleep(randSleep)
}

type Opt func(*ELocker)

// WithMaxHeldTime is an option to set the maximum duration a lock can be held
// before it is automatically released.
// Only used in Lock/Unlock.
func WithMaxHeldTime(maxHeldTime time.Duration) Opt {
	return func(e *ELocker) {
		e.maxHeldTime = maxHeldTime
	}
}

// WithMonitorTicks is an option to set the duration between lock monitor ticks.
func WithMonitorTicks(monitorTicks time.Duration) Opt {
	return func(e *ELocker) {
		e.monitorTicks = monitorTicks
	}
}

// WithTimeoutCallback is an option to set a callback function that will
// be called when a lock is automatically released after the maximum held duration.
func WithTimeoutCallback(timeoutFunc func(string, time.Duration)) Opt {
	return func(e *ELocker) {
		e.timeoutCallback = timeoutFunc
	}
}

// WithRandomSleep is an option to set the random sleep between lock acquisitions.
// Only used in RunInLock.
func WithRandomSleep(min, max time.Duration) Opt {
	return func(e *ELocker) {
		e.randomSleep = func() {
			randomSleep(min, max)
		}
	}
}

func WithPrefix(prefix string) Opt {
	return func(e *ELocker) {
		e.prefix = prefix
	}
}

func WithSuccessCallback(successCallback func(string)) Opt {
	return func(e *ELocker) {
		e.successCallback = successCallback
	}
}

// NewELocker creates a new ELocker instance with the given etcd client and options.
func NewELocker(client *clientv3.Client, opts ...Opt) *ELocker {
	e := &ELocker{
		client:       client,
		locks:        make(map[string]*lockSession),
		maxHeldTime:  defaultMaxHeldTime,
		mu:           sync.Mutex{},
		monitorTicks: defaultMonitorTicks,
		prefix:       defaultPrefix,
	}
	for _, o := range opts {
		o(e)
	}
	// run monitor
	if e.timeoutCallback != nil {
		go e.MonitorLocks()
	}
	return e
}

// Lock attempts to acquire a lock with the specified key and TTL (time-to-live).
func (e *ELocker) Lock(ctx context.Context, key string, ttl int) error {
	keyInLock := e.KeyPrefix(key)
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.locks[key]; exists {
		return fmt.Errorf("key already locked: %s", key)
	}

	//session will keepalive when lock is held
	session, err := concurrency.NewSession(e.client, concurrency.WithTTL(ttl))
	if err != nil {
		return err
	}

	lock := concurrency.NewMutex(session, keyInLock)

	if err := lock.TryLock(ctx); err != nil {
		session.Close()
		return err
	}

	if e.successCallback != nil {
		e.successCallback(key)
	}

	e.locks[key] = &lockSession{
		acquiredAt: time.Now(),
		lock:       lock,
		session:    session,
	}

	return nil
}

// Unlock releases the lock associated with the specified key.
func (e *ELocker) Unlock(ctx context.Context, key string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	ls, exists := e.locks[key]
	if !exists {
		return fmt.Errorf("lock not found for key: %s", key)
	}

	if err := ls.lock.Unlock(ctx); err != nil {
		return err
	}

	defer func() {
		ls.session.Close()
		delete(e.locks, key)
	}()

	return nil
}

// MonitorLocks runs a loop that periodically checks all acquired locks
// and releases those that have exceeded the maximum held duration.
func (e *ELocker) MonitorLocks() {
	ticker := time.NewTicker(e.monitorTicks)
	defer ticker.Stop()

	for range ticker.C {
		for key, ls := range e.locks {
			lockDuration := time.Since(ls.acquiredAt)
			if lockDuration > e.maxHeldTime {
				go e.timeoutCallback(key, lockDuration)
				e.Unlock(context.TODO(), key)
			}
		}
	}
}

func (e *ELocker) KeyPrefix(key string) string {
	return fmt.Sprintf("/%s/%s", e.prefix, key)
}

// RunJobInLock executes a function while holding a lock with the specified key and TTL.
func (e *ELocker) RunJobInLock(ctx context.Context, key string, ttl int, job Job) (time.Duration, error) {
	if job == nil {
		return 0, fmt.Errorf("job cannot be nil")
	}
	return e.RunFuncInLock(ctx, key, ttl, job.Run)
}

// RunFuncInLock executes a function while holding a lock with the specified key and TTL.
func (e *ELocker) RunFuncInLock(ctx context.Context, key string, ttl int, fn func()) (time.Duration, error) {
	if fn == nil {
		return 0, fmt.Errorf("function cannot be nil")
	}
	keyInLock := e.KeyPrefix(key)

	session, err := concurrency.NewSession(e.client, concurrency.WithTTL(ttl))
	if err != nil {
		return 0, err
	}
	defer session.Close()

	lock := concurrency.NewMutex(session, keyInLock)

	if err := lock.TryLock(ctx); err != nil {
		return 0, err
	}
	defer lock.Unlock(ctx)

	if e.successCallback != nil {
		e.successCallback(key)
	}

	if e.randomSleep != nil {
		e.randomSleep()
	}

	startTime := time.Now()
	fn()
	duration := time.Since(startTime)

	return duration, nil
}

// Close closes the ELocker client.
func (e *ELocker) Close() error {
	return e.client.Close()
}
