package explore_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/oggyb/muzz-exercise/internal/app"
	"github.com/oggyb/muzz-exercise/internal/cache"
	"github.com/oggyb/muzz-exercise/internal/config"
	"github.com/oggyb/muzz-exercise/internal/db"
	pb "github.com/oggyb/muzz-exercise/internal/proto/explore"
	"github.com/oggyb/muzz-exercise/internal/service/explore"
)

//
// Test helpers
//

// SeedMinimalTestData wipes the DB and inserts a minimal, deterministic dataset
// for repeatable service tests.
//
// Dataset:
//   - Users: user1 (male), user2 (female), user3 (female)
//   - Decisions:
//   - user1 → user2 = like
//   - user2 → user1 = like (mutual with above)
//   - user3 → user1 = like (but excluded later because user1 → user3 = pass)
//   - user1 → user3 = pass
//
// This dataset allows us to test all cases:
//   - mutual like detection
//   - filtering out passed users
//   - cache counting correctness
func SeedMinimalTestData(t *testing.T, gdb *gorm.DB) {
	t.Helper()

	// Clean slate
	require.NoError(t, gdb.Exec("DELETE FROM decisions").Error)
	require.NoError(t, gdb.Exec("DELETE FROM users").Error)

	// Insert users
	users := []db.User{
		{ID: 1, Username: "user1", Email: "u1@test.com", PasswordHash: "x", Gender: "male"},
		{ID: 2, Username: "user2", Email: "u2@test.com", PasswordHash: "x", Gender: "female"},
		{ID: 3, Username: "user3", Email: "u3@test.com", PasswordHash: "x", Gender: "female"},
	}
	require.NoError(t, gdb.Create(&users).Error)

	// Insert decisions
	decisions := []db.Decision{
		{ActorID: 1, RecipientID: 2, Liked: true},  // user1 → user2
		{ActorID: 2, RecipientID: 1, Liked: true},  // user2 → user1 (mutual with above)
		{ActorID: 3, RecipientID: 1, Liked: true},  // user3 → user1 (excluded later)
		{ActorID: 1, RecipientID: 3, Liked: false}, // user1 → user3 (pass)
	}
	require.NoError(t, gdb.Create(&decisions).Error)

	// Debug: verify insertions
	var dbUsers []db.User
	gdb.Find(&dbUsers)
	t.Logf("Seeded users: %+v", dbUsers)

	var dbDecisions []db.Decision
	gdb.Find(&dbDecisions)
	t.Logf("Seeded decisions: %+v", dbDecisions)
}

// setupService spins up an in-memory SQLite DB, applies migrations,
// seeds test data, starts a miniredis, and wires everything into an
// ExploreService instance.
//
// Each test gets its own isolated DB + Redis.
func setupService(t *testing.T) *explore.Service {
	t.Helper()

	// In-memory SQLite
	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	dbase, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		NowFunc:                func() time.Time { return time.Now().UTC().Truncate(time.Millisecond) },
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	sqlDB, err := dbase.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	// Auto-migrate schema
	require.NoError(t, dbase.AutoMigrate(&db.User{}, &db.Decision{}))

	// Seed data
	SeedMinimalTestData(t, dbase)

	// Fake Redis
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(func() { mr.Close() })

	cfg := config.New()
	cfg.Redis.Addr = mr.Addr()

	redisCache := cache.NewRedisCache(cfg)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil)) // discard logs in tests

	appCtx := app.New(dbase, redisCache, logger)
	return explore.NewExploreService(appCtx)
}

//
// Tests
//

// TestPutDecisionAndMutualLike ensures that a mutual like is correctly detected
// when user2 likes back user1, who already liked user2 in the seed dataset.
func TestPutDecisionAndMutualLike(t *testing.T) {
	ctx := context.Background()
	svc := setupService(t)

	resp, err := svc.PutDecision(ctx, &pb.PutDecisionRequest{
		ActorUserId:     "2",
		RecipientUserId: "1",
		LikedRecipient:  true,
	})
	require.NoError(t, err)

	// mutual like confirmed (1 ↔ 2)
	assert.True(t, resp.MutualLikes)
}

// TestListLikedYou checks that only valid likers are returned.
// Expects only user2 because user3 liked user1 but was passed by user1.
func TestListLikedYou(t *testing.T) {
	ctx := context.Background()
	svc := setupService(t)

	resp, err := svc.ListLikedYou(ctx, &pb.ListLikedYouRequest{RecipientUserId: "1"})
	require.NoError(t, err)

	require.Len(t, resp.Likers, 1)
	assert.Equal(t, "2", resp.Likers[0].ActorId)
}

// TestListNewLikedYou checks that new likes are correctly filtered.
// User3 liked user1, but since user1 already passed user3, it should not appear.
func TestListNewLikedYou(t *testing.T) {
	ctx := context.Background()
	svc := setupService(t)

	resp, err := svc.ListNewLikedYou(ctx, &pb.ListLikedYouRequest{RecipientUserId: "1"})
	require.NoError(t, err)

	require.Len(t, resp.Likers, 0)
}

// TestCountLikedYouCache verifies like counts with cache.
// Only user2 counts for user1. User3 is excluded due to a pass.
func TestCountLikedYouCache(t *testing.T) {
	ctx := context.Background()
	svc := setupService(t)

	// First call → DB
	resp1, err := svc.CountLikedYou(ctx, &pb.CountLikedYouRequest{RecipientUserId: "1"})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), resp1.Count)

	// Second call → cache
	resp2, err := svc.CountLikedYou(ctx, &pb.CountLikedYouRequest{RecipientUserId: "1"})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), resp2.Count)
}
