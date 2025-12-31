package database

import (
	"context"
	"fmt"
	"sync"

	"gorm.io/gorm"
)

type contextKey string

const schemaKey contextKey = "tenant_schema"

// WithSchema adds schema name to context
func WithSchema(ctx context.Context, schemaName string) context.Context {
	return context.WithValue(ctx, schemaKey, schemaName)
}

// GetSchema retrieves schema from context
func GetSchema(ctx context.Context) string {
	if v := ctx.Value(schemaKey); v != nil {
		return v.(string)
	}
	return "public"
}

// SchemaScope returns a GORM scope that sets search_path for a tenant schema
func SchemaScope(schemaName string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if schemaName == "" || schemaName == "public" {
			return db
		}
		return db.Exec(fmt.Sprintf("SET search_path TO %s, public", schemaName))
	}
}

// TenantDB returns a scoped DB for a tenant schema
// This sets the search_path to the tenant's schema
func TenantDB(db *gorm.DB, schemaName string) *gorm.DB {
	if schemaName == "" || schemaName == "public" {
		return db
	}
	return db.Scopes(SchemaScope(schemaName))
}

// TenantDBCache provides cached tenant-scoped DB instances
// to avoid repeated SET search_path calls for the same tenant
type TenantDBCache struct {
	mu    sync.RWMutex
	cache map[string]*gorm.DB
	base  *gorm.DB
}

// NewTenantDBCache creates a new cache for tenant-scoped DB instances
func NewTenantDBCache(baseDB *gorm.DB) *TenantDBCache {
	return &TenantDBCache{
		cache: make(map[string]*gorm.DB),
		base:  baseDB,
	}
}

// Get returns a DB instance scoped to the given schema
// Results are cached to improve performance
func (c *TenantDBCache) Get(schemaName string) *gorm.DB {
	if schemaName == "" || schemaName == "public" {
		return c.base
	}

	// Fast path: read lock
	c.mu.RLock()
	if db, ok := c.cache[schemaName]; ok {
		c.mu.RUnlock()
		return db
	}
	c.mu.RUnlock()

	// Slow path: write lock
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if db, ok := c.cache[schemaName]; ok {
		return db
	}

	db := TenantDB(c.base, schemaName)
	c.cache[schemaName] = db
	return db
}

// Clear removes all cached DB instances
func (c *TenantDBCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*gorm.DB)
}

// Remove removes a specific schema from the cache
func (c *TenantDBCache) Remove(schemaName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, schemaName)
}
