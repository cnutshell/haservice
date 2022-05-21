package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlived(t *testing.T) {
	mock := &mockService{t: t}
	alive, err := NewService(mock, nil)
	require.NoError(t, err)

	err = alive.Start()
	require.NoError(t, err)

	err = alive.Stop()
	require.NoError(t, err)
}

var _ Service = (*mockService)(nil)

type mockService struct {
	t *testing.T
}

func (s *mockService) Name() string {
	return "longlongago"
}

func (s *mockService) Start() error {
	s.t.Log("start mock service:", s.Name())
	return nil
}

func (s *mockService) Stop() error {
	s.t.Log("stop mock service:", s.Name())
	return nil
}
