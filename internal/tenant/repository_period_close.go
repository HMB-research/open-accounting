package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) UpdateTenantWithPeriodCloseEvent(ctx context.Context, tenantID, name string, settingsJSON []byte, updatedAt time.Time, event *PeriodCloseEvent) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		UPDATE tenants
		SET name = $1, settings = $2, updated_at = $3
		WHERE id = $4
	`, name, settingsJSON, updatedAt, tenantID)
	if err != nil {
		return fmt.Errorf("update tenant: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO tenant_period_closes (
			id,
			tenant_id,
			action,
			close_kind,
			period_end_date,
			lock_date_before,
			lock_date_after,
			note,
			performed_by,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, event.ID, event.TenantID, event.Action, event.CloseKind, event.PeriodEndDate, event.LockDateBefore, event.LockDateAfter, event.Note, event.PerformedBy, event.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert period close event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *PostgresRepository) ListPeriodCloseEvents(ctx context.Context, tenantID string, limit int) ([]PeriodCloseEvent, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, action, close_kind, period_end_date, lock_date_before, lock_date_after, note, performed_by, created_at
		FROM tenant_period_closes
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list period close events: %w", err)
	}
	defer rows.Close()

	events := make([]PeriodCloseEvent, 0, limit)
	for rows.Next() {
		var event PeriodCloseEvent
		var periodEndDate time.Time
		var lockDateBefore *time.Time
		var lockDateAfter *time.Time

		if err := rows.Scan(
			&event.ID,
			&event.TenantID,
			&event.Action,
			&event.CloseKind,
			&periodEndDate,
			&lockDateBefore,
			&lockDateAfter,
			&event.Note,
			&event.PerformedBy,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan period close event: %w", err)
		}

		event.PeriodEndDate = periodEndDate.Format(periodCloseDateLayout)
		if lockDateBefore != nil {
			value := lockDateBefore.Format(periodCloseDateLayout)
			event.LockDateBefore = &value
		}
		if lockDateAfter != nil {
			value := lockDateAfter.Format(periodCloseDateLayout)
			event.LockDateAfter = &value
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate period close events: %w", err)
	}

	return events, nil
}

func (r *PostgresRepository) GetLatestCloseEventForPeriod(ctx context.Context, tenantID, periodEndDate string) (*PeriodCloseEvent, error) {
	var event PeriodCloseEvent
	var periodEndValue time.Time
	var lockDateBefore *time.Time
	var lockDateAfter *time.Time

	err := r.db.QueryRow(ctx, `
		SELECT id, tenant_id, action, close_kind, period_end_date, lock_date_before, lock_date_after, note, performed_by, created_at
		FROM tenant_period_closes
		WHERE tenant_id = $1
			AND period_end_date = $2
			AND action = $3
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, periodEndDate, PeriodCloseActionClose).Scan(
		&event.ID,
		&event.TenantID,
		&event.Action,
		&event.CloseKind,
		&periodEndValue,
		&lockDateBefore,
		&lockDateAfter,
		&event.Note,
		&event.PerformedBy,
		&event.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest period close event: %w", err)
	}

	event.PeriodEndDate = periodEndValue.Format(periodCloseDateLayout)
	if lockDateBefore != nil {
		value := lockDateBefore.Format(periodCloseDateLayout)
		event.LockDateBefore = &value
	}
	if lockDateAfter != nil {
		value := lockDateAfter.Format(periodCloseDateLayout)
		event.LockDateAfter = &value
	}

	return &event, nil
}
