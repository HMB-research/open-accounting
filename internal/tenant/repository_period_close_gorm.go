//go:build gorm

package tenant

import (
	"context"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"gorm.io/gorm"
)

type periodCloseEventModel struct {
	ID             string     `gorm:"type:uuid;primaryKey"`
	TenantID       string     `gorm:"column:tenant_id;type:uuid;not null;index"`
	Action         string     `gorm:"size:20;not null"`
	CloseKind      string     `gorm:"column:close_kind;size:20;not null"`
	PeriodEndDate  time.Time  `gorm:"column:period_end_date;type:date;not null"`
	LockDateBefore *time.Time `gorm:"column:lock_date_before;type:date"`
	LockDateAfter  *time.Time `gorm:"column:lock_date_after;type:date"`
	Note           string     `gorm:"type:text"`
	PerformedBy    string     `gorm:"column:performed_by;type:uuid;not null"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null;default:now()"`
}

func (periodCloseEventModel) TableName() string {
	return "tenant_period_closes"
}

func (r *GORMRepository) UpdateTenantWithPeriodCloseEvent(ctx context.Context, tenantID, name string, settingsJSON []byte, updatedAt time.Time, event *PeriodCloseEvent) error {
	eventModel, err := periodCloseEventToModel(event)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Tenant{}).
			Where("id = ?", tenantID).
			Updates(map[string]interface{}{
				"name":       name,
				"settings":   settingsJSON,
				"updated_at": updatedAt,
			}).Error; err != nil {
			return fmt.Errorf("update tenant: %w", err)
		}

		if err := tx.Create(eventModel).Error; err != nil {
			return fmt.Errorf("insert period close event: %w", err)
		}

		return nil
	})
}

func (r *GORMRepository) ListPeriodCloseEvents(ctx context.Context, tenantID string, limit int) ([]PeriodCloseEvent, error) {
	if limit <= 0 {
		limit = 20
	}

	var eventModels []periodCloseEventModel
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Limit(limit).
		Find(&eventModels).Error; err != nil {
		return nil, fmt.Errorf("list period close events: %w", err)
	}

	events := make([]PeriodCloseEvent, len(eventModels))
	for i, model := range eventModels {
		events[i] = *periodCloseEventFromModel(&model)
	}

	return events, nil
}

func (r *GORMRepository) GetLatestCloseEventForPeriod(ctx context.Context, tenantID, periodEndDate string) (*PeriodCloseEvent, error) {
	periodEndValue, err := time.Parse(periodCloseDateLayout, periodEndDate)
	if err != nil {
		return nil, fmt.Errorf("parse period end date: %w", err)
	}

	var eventModel periodCloseEventModel
	err = r.db.WithContext(ctx).
		Where("tenant_id = ? AND period_end_date = ? AND action = ?", tenantID, periodEndValue, PeriodCloseActionClose).
		Order("created_at DESC").
		First(&eventModel).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest period close event: %w", err)
	}

	return periodCloseEventFromModel(&eventModel), nil
}

func periodCloseEventToModel(event *PeriodCloseEvent) (*periodCloseEventModel, error) {
	periodEndDate, err := time.Parse(periodCloseDateLayout, event.PeriodEndDate)
	if err != nil {
		return nil, fmt.Errorf("parse period end date: %w", err)
	}

	var lockDateBefore *time.Time
	if event.LockDateBefore != nil {
		value, err := time.Parse(periodCloseDateLayout, *event.LockDateBefore)
		if err != nil {
			return nil, fmt.Errorf("parse lock date before: %w", err)
		}
		lockDateBefore = &value
	}

	var lockDateAfter *time.Time
	if event.LockDateAfter != nil {
		value, err := time.Parse(periodCloseDateLayout, *event.LockDateAfter)
		if err != nil {
			return nil, fmt.Errorf("parse lock date after: %w", err)
		}
		lockDateAfter = &value
	}

	return &periodCloseEventModel{
		ID:             event.ID,
		TenantID:       event.TenantID,
		Action:         event.Action,
		CloseKind:      event.CloseKind,
		PeriodEndDate:  periodEndDate,
		LockDateBefore: lockDateBefore,
		LockDateAfter:  lockDateAfter,
		Note:           event.Note,
		PerformedBy:    event.PerformedBy,
		CreatedAt:      event.CreatedAt,
	}, nil
}

func periodCloseEventFromModel(model *periodCloseEventModel) *PeriodCloseEvent {
	event := &PeriodCloseEvent{
		ID:            model.ID,
		TenantID:      model.TenantID,
		Action:        model.Action,
		CloseKind:     model.CloseKind,
		PeriodEndDate: model.PeriodEndDate.Format(periodCloseDateLayout),
		Note:          model.Note,
		PerformedBy:   model.PerformedBy,
		CreatedAt:     model.CreatedAt,
	}

	if model.LockDateBefore != nil {
		value := model.LockDateBefore.Format(periodCloseDateLayout)
		event.LockDateBefore = &value
	}
	if model.LockDateAfter != nil {
		value := model.LockDateAfter.Format(periodCloseDateLayout)
		event.LockDateAfter = &value
	}

	return event
}
