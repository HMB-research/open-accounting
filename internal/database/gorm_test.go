package database

import (
	"context"
	"testing"
)

func TestNewGormDBFromPoolRejectsNilPool(t *testing.T) {
	db, err := NewGormDBFromPool(context.Background(), nil)
	if err == nil {
		t.Fatal("expected nil pool error")
	}
	if db != nil {
		t.Fatal("expected nil gorm DB on error")
	}
}
