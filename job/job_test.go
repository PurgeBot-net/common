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
		TotalDeleted:      7,
		CommandMessageID:  999,
		FallbackChannelID: 100,
		FallbackMessageID: 998,
		UpdatedAt:         startedAt.Add(time.Minute),
		Channels: []PurgeChannelProgress{
			{ChannelID: 100, Deleted: 5, Done: true},
			{ChannelID: 200, Deleted: 2, Error: "missing access", BeforeID: 555, PendingDeleteIDs: []uint64{666, 777}},
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
		got.TotalDeleted != progress.TotalDeleted ||
		got.CommandMessageID != progress.CommandMessageID ||
		got.FallbackChannelID != progress.FallbackChannelID ||
		got.FallbackMessageID != progress.FallbackMessageID ||
		len(got.ChannelIDs) != len(progress.ChannelIDs) ||
		len(got.Channels) != len(progress.Channels) {
		t.Fatalf("progress mismatch after round trip: %#v", got)
	}
	if got.Channels[1].Error != "missing access" {
		t.Fatalf("channel error was not preserved: %#v", got.Channels[1])
	}
	if got.Channels[1].BeforeID != 555 || len(got.Channels[1].PendingDeleteIDs) != 2 {
		t.Fatalf("per-channel cursor and pending deletes were not preserved: %#v", got.Channels[1])
	}
	if got.Channels[0].BeforeID != 0 || got.Channels[0].PendingDeleteIDs != nil {
		t.Fatalf("an untouched channel should stay empty: %#v", got.Channels[0])
	}
}
