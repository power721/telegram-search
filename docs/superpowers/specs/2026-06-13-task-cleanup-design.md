# Task Auto-Cleanup Mechanism Design

**Date:** 2026-06-13  
**Status:** Approved  
**Author:** Claude Code

## Overview

Implement automatic cleanup of old tasks to prevent unbounded growth of the `sync_tasks` table. The system will periodically delete tasks based on configurable retention policies per task status.

## Problem Statement

Tasks accumulate over time, cluttering the task list and consuming database storage. Users currently have to manually delete old tasks through the UI, which is tedious and error-prone.

## Goals

- Automatically delete old tasks based on status and age
- Configurable retention policies per status type
- Minimal performance impact
- Simple to maintain and troubleshoot

## Non-Goals

- Archive tasks before deletion (YAGNI - tasks are not audit logs)
- Expose cleanup statistics via API (can add later if needed)
- Fine-grained control per task type (retention by status is sufficient)

## Architecture

### Component Integration

```
main.go
  └─> cleanupScheduler (existing, runs every hour)
        ├─> CleanupJob (existing: media cache)
        ├─> SessionCleanupJob (existing: admin sessions)
        └─> TaskCleanupJob (NEW) ← this feature
              ↓
          taskService (pass-through)
              ↓
          taskRepository.DeleteOldTasks(status, days)
```

### Configuration

Extend `config.yaml` with a new `task_retention` section:

```yaml
task_retention:
  succeeded_days: 7       # Completed tasks - short retention
  failed_days: 30         # Failed tasks - longer for troubleshooting
  canceled_days: 7        # Canceled tasks - short retention
  paused_days: 30         # Paused tasks - may need manual intervention
  flood_wait_days: 30     # Rate-limited tasks - may need investigation
  reconnecting_days: 7    # Reconnecting tasks - transient state
```

**Excluded statuses:**
- `queued` — should not be deleted by age; these are pending work
- `running` — actively executing
- `canceling` — in process of cancellation

Setting any `*_days` to `0` disables cleanup for that status.

## Data Layer

### Repository Changes

**File:** `internal/task/repository.go`

Add method:

```go
// DeleteOldTasks deletes tasks of the specified status older than the given days.
// Returns the number of deleted rows.
// Running and canceling tasks are never deleted regardless of age.
func (r *Repository) DeleteOldTasks(ctx context.Context, status string, olderThanDays int) (int64, error) {
    cutoff := time.Now().UTC().AddDate(0, 0, -olderThanDays)
    
    result, err := r.db.ExecContext(ctx, `
        DELETE FROM sync_tasks
        WHERE status = ?
          AND updated_at < ?
          AND status NOT IN (?, ?)`,
        status, cutoff, model.TaskStatusRunning, model.TaskStatusCanceling)
    
    if err != nil {
        return 0, fmt.Errorf("delete old tasks: %w", err)
    }
    
    deleted, err := result.RowsAffected()
    if err != nil {
        return 0, fmt.Errorf("get rows affected: %w", err)
    }
    
    return deleted, nil
}
```

**Why `updated_at` instead of `finished_at`:**
- `paused` and `flood_wait` tasks don't have `finished_at`
- `updated_at` reflects the last activity time, suitable for all statuses

**SQL Safety:**
- The `status NOT IN ('running', 'canceling')` clause is defensive even though we only pass eligible statuses
- Prevents accidental deletion if logic is modified later

## Config Layer

**File:** `internal/config/config.go`

### Config Structure

```go
type Config struct {
    Server        ServerConfig        `yaml:"server" json:"server"`
    Sync          SyncConfig          `yaml:"sync" json:"sync"`
    Storage       StorageConfig       `yaml:"storage" json:"storage"`
    Telegram      TelegramConfig      `yaml:"telegram" json:"telegram"`
    AI            AIConfig            `yaml:"ai" json:"ai"`
    Bot           BotConfig           `yaml:"bot" json:"bot"`
    TaskRetention TaskRetentionConfig `yaml:"task_retention" json:"task_retention"`
}

type TaskRetentionConfig struct {
    SucceededDays    int `yaml:"succeeded_days" json:"succeeded_days"`
    FailedDays       int `yaml:"failed_days" json:"failed_days"`
    CanceledDays     int `yaml:"canceled_days" json:"canceled_days"`
    PausedDays       int `yaml:"paused_days" json:"paused_days"`
    FloodWaitDays    int `yaml:"flood_wait_days" json:"flood_wait_days"`
    ReconnectingDays int `yaml:"reconnecting_days" json:"reconnecting_days"`
}
```

### Default Values

In `defaultConfig()`:

```go
TaskRetention: TaskRetentionConfig{
    SucceededDays:    7,   // Success confirmed, short retention
    FailedDays:       30,  // Need longer for troubleshooting
    CanceledDays:     7,   // User canceled, short retention
    PausedDays:       30,  // May need manual review
    FloodWaitDays:    30,  // May need investigation
    ReconnectingDays: 7,   // Transient state
}
```

### Validation

In `validate()`, add:

```go
if cfg.TaskRetention.SucceededDays < 0 {
    return errors.New("task_retention.succeeded_days must be >= 0")
}
// ... repeat for all fields
```

Zero is valid (disables cleanup for that status). No upper limit needed.

## Scheduler Job

**File:** `internal/task/cleanup.go` (new file)

```go
package task

import (
    "context"
    "errors"
    
    "go.uber.org/zap"
    
    "tg-search/internal/config"
    "tg-search/internal/model"
)

type CleanupJob struct {
    service *Service
    policy  config.TaskRetentionConfig
    logger  *zap.Logger
}

func NewCleanupJob(service *Service, policy config.TaskRetentionConfig, logger *zap.Logger) CleanupJob {
    return CleanupJob{
        service: service,
        policy:  policy,
        logger:  logger,
    }
}

func (j CleanupJob) Name() string {
    return "task_cleanup"
}

func (j CleanupJob) Run(ctx context.Context) error {
    statusPolicies := map[string]int{
        model.TaskStatusSucceeded:    j.policy.SucceededDays,
        model.TaskStatusFailed:       j.policy.FailedDays,
        model.TaskStatusCanceled:     j.policy.CanceledDays,
        model.TaskStatusPaused:       j.policy.PausedDays,
        model.TaskStatusFloodWait:    j.policy.FloodWaitDays,
        model.TaskStatusReconnecting: j.policy.ReconnectingDays,
    }
    
    totalDeleted := int64(0)
    hasError := false
    
    for status, days := range statusPolicies {
        if days <= 0 {
            continue // 0 means skip cleanup for this status
        }
        
        deleted, err := j.service.repo.DeleteOldTasks(ctx, status, days)
        if err != nil {
            j.logger.Error("task cleanup failed",
                zap.String("status", status),
                zap.Int("retention_days", days),
                zap.Error(err))
            hasError = true
            continue // Continue cleaning other statuses
        }
        
        if deleted > 0 {
            j.logger.Info("tasks cleaned up",
                zap.String("status", status),
                zap.Int("retention_days", days),
                zap.Int64("deleted", deleted))
            totalDeleted += deleted
        }
    }
    
    if totalDeleted > 0 {
        j.logger.Info("task cleanup completed", zap.Int64("total_deleted", totalDeleted))
    }
    
    if hasError {
        return errors.New("task cleanup completed with errors")
    }
    
    return nil
}
```

**Key Design Decisions:**

1. **Continue on error:** If one status fails to clean, continue with others
2. **Silent when nothing to clean:** Avoids log noise
3. **Per-status logging:** Makes troubleshooting easier
4. **Total summary:** Single line showing total impact

## Integration

**File:** `cmd/tg-search/main.go`

In the `cleanupScheduler` jobs array, add:

```go
cleanupScheduler := scheduler.New(scheduler.Options{
    Interval: time.Hour,
    Jobs: []scheduler.Job{
        scheduler.CleanupJob{Logger: logs.App, MediaCache: imageCache},
        adminauth.SessionCleanupJob{Service: adminAuth},
        taskpkg.NewCleanupJob(taskService, cfg.TaskRetention, logs.App), // NEW
    },
    Logger: logs.App,
})
```

## Error Handling

### Database Errors

- **Locked database:** Logged and retried in next cycle
- **Connection lost:** Logged and retried in next cycle
- **Constraint violations:** Should not occur (no foreign keys on sync_tasks)

### Partial Failures

If cleanup fails for one status (e.g., `succeeded`), the job continues cleaning other statuses and returns an error at the end. Scheduler will log the error but won't crash.

### Edge Cases

1. **Config value is 0:** Skip that status (handled with `if days <= 0`)
2. **No tasks to delete:** Silent return, no log spam
3. **Running/canceling tasks:** Protected by SQL `WHERE status NOT IN`
4. **Queued tasks:** Not in cleanup map, will never be deleted by age

## Testing Strategy

### Unit Tests

**File:** `internal/task/repository_test.go`

Test `DeleteOldTasks`:

```go
func TestRepository_DeleteOldTasks(t *testing.T) {
    // Setup: create tasks with different statuses and ages
    // - succeeded: 10 days old (should delete with 7-day policy)
    // - succeeded: 5 days old (should keep with 7-day policy)
    // - running: 30 days old (should never delete)
    // - canceling: 30 days old (should never delete)
    // - failed: 20 days old (should delete with 15-day policy)
    
    // Assert: deleted count matches expected
    // Assert: remaining tasks are correct
}
```

**File:** `internal/task/cleanup_test.go`

Test `CleanupJob`:

```go
func TestCleanupJob_Run(t *testing.T) {
    t.Run("skips status with zero retention", ...)
    t.Run("cleans multiple statuses", ...)
    t.Run("continues on partial failure", ...)
    t.Run("returns error when any cleanup fails", ...)
}
```

### Integration Testing

Manual testing steps:

1. Start application with default config
2. Create test tasks via API with various statuses
3. Manually set `updated_at` in database to simulate old tasks:
   ```sql
   UPDATE sync_tasks SET updated_at = datetime('now', '-10 days') WHERE id = ?
   ```
4. Wait for scheduler cycle or restart application
5. Verify tasks are deleted according to policy
6. Check logs for cleanup messages

### Test Data Helpers

Add to `internal/task/service_test.go`:

```go
func createTaskWithAge(t *testing.T, repo *Repository, status string, daysOld int) model.Task {
    task := model.Task{
        Type:        model.TaskTypeHistorySync,
        Status:      status,
        PayloadJSON: "{}",
    }
    created, err := repo.Create(context.Background(), task)
    require.NoError(t, err)
    
    // Backdate the task
    updatedAt := time.Now().UTC().AddDate(0, 0, -daysOld)
    _, err = repo.db.Exec(`UPDATE sync_tasks SET updated_at = ? WHERE id = ?`, updatedAt, created.ID)
    require.NoError(t, err)
    
    return created
}
```

## Observability

### Logging

**Normal cleanup:**
```
INFO  tasks cleaned up  status=succeeded retention_days=7 deleted=42
INFO  tasks cleaned up  status=failed retention_days=30 deleted=5
INFO  task cleanup completed  total_deleted=47
```

**No tasks to clean (silent):**
```
(no output - avoids log spam)
```

**Cleanup failure:**
```
ERROR task cleanup failed  status=succeeded retention_days=7 error="database is locked"
```

**Partial failure:**
```
INFO  tasks cleaned up  status=succeeded retention_days=7 deleted=42
ERROR task cleanup failed  status=failed retention_days=30 error="database is locked"
INFO  task cleanup completed  total_deleted=42
```

### Metrics (Future Enhancement)

If Prometheus metrics are added later:
- `task_cleanup_deleted_total{status}` — Counter of deleted tasks per status
- `task_cleanup_errors_total{status}` — Counter of cleanup failures
- `task_cleanup_duration_seconds` — Histogram of cleanup duration

## Performance Impact

### Expected Load

- **Frequency:** Once per hour
- **Operations:** 6 DELETE queries (one per status, skipping if 0)
- **Typical volume:** 50-200 tasks per hour
- **Execution time:** < 100ms total

### Database Considerations

**Current schema:** Tasks are queried by `status` frequently, so an index might help:

```sql
CREATE INDEX IF NOT EXISTS idx_sync_tasks_status_updated_at 
ON sync_tasks(status, updated_at);
```

**When to add:**
- If task table exceeds 100k rows
- If cleanup takes > 500ms
- Monitor with `EXPLAIN QUERY PLAN` during testing

**Impact on inserts:** Minimal - tasks are created infrequently compared to reads

## Operations

### Configuration Changes

**View current policy:**
```bash
grep -A 6 "task_retention:" config.yaml
```

**Adjust retention:**
1. Edit `config.yaml`
2. Restart application
3. Verify in logs: `INFO tg-search starting ...`

**Disable cleanup for a status:**
```yaml
task_retention:
  succeeded_days: 0  # Disables cleanup
```

**Emergency: Disable all cleanup:**
Set all `*_days` to 0 and restart.

### Manual Cleanup (Not Recommended)

Direct SQL if needed:
```sql
DELETE FROM sync_tasks 
WHERE status = 'succeeded' 
  AND updated_at < datetime('now', '-7 days');
```

Better: Add a management API endpoint in future if manual cleanup is frequently needed.

## Migration Path

No database migration needed - this is purely additive.

**Deployment steps:**
1. Deploy code changes
2. Add `task_retention` section to `config.yaml` (or rely on defaults)
3. Restart application
4. Monitor logs for first cleanup cycle (within 1 hour)

**Rollback:**
Remove `TaskCleanupJob` from main.go and restart.

## Future Enhancements

1. **Management API:** Trigger cleanup on demand via `/api/admin/tasks/cleanup`
2. **Cleanup history:** Store last cleanup time in `maintenance` table
3. **Per-type policies:** Different retention for `backup` vs `history_sync` tasks
4. **Archive before delete:** Export to JSON before deletion
5. **Metrics:** Prometheus counters for monitoring

## Security Considerations

- No user input involved - policies are config-driven
- No SQL injection risk - parameterized queries
- No authorization needed - cleanup is an internal operation

## References

- Existing scheduler: `internal/scheduler/scheduler.go`
- Task model: `internal/model/model.go` (lines 304-313)
- Similar cleanup: `internal/adminauth/cleanup.go` (session cleanup pattern)
