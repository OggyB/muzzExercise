package db

import (
	"time"
)

// User table
type User struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	Username     string `gorm:"uniqueIndex;size:64;not null"`
	Email        string `gorm:"uniqueIndex;size:128;not null"`
	PasswordHash string `gorm:"size:255;not null"`
	Active       bool   `gorm:"default:true"`
	LastLoginAt  time.Time
	Gender       string    `gorm:"size:16;not null"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

// Decision represents an actor's like/pass decision on a recipient.
//
// Composite PK: (ActorID, RecipientID)
//   - Ensures a single row per pair (overwrite guarantee).
//
// Indexes:
//   - idx_recipient_liked_updated_actor(recipient_id, liked, updated_at DESC, actor_id)
//     Optimizes queries for "who liked me" lists with pagination.
//   - idx_actor_recipient_liked(actor_id, recipient_id, liked)
//     Optimizes O(1) lookup for mutual like checks.
//
// Fields:
//   - ActorID: The user making the decision.
//   - RecipientID: The user being liked/passed.
//   - Liked: true if liked, false if passed.
//   - CreatedAt: When the decision was first created.
//   - UpdatedAt: When the decision was last updated.
type Decision struct {
	ActorID     uint64    `gorm:"primaryKey;index:idx_actor_recipient_liked,priority:1"`
	RecipientID uint64    `gorm:"primaryKey;index:idx_recipient_liked_updated_actor,priority:1;index:idx_actor_recipient_liked,priority:2"`
	Liked       bool      `gorm:"not null;type:tinyint(1);index:idx_recipient_liked_updated_actor,priority:2;index:idx_actor_recipient_liked,priority:3"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime;index:idx_recipient_liked_updated_actor,priority:3,sort:desc"`
}
