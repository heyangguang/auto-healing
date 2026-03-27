package main

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunHTTPServerInvokesShutdownHook(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var shutdownCalled atomic.Bool
	server := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: http.NewServeMux(),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- runHTTPServer(ctx, server, time.Second, func() {
			shutdownCalled.Store(true)
		})
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runHTTPServer() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runHTTPServer() did not stop in time")
	}

	if !shutdownCalled.Load() {
		t.Fatal("shutdown hook was not called")
	}
}

func TestRunStartupJobReturnsErrorForRequiredJob(t *testing.T) {
	err := runStartupJob(startupJob{
		name:     "required job",
		required: true,
		run: func() error {
			return errors.New("boom")
		},
	})
	if err == nil {
		t.Fatal("runStartupJob() error = nil, want required job failure")
	}
}

func TestRunStartupJobSwallowsOptionalJobError(t *testing.T) {
	err := runStartupJob(startupJob{
		name:     "optional job",
		required: false,
		run: func() error {
			return errors.New("boom")
		},
	})
	if err != nil {
		t.Fatalf("runStartupJob() error = %v, want nil for optional job", err)
	}
}

func TestRunStartupJobsReturnsErrorWhenDictionarySeedFails(t *testing.T) {
	restore := stubStartupJobs(func() []startupJob { return nil })
	defer restore()

	err := runStartupJobsWithDeps(context.Background(), startupDeps{
		dictionary: fakeDictionarySeeder{err: errors.New("dict failed")},
		cleaner:    fakeSiteMessageCleaner{},
	})
	if err == nil {
		t.Fatal("runStartupJobs() error = nil, want dictionary seed failure")
	}
}

func TestRunStartupJobsIgnoresCleanExpiredFailure(t *testing.T) {
	restore := stubStartupJobs(func() []startupJob { return nil })
	defer restore()

	if err := runStartupJobsWithDeps(context.Background(), startupDeps{
		dictionary: fakeDictionarySeeder{},
		cleaner:    fakeSiteMessageCleaner{err: errors.New("clean failed")},
	}); err != nil {
		t.Fatalf("runStartupJobs() error = %v, want nil", err)
	}
}

type fakeDictionarySeeder struct {
	err error
}

func (f fakeDictionarySeeder) SeedDictionaries(context.Context) error {
	return f.err
}

type fakeSiteMessageCleaner struct {
	err error
}

func (f fakeSiteMessageCleaner) CleanExpired(context.Context) (int64, error) {
	return 0, f.err
}

func stubStartupJobs(jobs func() []startupJob) func() {
	oldJobs := listStartupSeedJobs

	listStartupSeedJobs = jobs

	return func() {
		listStartupSeedJobs = oldJobs
	}
}
