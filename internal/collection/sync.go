package collection

import (
	"context"
	cryptorand "crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"boardgames/internal/bgg"
	"boardgames/internal/config"
	"boardgames/internal/db"
	"boardgames/internal/plays"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type SyncService struct {
	DB      *sql.DB
	BGG     *bgg.Client
	Games   *Repo
	Plays   *plays.Repo
	Config  *config.Config
	startMu sync.Mutex
}

func NewSyncService(database *sql.DB, client *bgg.Client, games *Repo, playRepo *plays.Repo, cfg *config.Config) *SyncService {
	return &SyncService{DB: database, BGG: client, Games: games, Plays: playRepo, Config: cfg}
}

type SyncRun struct {
	ID              string
	UserID          string
	Status          string
	Phase           string
	CollectionCount int
	ThingsCount     int
	PlaysCount      int
	Error           string
	Trigger         string
	StartedAt       string
	UpdatedAt       string
	FinishedAt      string
}

// Start validates, records a run, and launches it asynchronously.
func (s *SyncService) Start(userID, trigger string) (*SyncRun, error) {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	if trigger == "" {
		trigger = "manual"
	}
	var active int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM sync_runs WHERE user_id=? AND status='running'`, userID).Scan(&active); err != nil {
		return nil, err
	}
	if active > 0 {
		return nil, ErrSyncRunning
	}
	var username string
	err := s.DB.QueryRow(`SELECT bgg_username FROM user_settings WHERE user_id=?`, userID).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) || strings.TrimSpace(username) == "" {
		return nil, ErrNoBGGUsername
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	run := &SyncRun{ID: newID(), UserID: userID, Status: "running", Phase: "queued", Trigger: trigger, StartedAt: db.Now()}
	run.UpdatedAt = run.StartedAt
	_, err = s.DB.Exec(`INSERT INTO sync_runs(id,user_id,status,phase,trigger,started_at,updated_at) VALUES(?,?,?,?,?,?,?)`,
		run.ID, userID, run.Status, run.Phase, run.Trigger, run.StartedAt, run.UpdatedAt)
	if err != nil {
		return nil, err
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		s.run(ctx, userID, run.ID)
	}()
	return run, nil
}

var (
	ErrSyncRunning   = errors.New("a sync is already running")
	ErrNoBGGUsername = errors.New("add your public BoardGameGeek username first")
)

func newID() string {
	b := make([]byte, 16)
	_, _ = cryptorand.Read(b)
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&15]
	}
	return string(out)
}

func (s *SyncService) run(ctx context.Context, userID, runID string) {
	ctx, span := otel.Tracer("sync").Start(ctx, "bgg.sync", trace.WithSpanKind(trace.SpanKindInternal))
	var lastErr error
	var thingsErr error
	defer func() {
		if p := recover(); p != nil {
			lastErr = fmt.Errorf("sync panic: %v", p)
		}
		if lastErr != nil {
			s.finish(runID, "failed", lastErr.Error())
			slog.Error("sync failed", "user", userID, "run", runID, "error", lastErr)
			span.RecordError(lastErr)
			span.SetStatus(codes.Error, lastErr.Error())
		} else if thingsErr != nil {
			span.SetAttributes(attribute.String("sync.status", "partial"))
			span.SetStatus(codes.Error, "metadata incomplete")
		} else {
			span.SetAttributes(attribute.String("sync.status", "success"))
		}
		span.End()
	}()

	var username string
	if err := s.DB.QueryRowContext(ctx, `SELECT bgg_username FROM user_settings WHERE user_id=?`, userID).Scan(&username); err != nil {
		lastErr = err
		return
	}
	if strings.TrimSpace(username) == "" {
		lastErr = ErrNoBGGUsername
		return
	}

	s.phase(runID, "collection", 0, 0, 0)
	collectionCount, err := s.syncCollection(ctx, userID, username)
	if err != nil {
		lastErr = fmt.Errorf("collection sync: %w", err)
		return
	}
	span.SetAttributes(attribute.Int("sync.collection_count", collectionCount))
	slog.Info("bgg collection synced", "user", userID, "games", collectionCount)

	s.phase(runID, "things", collectionCount, 0, 0)
	var thingsCount int
	thingsCount, thingsErr = s.syncThings(ctx, userID, runID)
	if thingsErr != nil {
		// Successful batches remain committed; still attempt plays and finish partial.
		slog.Warn("metadata sync incomplete", "user", userID, "error", thingsErr)
	}
	span.SetAttributes(attribute.Int("sync.things_count", thingsCount))
	slog.Info("bgg metadata synced", "user", userID, "games", thingsCount)

	s.phase(runID, "plays", collectionCount, thingsCount, 0)
	playsCount, err := s.syncPlays(ctx, userID, username, runID)
	if err != nil {
		lastErr = fmt.Errorf("plays sync: %w", err)
		return
	}
	span.SetAttributes(attribute.Int("sync.plays_count", playsCount))
	slog.Info("bgg plays synced", "user", userID, "plays", playsCount)

	s.phase(runID, "finalize", collectionCount, thingsCount, playsCount)
	if err := db.WithTx(ctx, s.DB, func(tx *sql.Tx) error { return RefreshAllFTS(ctx, tx) }); err != nil {
		lastErr = fmt.Errorf("refresh search index: %w", err)
		return
	}
	if thingsErr != nil {
		s.finish(runID, "partial", "Some game details could not be refreshed. The next sync will retry them.")
		return
	}
	s.finish(runID, "success", "")
}

func (s *SyncService) syncCollection(ctx context.Context, userID, username string) (int, error) {
	c, err := s.BGG.Collection(ctx, username)
	if err != nil {
		return 0, err
	}
	var count int
	err = db.WithTx(ctx, s.DB, func(tx *sql.Tx) error {
		keep := make([]int64, 0, len(c.Items))
		for _, item := range c.Items {
			id, err := UpsertCollectionItem(ctx, tx, userID, item)
			if err != nil {
				return err
			}
			keep = append(keep, id)
			count++
		}
		return ClearRemovedOwnership(ctx, tx, userID, keep)
	})
	return count, err
}

func (s *SyncService) syncThings(ctx context.Context, userID, runID string) (int, error) {
	cutoff := time.Now().Add(-time.Duration(s.Config.BGGRefreshDays) * 24 * time.Hour).UTC().Format(time.RFC3339)
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT g.id,g.bgg_id FROM games g JOIN user_games ug ON ug.game_id=g.id
		WHERE ug.user_id=? AND (g.fetched_at IS NULL OR g.fetched_at < ?)`, userID, cutoff)
	if err != nil {
		return 0, err
	}
	var ids []int
	for rows.Next() {
		var internalID int64
		var bggID int
		if rows.Scan(&internalID, &bggID) == nil {
			ids = append(ids, bggID)
		}
	}
	rows.Close()
	count := 0
	var lastErr error
	for start := 0; start < len(ids); start += 20 {
		end := start + 20
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		things, err := s.BGG.Things(ctx, batch)
		if err != nil {
			lastErr = err
			slog.Warn("metadata batch failed", "run", runID, "error", err)
			continue
		}
		err = db.WithTx(ctx, s.DB, func(tx *sql.Tx) error {
			for _, t := range things.Items {
				if _, err := UpsertThing(ctx, tx, t); err != nil {
					return err
				}
				count++
			}
			return nil
		})
		if err != nil {
			lastErr = err
			continue
		}
		s.phase(runID, "things", 0, count, 0)
	}
	return count, lastErr
}

func (s *SyncService) syncPlays(ctx context.Context, userID, username, runID string) (int, error) {
	count := 0
	totalPages := 1
	for page := 1; page <= totalPages; page++ {
		res, err := s.BGG.Plays(ctx, username, page)
		if err != nil {
			return count, err
		}
		if res.Total > 0 {
			totalPages = (res.Total + 99) / 100
		} else if len(res.Plays) == 0 {
			break
		}
		if totalPages > 200 {
			totalPages = 200
		}
		err = db.WithTx(ctx, s.DB, func(tx *sql.Tx) error {
			for _, bp := range res.Plays {
				gameID, err := EnsureStubGame(ctx, tx, bp.Item.ObjectID, bp.Item.Name)
				if err != nil {
					return err
				}
				id := newID()
				bggID := int64(bp.ID)
				p := plays.Play{
					ID: id, GameID: gameID, PlayedAt: normaliseDate(bp.Date), Location: bp.Location,
					Duration: bp.Length, Quantity: max(bp.Quantity, 1), Notes: bp.Comments, BGGPlayID: &bggID,
				}
				for i, pp := range bp.Players {
					name := pp.Name
					if name == "" {
						name = pp.Username
					}
					p.Players = append(p.Players, plays.Player{Name: name, Score: pp.Score, IsWinner: pp.IsWinner(), IsFirst: pp.IsNew()})
					_ = i
				}
				if _, err := s.Plays.UpsertImportedTx(ctx, tx, userID, p); err != nil {
					return err
				}
				count++
			}
			return nil
		})
		if err != nil {
			return count, err
		}
		s.phase(runID, "plays", 0, 0, count)
		if len(res.Plays) == 0 {
			break
		}
	}
	return count, nil
}

func normaliseDate(v string) string {
	if len(v) >= 10 {
		return v[:10]
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *SyncService) phase(runID, phase string, collection, things, playCount int) {
	_, err := s.DB.Exec(`UPDATE sync_runs SET phase=?, collection_count=MAX(collection_count,?), things_count=MAX(things_count,?), plays_count=MAX(plays_count,?), updated_at=? WHERE id=?`,
		phase, collection, things, playCount, db.Now(), runID)
	if err != nil {
		slog.Warn("update sync progress", "error", err)
	}
}

func (s *SyncService) finish(runID, status, message string) {
	now := db.Now()
	_, err := s.DB.Exec(`UPDATE sync_runs SET status=?, phase='finished', error=?, updated_at=?, finished_at=? WHERE id=?`, status, message, now, now, runID)
	if err != nil {
		slog.Warn("finish sync run", "error", err)
	}
}

func (s *SyncService) Latest(ctx context.Context, userID string) (*SyncRun, error) {
	return s.scanOne(ctx, `SELECT id,user_id,status,phase,collection_count,things_count,plays_count,error,trigger,started_at,updated_at,coalesce(finished_at,'')
		FROM sync_runs WHERE user_id=? ORDER BY started_at DESC LIMIT 1`, userID)
}

func (s *SyncService) History(ctx context.Context, userID string, limit int) ([]SyncRun, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id,user_id,status,phase,collection_count,things_count,plays_count,error,trigger,started_at,updated_at,coalesce(finished_at,'')
		FROM sync_runs WHERE user_id=? ORDER BY started_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SyncRun
	for rows.Next() {
		var r SyncRun
		if err := rows.Scan(&r.ID, &r.UserID, &r.Status, &r.Phase, &r.CollectionCount, &r.ThingsCount, &r.PlaysCount, &r.Error, &r.Trigger, &r.StartedAt, &r.UpdatedAt, &r.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SyncService) scanOne(ctx context.Context, query string, args ...any) (*SyncRun, error) {
	var r SyncRun
	err := s.DB.QueryRowContext(ctx, query, args...).Scan(&r.ID, &r.UserID, &r.Status, &r.Phase, &r.CollectionCount, &r.ThingsCount, &r.PlaysCount, &r.Error, &r.Trigger, &r.StartedAt, &r.UpdatedAt, &r.FinishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// Scheduler checks every minute for users whose interval has elapsed.
func (s *SyncService) Scheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDue(ctx)
		}
	}
}

func (s *SyncService) runDue(ctx context.Context) {
	rows, err := s.DB.QueryContext(ctx, `SELECT user_id, bgg_sync_interval_seconds, coalesce((SELECT MAX(started_at) FROM sync_runs r WHERE r.user_id=user_settings.user_id),'')
		FROM user_settings WHERE bgg_sync_enabled=1 AND bgg_username<>''`)
	if err != nil {
		slog.Warn("scheduler query", "error", err)
		return
	}
	type due struct {
		userID   string
		interval time.Duration
	}
	var users []due
	for rows.Next() {
		var userID, last string
		var secs int
		if rows.Scan(&userID, &secs, &last) == nil {
			elapsed := true
			if t := db.ParseTime(last); !t.IsZero() {
				elapsed = time.Since(t) >= time.Duration(secs)*time.Second
			}
			if elapsed {
				users = append(users, due{userID, time.Duration(secs) * time.Second})
			}
		}
	}
	rows.Close()
	for _, u := range users {
		if _, err := s.Start(u.userID, "scheduled"); err != nil {
			if !errors.Is(err, ErrSyncRunning) {
				slog.Warn("scheduled sync could not start", "user", u.userID, "error", err)
			}
		}
	}
}

// VerifyUsername asks BGG whether the public collection can be read.
func (s *SyncService) VerifyUsername(ctx context.Context, username string) error {
	return s.BGG.ValidateUsername(ctx, username)
}
