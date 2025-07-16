package models

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// GetMemberByUserID retrieves a single member from the members table
func GetMemberByUserID(ctx context.Context, db *gorm.DB, channelID string, userID string) (*Member, error) {
	var member *Member

	dbCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	result := db.WithContext(dbCtx).
		Where("channel_id = ?", channelID).
		Where("user_id = ?", userID).
		First(&member)

	if result.Error != nil {
		return nil, errors.Wrap(result.Error, "failed to retrieve member by user_id from the database")
	}

	return member, nil
}
