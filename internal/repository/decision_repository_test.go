package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/oggyb/muzz-exercise/internal/db"
	"github.com/oggyb/muzz-exercise/internal/repository"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setup in-memory DB
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC().Truncate(time.Millisecond) },
	})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := database.AutoMigrate(&db.Decision{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return database
}

func TestCreateOrUpdateDecision(t *testing.T) {
	ctx := context.Background()
	dbase := setupTestDB(t)
	repo := repository.NewDecisionRepository(dbase)

	// insert like
	err := repo.CreateOrUpdateDecision(ctx, 1, 2, true)
	assert.NoError(t, err)

	// overwrite with pass
	err = repo.CreateOrUpdateDecision(ctx, 1, 2, false)
	assert.NoError(t, err)

	var d db.Decision
	_ = dbase.First(&d).Error
	assert.Equal(t, false, d.Liked)
}

func TestGetLikersAndPagination(t *testing.T) {
	ctx := context.Background()
	dbase := setupTestDB(t)
	repo := repository.NewDecisionRepository(dbase)

	// actors 1,2 liked recipient 99
	_ = repo.CreateOrUpdateDecision(ctx, 1, 99, true)
	_ = repo.CreateOrUpdateDecision(ctx, 2, 99, true)
	// recipient passed actor 2 → exclude
	_ = repo.CreateOrUpdateDecision(ctx, 99, 2, false)

	decisions, _, err := repo.GetLikers(ctx, 99, nil, 10)
	assert.NoError(t, err)
	assert.Len(t, decisions, 1)
	assert.Equal(t, uint64(1), decisions[0].ActorID)
}

func TestGetNewLikers(t *testing.T) {
	ctx := context.Background()
	dbase := setupTestDB(t)
	repo := repository.NewDecisionRepository(dbase)

	// actor 1 liked 99, and 99 liked back → mutual
	_ = repo.CreateOrUpdateDecision(ctx, 1, 99, true)
	_ = repo.CreateOrUpdateDecision(ctx, 99, 1, true)

	// actor 2 liked 99, but not mutual
	_ = repo.CreateOrUpdateDecision(ctx, 2, 99, true)

	decisions, _, err := repo.GetNewLikers(ctx, 99, nil, 10)
	assert.NoError(t, err)
	assert.Len(t, decisions, 1)
	assert.Equal(t, uint64(2), decisions[0].ActorID)
}
