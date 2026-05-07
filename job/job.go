package job

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	QueueKey      = "purgebot:purge:queue"
	lockKey       = "purgebot:purge:lock:%d"        // %d = guildID
	cancelKey     = "purgebot:purge:cancel:%s"      // %s = jobID
	pendingJobKey = "purgebot:purge:pending:%d"     // %d = guildID
	skipSelectKey = "purgebot:purge:skip_select:%d" // %d = guildID
	pendingTTL    = 5 * time.Minute
)

// unlockScript atomically deletes the lock only if its value matches jobID.
var unlockScript = redis.NewScript(`
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
`)

type PurgeType string

const (
	PurgeTypeUser     PurgeType = "user"
	PurgeTypeRole     PurgeType = "role"
	PurgeTypeEveryone PurgeType = "everyone"
	PurgeTypeInactive PurgeType = "inactive"
	PurgeTypeWebhook  PurgeType = "webhook"
	PurgeTypeDeleted  PurgeType = "deleted"
)

type TargetType string

const (
	TargetTypeServer   TargetType = "server"
	TargetTypeCategory TargetType = "category"
	TargetTypeChannel  TargetType = "channel"
)

type FilterMode string

const (
	FilterModeContains   FilterMode = "contains"
	FilterModeRegex      FilterMode = "regex"
	FilterModeExact      FilterMode = "exact"
	FilterModeStartsWith FilterMode = "starts_with"
	FilterModeEndsWith   FilterMode = "ends_with"
)

type PurgeJob struct {
	ID               string     `json:"id"`
	GuildID          uint64     `json:"guild_id"`
	Locale           string     `json:"locale"`
	TargetID         uint64     `json:"target_id"`
	TargetType       TargetType `json:"target_type"`
	PurgeType        PurgeType  `json:"purge_type"`
	FilterUserID     uint64     `json:"filter_user_id,omitempty"`
	FilterRoleID     uint64     `json:"filter_role_id,omitempty"`
	Days             int        `json:"days,omitempty"`
	Filter           string     `json:"filter,omitempty"`
	FilterMode       FilterMode `json:"filter_mode,omitempty"`
	CaseSensitive    bool       `json:"case_sensitive"`
	IncludeThreads   bool       `json:"include_threads"`
	IncludeBots      bool       `json:"include_bots"`
	SkipChannelIDs   []uint64   `json:"skip_channel_ids,omitempty"`
	InteractionToken string     `json:"interaction_token"`
	ApplicationID    uint64     `json:"application_id"`
	RequestedByID    uint64     `json:"requested_by_id"`
	CreatedAt        time.Time  `json:"created_at"`
}

func Enqueue(ctx context.Context, rdb *redis.Client, j *PurgeJob) error {
	data, err := json.Marshal(j)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	return rdb.LPush(ctx, QueueKey, data).Err()
}

func Dequeue(ctx context.Context, rdb *redis.Client, timeout time.Duration) (*PurgeJob, error) {
	result, err := rdb.BRPop(ctx, timeout, QueueKey).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var j PurgeJob
	if err := json.Unmarshal([]byte(result[1]), &j); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}
	return &j, nil
}

func LockGuild(ctx context.Context, rdb *redis.Client, guildID uint64, jobID string, ttl time.Duration) (bool, error) {
	return rdb.SetNX(ctx, fmt.Sprintf(lockKey, guildID), jobID, ttl).Result()
}

// UnlockGuild releases the guild lock only if it is still held by jobID.
func UnlockGuild(ctx context.Context, rdb *redis.Client, guildID uint64, jobID string) error {
	err := unlockScript.Run(ctx, rdb, []string{fmt.Sprintf(lockKey, guildID)}, jobID).Err()
	if err == redis.Nil {
		return nil
	}
	return err
}

func IsCancelled(ctx context.Context, rdb *redis.Client, jobID string) (bool, error) {
	n, err := rdb.Exists(ctx, fmt.Sprintf(cancelKey, jobID)).Result()
	return n > 0, err
}

func Cancel(ctx context.Context, rdb *redis.Client, jobID string) error {
	return rdb.Set(ctx, fmt.Sprintf(cancelKey, jobID), 1, 30*time.Minute).Err()
}

// StorePendingJob stores a job awaiting skip-channel selection (TTL: 5 min).
func StorePendingJob(ctx context.Context, rdb *redis.Client, j *PurgeJob) error {
	data, err := json.Marshal(j)
	if err != nil {
		return fmt.Errorf("marshal pending job: %w", err)
	}
	return rdb.Set(ctx, fmt.Sprintf(pendingJobKey, j.GuildID), data, pendingTTL).Err()
}

// GetPendingJob retrieves a stored pending job. Returns nil if it has expired.
func GetPendingJob(ctx context.Context, rdb *redis.Client, guildID uint64) (*PurgeJob, error) {
	data, err := rdb.Get(ctx, fmt.Sprintf(pendingJobKey, guildID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var j PurgeJob
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("unmarshal pending job: %w", err)
	}
	return &j, nil
}

// DeletePendingJob removes the pending job and skip selection for a guild.
func DeletePendingJob(ctx context.Context, rdb *redis.Client, guildID uint64) {
	_ = rdb.Del(ctx, fmt.Sprintf(pendingJobKey, guildID), fmt.Sprintf(skipSelectKey, guildID)).Err()
}

// StoreSkipSelection stores the selected channel IDs for a pending skip-channels flow.
func StoreSkipSelection(ctx context.Context, rdb *redis.Client, guildID uint64, channelIDs []uint64) error {
	data, err := json.Marshal(channelIDs)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, fmt.Sprintf(skipSelectKey, guildID), data, pendingTTL).Err()
}

// GetSkipSelection retrieves the selected channel IDs for the pending skip-channels flow.
func GetSkipSelection(ctx context.Context, rdb *redis.Client, guildID uint64) ([]uint64, error) {
	data, err := rdb.Get(ctx, fmt.Sprintf(skipSelectKey, guildID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []uint64
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}
