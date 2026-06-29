package mcp

import (
	"context"
	"fmt"
	"sync"
)

// ResourceHandler is a function that reads a resource
type ResourceHandler func(ctx context.Context, uri string) (*ReadResourceResult, error)

// ResourceRegistry manages registered resources
type ResourceRegistry struct {
	mu        sync.RWMutex
	resources map[string]Resource
	handlers  map[string]ResourceHandler
}

// NewResourceRegistry creates a new resource registry
func NewResourceRegistry() *ResourceRegistry {
	return &ResourceRegistry{
		resources: make(map[string]Resource),
		handlers:  make(map[string]ResourceHandler),
	}
}

// RegisterResource registers a new resource with its handler
func (r *ResourceRegistry) RegisterResource(resource Resource, handler ResourceHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.resources[resource.URI]; exists {
		return fmt.Errorf("resource already registered: %s", resource.URI)
	}

	r.resources[resource.URI] = resource
	r.handlers[resource.URI] = handler
	return nil
}

// ListResources returns all registered resources
func (r *ResourceRegistry) ListResources() []Resource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	resources := make([]Resource, 0, len(r.resources))
	for _, resource := range r.resources {
		resources = append(resources, resource)
	}
	return resources
}

// ReadResource reads a resource with the given URI
func (r *ResourceRegistry) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	r.mu.RLock()
	handler, exists := r.handlers[uri]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("resource not found: %s", uri)
	}

	return handler(ctx, uri)
}
