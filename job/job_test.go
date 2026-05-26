package job

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPurgeProgressJSONRoundTrip(t *testing.T) {
	startedAt := time.Date(2026, 5, 23, 21, 0, 0, 0, time.UTC)
	progress := PurgeProgress{
		JobID:             "job-1",
		GuildID:           42,
		StartedAt:         startedAt,
		CutoffAt:          startedAt.Add(-24 * time.Hour),
		ChannelIDs:        []uint64{100, 200},
		CurrentIndex:      1,
		BeforeID:          12345,
		TotalDeleted:      7,
		PendingChannelID:  200,
		PendingDeleteIDs:  []uint64{333, 444},
		CommandMessageID:  999,
		FallbackChannelID: 100,
		FallbackMessageID: 998,
		UpdatedAt:         startedAt.Add(time.Minute),
		Channels: []PurgeChannelProgress{
			{ChannelID: 100, Deleted: 5, Done: true},
			{ChannelID: 200, Deleted: 2, Error: "missing access"},
		},
	}

	data, err := json.Marshal(progress)
	if err != nil {
		t.Fatalf("marshal progress: %v", err)
	}

	var got PurgeProgress
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal progress: %v", err)
	}

	if got.JobID != progress.JobID ||
		got.GuildID != progress.GuildID ||
		!got.StartedAt.Equal(progress.StartedAt) ||
		!got.CutoffAt.Equal(progress.CutoffAt) ||
		got.CurrentIndex != progress.CurrentIndex ||
		got.BeforeID != progress.BeforeID ||
		got.TotalDeleted != progress.TotalDeleted ||
		got.PendingChannelID != progress.PendingChannelID ||
		got.CommandMessageID != progress.CommandMessageID ||
		got.FallbackChannelID != progress.FallbackChannelID ||
		got.FallbackMessageID != progress.FallbackMessageID ||
		len(got.ChannelIDs) != len(progress.ChannelIDs) ||
		len(got.PendingDeleteIDs) != len(progress.PendingDeleteIDs) ||
		len(got.Channels) != len(progress.Channels) {
		t.Fatalf("progress mismatch after round trip: %#v", got)
	}
	if got.Channels[1].Error != "missing access" {
		t.Fatalf("channel error was not preserved: %#v", got.Channels[1])
	}
}
