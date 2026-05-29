package db

import (
	"context"
	"sync"

	"github.com/lingyuins/octopus/internal/utils/log"
)

type WriteJob struct {
	Name string
	Fn   func(ctx context.Context) error
}

var (
	writeQueue chan WriteJob
	writerOnce sync.Once
)

func StartSerialWriter(ctx context.Context) {
	writerOnce.Do(func() {
		writeQueue = make(chan WriteJob, 32)
		go func() {
			for {
				select {
				case job := <-writeQueue:
					if err := job.Fn(ctx); err != nil {
						log.Warnf("serial DB write job %q failed: %v", job.Name, err)
					}
				case <-ctx.Done():
					for {
						select {
						case job := <-writeQueue:
							if err := job.Fn(context.Background()); err != nil {
								log.Warnf("shutdown: serial DB write job %q failed: %v", job.Name, err)
							}
						default:
							return
						}
					}
				}
			}
		}()
	})
}

func EnqueueWrite(job WriteJob) {
	if writeQueue == nil {
		job.Fn(context.Background())
		return
	}
	select {
	case writeQueue <- job:
	default:
		log.Warnf("serial DB write queue full, dropping job %q", job.Name)
	}
}
