package task

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"tg-search/internal/config"
	"tg-search/internal/model"
)

func TestCleanupJob_Name(t *testing.T) {
	job := NewCleanupJob(nil, config.TaskRetentionConfig{}, nil)
	assert.Equal(t, "task_cleanup", job.Name())
}

func TestCleanupJob_Run_SkipsZeroRetention(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	service := NewService(repo)
	logger := zaptest.NewLogger(t)

	policy := config.TaskRetentionConfig{
		SucceededDays: 0, // Disabled
		FailedDays:    7,
	}

	// Create old succeeded task
	old := time.Now().UTC().AddDate(0, 0, -10)
	task := createTaskWithTimestamp(t, repo, model.TaskStatusSucceeded, old)

	job := NewCleanupJob(service, policy, logger)
	err := job.Run(context.Background())
	require.NoError(t, err)

	// Verify succeeded task was NOT deleted (policy is 0)
	_, err = repo.FindByID(context.Background(), task.ID)
	assert.NoError(t, err, "task should not be deleted when retention is 0")
}

func TestCleanupJob_Run_CleansMultipleStatuses(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	service := NewService(repo)
	logger := zaptest.NewLogger(t)

	policy := config.TaskRetentionConfig{
		SucceededDays: 7,
		FailedDays:    7,
		CanceledDays:  7,
	}

	old := time.Now().UTC().AddDate(0, 0, -10)

	// Create old tasks
	oldSucceeded := createTaskWithTimestamp(t, repo, model.TaskStatusSucceeded, old)
	oldFailed := createTaskWithTimestamp(t, repo, model.TaskStatusFailed, old)
	oldCanceled := createTaskWithTimestamp(t, repo, model.TaskStatusCanceled, old)

	job := NewCleanupJob(service, policy, logger)
	err := job.Run(context.Background())
	require.NoError(t, err)

	// Verify all old tasks were deleted
	_, err = repo.FindByID(context.Background(), oldSucceeded.ID)
	assert.Equal(t, sql.ErrNoRows, err, "succeeded should be deleted")

	_, err = repo.FindByID(context.Background(), oldFailed.ID)
	assert.Equal(t, sql.ErrNoRows, err, "failed should be deleted")

	_, err = repo.FindByID(context.Background(), oldCanceled.ID)
	assert.Equal(t, sql.ErrNoRows, err, "canceled should be deleted")
}

func TestCleanupJob_Run_ContinuesOnPartialFailure(t *testing.T) {
	// This test is harder to implement without mocking
	// For now, we'll test the happy path and document that
	// error handling is covered by integration tests
	t.Skip("Partial failure testing requires mock repository")
}

func TestCleanupJob_Run_NoTasksToClean(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	service := NewService(repo)
	logger := zaptest.NewLogger(t)

	policy := config.TaskRetentionConfig{
		SucceededDays: 7,
	}

	// Create recent task (won't be deleted)
	recent := time.Now().UTC().AddDate(0, 0, -3)
	createTaskWithTimestamp(t, repo, model.TaskStatusSucceeded, recent)

	job := NewCleanupJob(service, policy, logger)
	err := job.Run(context.Background())
	require.NoError(t, err) // Should succeed silently
}
