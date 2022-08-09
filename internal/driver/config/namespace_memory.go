package config

import (
	"context"
	"reflect"
	"sync"

	"github.com/ory/herodot"
	"github.com/pkg/errors"

	"github.com/ory/keto/internal/namespace"
)

type (
	memoryNamespaceManager struct {
		byName map[string]*namespace.Namespace
		sync.RWMutex
	}
)

var _ namespace.Manager = &memoryNamespaceManager{}

func NewMemoryNamespaceManager(nn ...*namespace.Namespace) *memoryNamespaceManager {
	nm := &memoryNamespaceManager{
		byName: make(map[string]*namespace.Namespace),
	}
	for _, n := range nn {
		nm.byName[n.Name] = n
	}
	return nm
}

func (s *memoryNamespaceManager) GetNamespaceByName(_ context.Context, name string) (*namespace.Namespace, error) {
	s.RLock()
	defer s.RUnlock()

	if n, ok := s.byName[name]; ok {
		return n, nil
	}

	return nil, errors.WithStack(herodot.ErrNotFound.WithReasonf("Unknown namespace with name %q.", name))
}

func (s *memoryNamespaceManager) GetNamespaceByConfigID(_ context.Context, id int32) (*namespace.Namespace, error) {
	s.RLock()
	defer s.RUnlock()

	for _, n := range s.byName {
		if n.ID == id { // nolint ignore deprecated method
			return n, nil
		}
	}

	return nil, errors.WithStack(herodot.ErrNotFound.WithReasonf("Unknown namespace with id %d.", id))
}

func (s *memoryNamespaceManager) Namespaces(_ context.Context) ([]*namespace.Namespace, error) {
	s.RLock()
	defer s.RUnlock()

	nn := make([]*namespace.Namespace, 0, len(s.byName))
	for _, n := range s.byName {
		nn = append(nn, n)
	}

	return nn, nil
}

func (s *memoryNamespaceManager) ShouldReload(newValue interface{}) bool {
	s.RLock()
	defer s.RUnlock()

	nn, _ := s.Namespaces(context.Background())

	return !reflect.DeepEqual(newValue, nn)
}

func (s *memoryNamespaceManager) add(n *namespace.Namespace) {
	s.Lock()
	defer s.Unlock()

	s.byName[n.Name] = n
}
func (s *memoryNamespaceManager) delete(n *namespace.Namespace) {
	s.Lock()
	defer s.Unlock()

	delete(s.byName, n.Name)
}
