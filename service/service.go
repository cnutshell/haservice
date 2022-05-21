package service

import (
	"sync"

	"go.uber.org/zap"
)

type Service interface {
	Name() string
	Start() error
	Stop() error
}

type launcher struct {
	logger *zap.Logger

	mu struct {
		sync.Mutex
		running bool
		service Service
	}
}

func NewService(service Service, logger *zap.Logger) (Service, error) {
	if logger == nil {
		logger = zap.L().Named("membership")
	}

	alived := &launcher{
		logger: logger,
	}
	alived.mu.service = service
	return alived, nil
}

func (l *launcher) Name() string {
	return l.mu.service.Name()
}

func (a *launcher) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.mu.running {
		return nil
	}

	if err := a.mu.service.Start(); err != nil {
		a.logger.Error("fail to start service")
		return err
	}
	a.mu.running = true

	return nil
}

func (a *launcher) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.mu.running {
		return nil
	}

	if err := a.mu.service.Stop(); err != nil {
		a.logger.Error("fail to stop service")
		return err
	}
	a.mu.running = false

	return nil
}
