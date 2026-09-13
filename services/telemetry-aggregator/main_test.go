package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestAggregate(t *testing.T) {
	events := []Event{
		{Provider:"codex", Fingerprint:"run-tests", QuestionType:"confirmation", Outcome:"accepted", LatencyMS:100},
		{Provider:"codex", Fingerprint:"run-tests", QuestionType:"confirmation", Outcome:"accepted", LatencyMS:300},
		{Provider:"claude", Fingerprint:"deploy-prod", QuestionType:"permission", Outcome:"changed", LatencyMS:900},
	}
	got := aggregate(events)
	if len(got) != 2 { t.Fatalf("expected 2 summaries, got %d", len(got)) }
	if got[0].Fingerprint != "run-tests" || got[0].Total != 2 { t.Fatalf("unexpected top summary: %#v", got[0]) }
	if got[0].Outcomes["accepted"] != 2 { t.Fatalf("expected accepted=2: %#v", got[0].Outcomes) }
	if got[0].AvgLatencyMS != 200 { t.Fatalf("expected avg latency 200, got %d", got[0].AvgLatencyMS) }
}

func TestLocalStoreRoundTrip(t *testing.T) {
	store := &LocalStore{path: filepath.Join(t.TempDir(), "events.jsonl")}
	now := time.Now().UTC()
	e := Event{SchemaVersion:1, EventID:"1", Timestamp:now, Provider:"codex", Fingerprint:"run-tests", Outcome:"accepted"}
	if err := store.Put(context.Background(), e); err != nil { t.Fatal(err) }
	got, err := store.List(context.Background(), now.Add(-time.Minute))
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0].Fingerprint != e.Fingerprint { t.Fatalf("unexpected events: %#v", got) }
	old, err := store.List(context.Background(), now.Add(time.Minute))
	if err != nil { t.Fatal(err) }
	if len(old) != 0 { t.Fatalf("expected no events after future cutoff: %#v", old) }
}
