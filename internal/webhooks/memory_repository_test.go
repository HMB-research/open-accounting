package webhooks

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type memoryRepository struct {
	endpoints  map[string]*Endpoint
	deliveries []Delivery
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		endpoints: make(map[string]*Endpoint),
	}
}

func (r *memoryRepository) ListEndpoints(_ context.Context, tenantID string, activeOnly bool) ([]Endpoint, error) {
	var endpoints []Endpoint
	for _, endpoint := range r.endpoints {
		if endpoint.TenantID != tenantID {
			continue
		}
		if activeOnly && !endpoint.IsActive {
			continue
		}
		copyEndpoint := *endpoint
		endpoints = append(endpoints, copyEndpoint)
	}
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].Name < endpoints[j].Name
	})
	return endpoints, nil
}

func (r *memoryRepository) GetEndpoint(_ context.Context, tenantID, endpointID string) (*Endpoint, error) {
	endpoint, ok := r.endpoints[endpointID]
	if !ok || endpoint.TenantID != tenantID {
		return nil, fmt.Errorf("webhook endpoint not found")
	}
	copyEndpoint := *endpoint
	return &copyEndpoint, nil
}

func (r *memoryRepository) CreateEndpoint(_ context.Context, endpoint *Endpoint) error {
	copyEndpoint := *endpoint
	r.endpoints[endpoint.ID] = &copyEndpoint
	return nil
}

func (r *memoryRepository) UpdateEndpoint(_ context.Context, endpoint *Endpoint) error {
	if _, ok := r.endpoints[endpoint.ID]; !ok {
		return fmt.Errorf("webhook endpoint not found")
	}
	copyEndpoint := *endpoint
	r.endpoints[endpoint.ID] = &copyEndpoint
	return nil
}

func (r *memoryRepository) DeleteEndpoint(_ context.Context, tenantID, endpointID string) (int64, error) {
	endpoint, ok := r.endpoints[endpointID]
	if !ok || endpoint.TenantID != tenantID {
		return 0, nil
	}
	delete(r.endpoints, endpointID)
	return 1, nil
}

func (r *memoryRepository) CreateDelivery(_ context.Context, delivery *Delivery) error {
	r.deliveries = append([]Delivery{*delivery}, r.deliveries...)
	return nil
}

func (r *memoryRepository) ListDeliveries(_ context.Context, tenantID, endpointID string, limit int) ([]Delivery, error) {
	var deliveries []Delivery
	for _, delivery := range r.deliveries {
		if delivery.TenantID == tenantID && delivery.EndpointID == endpointID {
			deliveries = append(deliveries, delivery)
		}
	}
	if limit > 0 && len(deliveries) > limit {
		deliveries = deliveries[:limit]
	}
	return deliveries, nil
}

func (r *memoryRepository) UpdateEndpointLastDelivery(_ context.Context, tenantID, endpointID string, deliveredAt time.Time) error {
	endpoint, ok := r.endpoints[endpointID]
	if !ok || endpoint.TenantID != tenantID {
		return fmt.Errorf("webhook endpoint not found")
	}
	endpoint.LastDeliveryAt = &deliveredAt
	return nil
}
