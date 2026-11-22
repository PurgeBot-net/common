# common

Shared library for PurgeBot services. Contains the purge job definition, Redis queue helpers, a Redis client constructor, and logging setup.

## Packages

### `job`

Purge job type, Redis queue operations, guild locking, cancel flags, progress checkpoints, and pending skip-channel selections.

```go
import "github.com/PurgeBot-net/common/job"

// Enqueue a job from the interactions service
now := time.Now()
err := job.Enqueue(ctx, rdb, &job.PurgeJob{ID: job.NewID(guildID, now), CreatedAt: now, ... })

// Dequeue a job in the purge worker (blocks up to timeout)
j, err := job.Dequeue(ctx, rdb, 5*time.Second)

// Claim the guild's active job slot (one active purge per guild);
// releasing it also drops the job's progress checkpoint
claimed, err := job.SetActiveJob(ctx, rdb, j)
job.DeleteActiveJob(ctx, rdb, *j)

// Cancel a job, queued or running (the flag lasts 7 days)
job.Cancel(ctx, rdb, jobID)
job.IsCancelled(ctx, rdb, jobID)

// Build and read the custom ID of a status message's cancel button
customID := job.CancelButtonID(j.ID, j.RequestedByID)
jobID, requestedByID := job.ParseCancelButtonID(customID)

// Find and delete the purge keys of guilds PurgeBot has left
ids, err := job.GuildIDs(ctx, rdb)
err = job.DeleteGuilds(ctx, rdb, ids)
```

### `log`

Zap logger with optional Sentry-compatible error reporting, and a `slog` bridge that routes a library's logs into it with raw payloads redacted. `log.Config` holds `SENTRY_DSN`, `LOG_LEVEL` and `LOG_JSON` for a service's `caarlos0/env` config.

```go
import "github.com/PurgeBot-net/common/log"

logger, err := log.New(cfg.Log.Level, cfg.Log.JSON)
logger = log.WithSentry(logger, cfg.Log.SentryDSN)
client, err := disgo.New(token, bot.WithLogger(log.Slog(logger)))

// Snowflakes are logged as strings
logger.Info("purge started", log.ID("guild_id", guildID))
```

### `rdb`

Redis client constructor with a startup ping. `rdb.Config` holds `REDIS_ADDR`, `REDIS_PASSWORD` and `REDIS_DB`.

```go
import "github.com/PurgeBot-net/common/rdb"

client, err := rdb.New(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
```
