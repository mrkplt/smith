package replica

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidJobRequest = errors.New("invalid replica job request")
	ErrSubmitFailed      = errors.New("replica job submit failed")
	ErrDeleteFailed      = errors.New("replica job delete failed")
)

type JobsAPI interface {
	CreateJob(ctx context.Context, job JobManifest) error
	DeleteJob(ctx context.Context, namespace string, name string) error
}

type JobGenerator struct {
	jobs JobsAPI
}

func NewJobGenerator(jobs JobsAPI) *JobGenerator {
	return &JobGenerator{jobs: jobs}
}

func (g *JobGenerator) Submit(ctx context.Context, req JobRequest) (JobManifest, error) {
	job, err := BuildReplicaJob(req)
	if err != nil {
		return JobManifest{}, err
	}
	if err := g.jobs.CreateJob(ctx, job); err != nil {
		return JobManifest{}, fmt.Errorf("%w: %v", ErrSubmitFailed, err)
	}
	return job, nil
}

func (g *JobGenerator) Delete(ctx context.Context, namespace, name string) error {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: namespace and name are required", ErrInvalidJobRequest)
	}
	if err := g.jobs.DeleteJob(ctx, namespace, name); err != nil {
		return fmt.Errorf("%w: %v", ErrDeleteFailed, err)
	}
	return nil
}
