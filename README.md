# common

Shared library for PurgeBot services. Contains the purge job definition, Redis queue helpers, and logging setup.

## Packages

### `job`

Purge job type, Redis queue operations, and guild locking.

```go
import "github.com/PurgeBot-net/common/job"

// Enqueue a job from the interactions service
err := job.Enqueue(ctx, rdb, &job.PurgeJob{ ... })

// Dequeue a job in the purge worker (blocks up to timeout)
j, err := job.Dequeue(ctx, rdb, 5*time.Second)

// Acquire a per-guild lock (one active purge per guild)
locked, err := job.LockGuild(ctx, rdb, guildID, jobID)
job.UnlockGuild(ctx, rdb, guildID, jobID)

// Cancel a running job
job.Cancel(ctx, rdb, jobID)
job.IsCancelled(ctx, rdb, jobID)
```

### `log`

Zap logger with optional Sentry integration.

```go
import "github.com/PurgeBot-net/common/log"

logger, err := log.New(os.Getenv("LOG_LEVEL"), os.Getenv("LOG_JSON") == "true")
logger = log.WithSentry(logger, os.Getenv("SENTRY_DSN"))
```

### `rdb`

Redis client constructor with a startup ping.

```go
import "github.com/PurgeBot-net/common/rdb"

client, err := rdb.New(
    os.Getenv("REDIS_ADDR"),
    os.Getenv("REDIS_PASSWORD"),
    0,
)
```
