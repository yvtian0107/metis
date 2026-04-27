package sla

import (
	"testing"

	"gorm.io/gorm"

	"metis/internal/app/itsm/testutil"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testutil.NewTestDB(t)
}
