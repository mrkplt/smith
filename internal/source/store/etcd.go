package store

import (
	"context"
	"errors"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type Store struct {
	cli *clientv3.Client
}

func New(ctx context.Context, endpoints []string, dialTimeout time.Duration) (*Store, error) {
	clean := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		ep = strings.TrimSpace(ep)
		if ep != "" {
			clean = append(clean, ep)
		}
	}
	if len(clean) == 0 {
		return nil, errors.New("at least one etcd endpoint is required")
	}
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	cli, err := clientv3.New(clientv3.Config{
		Context:     ctx,
		Endpoints:   clean,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &Store{cli: cli}, nil
}

func (s *Store) Close() error {
	if s == nil || s.cli == nil {
		return nil
	}
	return s.cli.Close()
}

func (s *Store) Client() *clientv3.Client {
	return s.cli
}
