package elock_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/phpgao/elock"
)

func TestELocker_LockUnlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client1, err := clientv3.NewFromURL("localhost:2379")
	assert.NoError(t, err)

	client2, err := clientv3.NewFromURL("localhost:2379")
	assert.NoError(t, err)

	locker1 := elock.NewELocker(client1)
	defer locker1.Close()
	locker2 := elock.NewELocker(client2)
	defer locker2.Close()

	testKey := "testkey"

	err = locker1.Lock(ctx, testKey, 5)
	assert.NoError(t, err, "Client 1 should acquire the lock")

	lockCh := make(chan error)
	go func() {
		err := locker2.Lock(ctx, testKey, 5)
		lockCh <- err
	}()

	select {
	case err := <-lockCh:
		assert.Error(t, err, "Client 2 should fail to acquire the lock")
	case <-time.After(1 * time.Second):
	}

	err = locker1.Unlock(ctx, testKey)
	assert.NoError(t, err, "Client 1 should release the lock")

	err = locker2.Lock(ctx, testKey, 5)
	assert.NoError(t, err, "Client 2 should acquired the lock")

	err = locker2.Unlock(ctx, testKey)
	assert.NoError(t, err, "Client 2 should release the lock")
}

func TestELocker_RunInLock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client1, err := clientv3.NewFromURL("localhost:2379")
	assert.NoError(t, err)

	client2, err := clientv3.NewFromURL("localhost:2379")
	assert.NoError(t, err)

	locker1 := elock.NewELocker(client1)
	defer locker1.Close()

	locker2 := elock.NewELocker(client2)
	defer locker2.Close()

	longRunningOperation := func() {
		time.Sleep(1 * time.Second)
	}

	key := "test-lock"
	ttl := 5

	done := make(chan error)
	go func() {
		_, err = locker1.RunFuncInLock(ctx, key, ttl, longRunningOperation)
		done <- err

	}()

	go func() {
		time.Sleep(500 * time.Millisecond)
		_, err = locker2.RunFuncInLock(ctx, key, ttl, longRunningOperation)
		assert.Error(t, err, "Expected RunInLock to fail while the lock is already held, but it succeeded")
	}()

	select {
	case err := <-done:
		assert.NoError(t, err, "Expected locker1 to succeed")
	case <-ctx.Done():
		t.Errorf("Test timed out: the lock was not released in time")
	}
}

func TestELocker_RunInLockWithSleep(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	client1, err := clientv3.NewFromURL("localhost:2379")
	assert.NoError(t, err)

	o := elock.WithRandomSleep(2*time.Second, 1*time.Second)
	locker1 := elock.NewELocker(client1, o)

	defer locker1.Close()

	shortRunningOperation := func() {}

	key := "test-lock"
	ttl := 5

	done := make(chan error)
	startTime := time.Now()

	go func() {
		_, err = locker1.RunFuncInLock(ctx, key, ttl, shortRunningOperation)
		done <- err
	}()

	select {
	case err := <-done:
		endtime := time.Now()
		assert.True(t, endtime.Sub(startTime) >= 1*time.Second,
			"Expected RunInLock to sleep for at least 1 seconds")
		assert.True(t, endtime.Sub(startTime) <= 2100*time.Millisecond,
			"Expected RunInLock to sleep for at most 2 seconds")
		assert.NoError(t, err, "Expected locker1 to succeed")
	case <-ctx.Done():
		t.Errorf("Test timed out: the lock was not released in time")
	}
}

func TestELocker_MonitorLocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := clientv3.NewFromURL("localhost:2379")
	assert.NoError(t, err)

	testKey := "testkey"
	locker := elock.NewELocker(client,
		elock.WithMonitorTicks(time.Millisecond*500),
		elock.WithTimeoutCallback(func(k string, d time.Duration) {
			assert.Equal(t, k, testKey)
		}),
		elock.WithSuccessCallback(func(k string) {
			assert.Equal(t, k, testKey)
		}),
		elock.WithMaxHeldTime(1*time.Second),
	)

	defer locker.Close()

	err = locker.Lock(ctx, testKey, 5)
	assert.NoError(t, err, "Client 1 should acquire the lock")

	time.Sleep(2 * time.Second)

	err = locker.Unlock(ctx, "testkey")
	assert.Error(t, err, "Client 1 should fail to release the lock")
}
