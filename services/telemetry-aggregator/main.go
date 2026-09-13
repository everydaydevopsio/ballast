package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Event struct {
	SchemaVersion int       `json:"schema_version"`
	EventID       string    `json:"event_id,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	Provider      string    `json:"provider"`
	ProjectID     string    `json:"project_id,omitempty"`
	SessionID     string    `json:"session_id,omitempty"`
	Fingerprint   string    `json:"fingerprint"`
	QuestionType  string    `json:"question_type"`
	Outcome       string    `json:"outcome"`
	LatencyMS     int64     `json:"latency_ms,omitempty"`
	Question      string    `json:"question,omitempty"`
}

type Summary struct {
	Fingerprint  string         `json:"fingerprint"`
	Provider     string         `json:"provider"`
	QuestionType string         `json:"question_type"`
	Total        int            `json:"total"`
	Outcomes     map[string]int `json:"outcomes"`
	AvgLatencyMS int64          `json:"avg_latency_ms"`
	Sample       string         `json:"sample,omitempty"`
}

type Store interface {
	Put(context.Context, Event) error
	List(context.Context, time.Time) ([]Event, error)
}

type LocalStore struct {
	path string
	mu   sync.Mutex
}

func (s *LocalStore) Put(_ context.Context, e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil { return err }
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil { return err }
	defer f.Close()
	return json.NewEncoder(f).Encode(e)
}

func (s *LocalStore) List(_ context.Context, since time.Time) ([]Event, error) {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) { return nil, nil }
	if err != nil { return nil, err }
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	buf := make([]byte, 64*1024)
	sc.Buffer(buf, 2*1024*1024)
	for sc.Scan() {
		var e Event
		if json.Unmarshal(sc.Bytes(), &e) == nil && !e.Timestamp.Before(since) { out = append(out, e) }
	}
	return out, sc.Err()
}

type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

func (s *S3Store) Put(ctx context.Context, e Event) error {
	b, err := json.Marshal(e); if err != nil { return err }
	key := fmt.Sprintf("%s/%s/%s.json", strings.Trim(s.prefix, "/"), e.Timestamp.UTC().Format("2006/01/02/15"), e.EventID)
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: bytes.NewReader(b), ContentType: aws.String("application/json")})
	return err
}

func (s *S3Store) List(ctx context.Context, since time.Time) ([]Event, error) {
	prefix := strings.Trim(s.prefix, "/") + "/"
	var out []Event
	p := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{Bucket: aws.String(s.bucket), Prefix: aws.String(prefix)})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx); if err != nil { return nil, err }
		for _, obj := range page.Contents {
			if obj.LastModified != nil && obj.LastModified.Before(since) { continue }
			got, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: obj.Key}); if err != nil { return nil, err }
			var e Event
			err = json.NewDecoder(got.Body).Decode(&e); got.Body.Close()
			if err == nil && !e.Timestamp.Before(since) { out = append(out, e) }
		}
	}
	return out, nil
}

func randomID() string {
	b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b)
}

func aggregate(events []Event) []Summary {
	type acc struct { Summary; latency int64 }
	m := map[string]*acc{}
	for _, e := range events {
		k := e.Provider + "|" + e.Fingerprint
		a := m[k]
		if a == nil { a = &acc{Summary: Summary{Fingerprint:e.Fingerprint, Provider:e.Provider, QuestionType:e.QuestionType, Outcomes:map[string]int{}, Sample:e.Question}}; m[k]=a }
		a.Total++; a.Outcomes[e.Outcome]++; a.latency += e.LatencyMS
	}
	out := make([]Summary, 0, len(m))
	for _, a := range m { if a.Total > 0 { a.AvgLatencyMS = a.latency/int64(a.Total) }; out = append(out, a.Summary) }
	sort.Slice(out, func(i,j int) bool { return out[i].Total > out[j].Total })
	return out
}

func main() {
	ctx := context.Background()
	backend := getenv("BALLAST_TELEMETRY_STORAGE", "local")
	var store Store
	if backend == "s3" {
		bucket := os.Getenv("BALLAST_TELEMETRY_S3_BUCKET"); if bucket == "" { log.Fatal("BALLAST_TELEMETRY_S3_BUCKET is required for s3 storage") }
		cfg, err := config.LoadDefaultConfig(ctx); if err != nil { log.Fatal(err) }
		store = &S3Store{client:s3.NewFromConfig(cfg), bucket:bucket, prefix:getenv("BALLAST_TELEMETRY_S3_PREFIX","ballast/telemetry")}
	} else {
		store = &LocalStore{path:getenv("BALLAST_TELEMETRY_LOCAL_PATH","/data/telemetry.jsonl")}
	}
	token := os.Getenv("BALLAST_TELEMETRY_TOKEN")
	auth := func(next http.HandlerFunc) http.HandlerFunc { return func(w http.ResponseWriter, r *http.Request) { if token != "" && r.Header.Get("Authorization") != "Bearer "+token { http.Error(w,"unauthorized",http.StatusUnauthorized); return }; next(w,r) } }
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request){ w.Header().Set("Content-Type","application/json"); io.WriteString(w,`{"ok":true}`) })
	mux.HandleFunc("POST /v1/events", auth(func(w http.ResponseWriter, r *http.Request){
		defer r.Body.Close(); var e Event
		if err := json.NewDecoder(http.MaxBytesReader(w,r.Body,1<<20)).Decode(&e); err != nil { http.Error(w,"invalid event",400); return }
		if e.SchemaVersion == 0 { e.SchemaVersion=1 }; if e.EventID=="" { e.EventID=randomID() }; if e.Timestamp.IsZero() { e.Timestamp=time.Now().UTC() }
		if e.Provider=="" || e.Fingerprint=="" || e.Outcome=="" { http.Error(w,"provider, fingerprint, and outcome are required",400); return }
		if err := store.Put(r.Context(), e); err != nil { log.Printf("store event: %v",err); http.Error(w,"storage error",500); return }
		w.Header().Set("Content-Type","application/json"); w.WriteHeader(http.StatusAccepted); _=json.NewEncoder(w).Encode(map[string]string{"event_id":e.EventID})
	}))
	mux.HandleFunc("GET /v1/summary", auth(func(w http.ResponseWriter, r *http.Request){
		days := 30; if v:=r.URL.Query().Get("days"); v!="" { if n,err:=strconv.Atoi(v); err==nil && n>0 && n<=365 { days=n } }
		events, err := store.List(r.Context(), time.Now().UTC().Add(-time.Duration(days)*24*time.Hour)); if err != nil { log.Printf("list events: %v",err); http.Error(w,"storage error",500); return }
		w.Header().Set("Content-Type","application/json"); _=json.NewEncoder(w).Encode(map[string]any{"schema_version":1,"window_days":days,"events":len(events),"questions":aggregate(events)})
	}))
	addr := getenv("BALLAST_TELEMETRY_LISTEN",":8080")
	log.Printf("ballast telemetry aggregator listening on %s storage=%s",addr,backend)
	log.Fatal(http.ListenAndServe(addr,mux))
}

func getenv(k, fallback string) string { if v:=os.Getenv(k); v!="" { return v }; return fallback }
