package db

import (
	"fmt"
	"gorm.io/gorm/clause"
	"log"
	"math/rand"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedTestData resets the database and populates it with demo users and decisions.
//
// Behavior:
//  1. Clears existing data in `users` and `decisions` tables.
//  2. Creates 20 users (10 male, 10 female) with hashed passwords.
//  3. Generates ~200+ decisions with ~70% likes, and every 3rd ensures a mutual like.
//
// Compatible with both MySQL and SQLite (AUTO_INCREMENT reset skipped for SQLite).
func SeedTestData(db *gorm.DB) error {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// --- Fresh start ---
	if err := db.Exec("DELETE FROM decisions").Error; err != nil {
		return fmt.Errorf("failed to clear decisions: %w", err)
	}
	if err := db.Exec("DELETE FROM users").Error; err != nil {
		return fmt.Errorf("failed to clear users: %w", err)
	}

	// Reset auto-increment sequences (only for MySQL)
	switch db.Dialector.Name() {
	case "mysql":
		db.Exec("ALTER TABLE decisions AUTO_INCREMENT = 1")
		db.Exec("ALTER TABLE users AUTO_INCREMENT = 1")
	case "sqlite":
		// Optional: reset SQLite sequences
		db.Exec("DELETE FROM sqlite_sequence WHERE name = 'decisions'")
		db.Exec("DELETE FROM sqlite_sequence WHERE name = 'users'")
	}

	log.Println("Cleared existing data")

	// --- Seed Users (10 male, 10 female) ---
	for i := 1; i <= 20; i++ {
		username := fmt.Sprintf("user%d", i)
		email := fmt.Sprintf("user%d@example.com", i)

		hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}

		gender := "male"
		if i > 10 {
			gender = "female"
		}

		user := User{
			Username:     username,
			Email:        email,
			PasswordHash: string(hash),
			Gender:       gender,
			Active:       true,
			LastLoginAt:  time.Now().Add(-time.Duration(r.Intn(500)) * time.Hour),
		}

		if err := db.Create(&user).Error; err != nil {
			return fmt.Errorf("failed to seed user: %w", err)
		}
	}
	log.Println("Seeded 20 users.")

	// --- Seed Decisions (~200+) ---
	counter := 0
	for actorID := 1; actorID <= 20; actorID++ {
		for j := 0; j < 12; j++ { // each user decides on ~12 others
			recipientID := uint64(r.Intn(20) + 1)
			if uint64(actorID) == recipientID {
				continue
			}

			var actor, recipient User
			if err := db.First(&actor, actorID).Error; err != nil {
				continue
			}
			if err := db.First(&recipient, recipientID).Error; err != nil {
				continue
			}
			if actor.Gender == recipient.Gender {
				continue
			}

			// like probability 70%
			liked := r.Intn(100) < 70

			// guarantee mutual likes every 3rd pair
			if counter%3 == 0 {
				liked = true
				// also insert reciprocal like
				recip := Decision{
					ActorID:     recipientID,
					RecipientID: uint64(actorID),
					Liked:       true,
				}
				db.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "actor_id"}, {Name: "recipient_id"}},
					DoUpdates: clause.AssignmentColumns([]string{"liked", "updated_at"}),
				}).Create(&recip)
			}

			decision := Decision{
				ActorID:     uint64(actorID),
				RecipientID: recipientID,
				Liked:       liked,
			}
			if err := db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "actor_id"}, {Name: "recipient_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"liked", "updated_at"}),
			}).Create(&decision).Error; err != nil {
				return fmt.Errorf("failed to seed decision: %w", err)
			}

			counter++
		}
	}

	return nil
}

func SeedMinimalTestData(db *gorm.DB) error {
	// Clear
	if err := db.Exec("DELETE FROM decisions").Error; err != nil {
		return err
	}
	if err := db.Exec("DELETE FROM users").Error; err != nil {
		return err
	}

	// Users
	users := []User{
		{ID: 1, Username: "user1", Email: "u1@test.com", PasswordHash: "x", Gender: "male"},
		{ID: 2, Username: "user2", Email: "u2@test.com", PasswordHash: "x", Gender: "female"},
		{ID: 3, Username: "user3", Email: "u3@test.com", PasswordHash: "x", Gender: "female"},
	}
	if err := db.Create(&users).Error; err != nil {
		return err
	}

	// Decisions
	decisions := []Decision{
		{ActorID: 1, RecipientID: 2, Liked: true},  // user1 → user2 (like)
		{ActorID: 2, RecipientID: 1, Liked: true},  // user2 → user1 (like) → mutual
		{ActorID: 3, RecipientID: 1, Liked: true},  // user3 → user1 (like, non-mutual)
		{ActorID: 1, RecipientID: 3, Liked: false}, // user1 → user3 (pass)
	}
	if err := db.Create(&decisions).Error; err != nil {
		return err
	}

	return nil
}
