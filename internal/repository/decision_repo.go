package repository

import (
	"context"
	"errors"
	"github.com/oggyb/muzz-exercise/internal/db"
	"github.com/oggyb/muzz-exercise/internal/utils/pagination"
	"time"

	"gorm.io/gorm"
)

// DecisionRepository provides data access methods for the Decision model.
// It encapsulates all queries related to likes/passes between users.
type DecisionRepository struct {
	db *gorm.DB
}

// NewDecisionRepository creates a new repository bound to the given DB connection.
func NewDecisionRepository(database *gorm.DB) *DecisionRepository {
	return &DecisionRepository{db: database}
}

// CreateOrUpdateDecision inserts or updates a decision made by actor -> recipient.
//
// Behavior:
//   - If (actor_id, recipient_id) pair exists → the row is updated with the new "liked" value.
//   - If it doesn’t exist → a new row is inserted.
//   - Composite PK ensures overwrite guarantee.
//
// Example:
//
//	repo.CreateOrUpdateDecision(ctx, 1, 2, true) // user 1 liked user 2
func (r *DecisionRepository) CreateOrUpdateDecision(
	ctx context.Context,
	actorID, recipientID uint64,
	liked bool,
) (prev *bool, err error) {
	var decision db.Decision
	// Try to find an existing decision between actor and recipient
	result := r.db.WithContext(ctx).
		First(&decision, "actor_id = ? AND recipient_id = ?", actorID, recipientID)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// No existing decision → insert new
		newDecision := db.Decision{
			ActorID:     actorID,
			RecipientID: recipientID,
			Liked:       liked,
		}
		if err := r.db.WithContext(ctx).Create(&newDecision).Error; err != nil {
			return nil, err
		}
		// Return nil because there was no previous value
		return nil, nil
	} else if result.Error != nil {
		// Database error while fetching decision
		return nil, result.Error
	}

	// Save the previous value before updating
	prevVal := decision.Liked

	// Update only if the value has changed
	if decision.Liked != liked {
		decision.Liked = liked
		if err := r.db.WithContext(ctx).Save(&decision).Error; err != nil {
			return &prevVal, err
		}
	}

	// Return the previous value so the service layer can decide how to update cache
	return &prevVal, nil
}

// GetLikers returns all users who liked the given recipient.
//
// Behavior:
//   - Only decisions where recipient_id = X and liked = true are returned.
//   - Excludes users that the recipient explicitly passed (liked = false).
//   - Ordered by updated_at DESC, actor_id DESC.
//   - Supports cursor-based pagination via paginationToken.
//
// Example:
//
//	repo.GetLikers(ctx, 42, nil, 20) // list first 20 people who liked user 42
func (r *DecisionRepository) GetLikers(
	ctx context.Context,
	recipientID uint64,
	paginationToken *string,
	limit int,
) ([]db.Decision, *string, error) {
	var decisions []db.Decision

	// decode cursor if provided
	cursor, err := pagination.Decode(getString(paginationToken))
	if err != nil {
		return nil, nil, err
	}

	query := r.db.WithContext(ctx).
		Table("decisions d").
		Where("d.recipient_id = ? AND d.liked = true", recipientID).
		Where(`
			NOT EXISTS (
				SELECT 1 FROM decisions d2
				WHERE d2.actor_id = ?
				  AND d2.recipient_id = d.actor_id
				  AND d2.liked = false
			)`, recipientID).
		Order("d.updated_at DESC, d.actor_id DESC").
		Limit(limit + 1)

	// apply cursor
	if cursor.ActorID > 0 && cursor.UpdatedUnix > 0 {
		ts := time.UnixMilli(cursor.UpdatedUnix)
		query = query.Where(
			"(d.updated_at < ? OR (d.updated_at = ? AND d.actor_id < ?))",
			ts, ts, cursor.ActorID,
		)
	}

	if err := query.Find(&decisions).Error; err != nil {
		return nil, nil, err
	}

	// pagination: build next cursor if needed
	var nextToken *string
	if len(decisions) > limit {
		last := decisions[limit-1]
		token, _ := pagination.Encode(pagination.Cursor{
			ActorID:     last.ActorID,
			UpdatedUnix: last.UpdatedAt.UnixMilli(),
		})
		nextToken = &token
		decisions = decisions[:limit]
	}

	return decisions, nextToken, nil
}

// GetNewLikers returns users who liked the recipient but have not been liked back.
//
// Behavior:
//   - Only decisions where recipient_id = X and liked = true are considered.
//   - Excludes mutual likes (recipient already liked them back).
//   - Excludes users the recipient explicitly passed.
//   - Ordered by updated_at DESC, actor_id DESC.
//   - Supports cursor-based pagination.
//
// Example:
//
//	repo.GetNewLikers(ctx, 42, nil, 20) // list first 20 one-way likes for user 42
func (r *DecisionRepository) GetNewLikers(
	ctx context.Context,
	recipientID uint64,
	paginationToken *string,
	limit int,
) ([]db.Decision, *string, error) {
	var decisions []db.Decision

	cursor, err := pagination.Decode(getString(paginationToken))
	if err != nil {
		return nil, nil, err
	}

	// subquery to exclude mutual likes
	subQuery := r.db.
		Table("decisions").
		Select("1").
		Where("actor_id = d.recipient_id AND recipient_id = d.actor_id AND liked = true")

	query := r.db.WithContext(ctx).
		Table("decisions d").
		Where("d.recipient_id = ? AND d.liked = true AND NOT EXISTS (?)", recipientID, subQuery).
		Where(`
			NOT EXISTS (
				SELECT 1 FROM decisions d2
				WHERE d2.actor_id = ?
				  AND d2.recipient_id = d.actor_id
				  AND d2.liked = false
			)`, recipientID).
		Order("d.updated_at DESC, d.actor_id DESC").
		Limit(limit + 1)

	// apply cursor
	if cursor.ActorID > 0 && cursor.UpdatedUnix > 0 {
		ts := time.UnixMilli(cursor.UpdatedUnix)
		query = query.Where(
			"(d.updated_at < ? OR (d.updated_at = ? AND d.actor_id < ?))",
			ts, ts, cursor.ActorID,
		)
	}

	if err := query.Find(&decisions).Error; err != nil {
		return nil, nil, err
	}

	// pagination: build next cursor if needed
	var nextToken *string
	if len(decisions) > limit {
		last := decisions[limit-1]
		token, _ := pagination.Encode(pagination.Cursor{
			ActorID:     last.ActorID,
			UpdatedUnix: last.UpdatedAt.UnixMilli(),
		})
		nextToken = &token
		decisions = decisions[:limit]
	}

	return decisions, nextToken, nil
}

// CountLikers returns how many users liked the given recipient.
//
// Behavior:
//   - Counts only decisions where recipient_id = X and liked = true.
//   - Excludes users that recipient explicitly passed.
//   - Used in conjunction with Redis cache (DB is fallback).
//
// Example:
//
//	repo.CountLikers(ctx, 42) // -> 123
func (r *DecisionRepository) CountLikers(
	ctx context.Context,
	recipientID uint64,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("decisions d").
		Where("d.recipient_id = ? AND d.liked = true", recipientID).
		Where(`
			NOT EXISTS (
				SELECT 1 FROM decisions d2
				WHERE d2.actor_id = ?
				  AND d2.recipient_id = d.actor_id
				  AND d2.liked = false
			)`, recipientID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// HasLiked checks whether an actor has liked a recipient.
//
// Behavior:
//   - Returns true if there exists a decision row where actor_id = X,
//     recipient_id = Y, and liked = true.
//   - Used for checking mutual likes in PutDecision.
//
// Example:
//
//	repo.HasLiked(ctx, 1, 2) // -> true if user 1 liked user 2
func (r *DecisionRepository) HasLiked(
	ctx context.Context,
	actorID, recipientID uint64,
) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("decisions d").
		Where("d.actor_id = ? AND d.recipient_id = ? AND d.liked = true", actorID, recipientID).
		Count(&count).Error
	return count > 0, err
}

// getString safely dereferences a string pointer for pagination tokens.
func getString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
