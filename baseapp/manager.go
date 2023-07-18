package baseapp

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/berachain/offchain-sdk/job"
	"github.com/berachain/offchain-sdk/log"
	"github.com/berachain/offchain-sdk/worker"
)

type JobManager struct {
	// logger is the logger for the baseapp
	logger log.Logger

	// listening for conditions
	// conditionPool worker.Pool

	// list of jobs
	jobs []job.Basic

	// worker pool
	executionPool worker.Pool
}

// New creates a new baseapp.
func NewJobManager(
	name string,
	logger log.Logger,
	jobs []job.Basic,
) *JobManager {
	return &JobManager{
		logger: log.NewBlankLogger(os.Stdout),
		jobs:   jobs,
		executionPool: worker.NewPool(
			name+"-execution",
			4, //nolint:gomnd // hardcode 4 workers for now
			logger,
		),
	}
}

// Start
func (jm *JobManager) Start(ctx context.Context) {
	for _, j := range jm.jobs {
		fmt.Println("REEE")
		if basic, ok := j.(job.Conditional); ok {
			fmt.Println("REEE")
			go func() {
				for {
					time.Sleep(50 * time.Millisecond)
					if basic.Condition(ctx) {
						jm.executionPool.AddTask(job.NewExecutor(ctx, j, nil))
						return
					}
				}
			}()
		} else if basic, ok := j.(job.Subscribable); ok {
			go func() {
				for {
					time.Sleep(50 * time.Millisecond)
					ch := basic.Subscribe(ctx)
					val := <-ch
					switch val {
					case nil:
						continue
					default:
						jm.executionPool.AddTask(job.NewExecutor(ctx, j, val))
					}
				}
			}()
		} else if ethSubJob, ok := j.(job.EthSubscribable); ok {
			fmt.Println("ETH SUBSCRIBABLE")
			go func() {
				sub, ch := ethSubJob.Subscribe(ctx)
				for {
					select {
					case <-ctx.Done():
						fmt.Println("context done")
						ethSubJob.Unsubscribe(ctx)
						return
					case err := <-sub.Err():
						jm.logger.Error("error in subscription", "err", err)
						// TODO: add retry mechanism
						ethSubJob.Unsubscribe(ctx)
						return
					case val := <-ch:
						fmt.Println("adding executing job")
						jm.executionPool.AddTask(job.NewExecutor(ctx, ethSubJob, val))
						continue
					}
				}
			}()
		} else {
			panic("unknown job type")
		}
	}
}