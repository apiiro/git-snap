package parallel

import (
	"fmt"
	"sync"
)

func CreateJobQueue(queueSize int, poolSize int) *JobQueue {

	group := &JobQueue{
		jobsChannel: make(chan func(), queueSize),
		waitGroup:   &sync.WaitGroup{},
	}

	for i := 1; i <= poolSize; i++ {
		go group.worker()
	}
	return group
}

type JobQueue struct {
	jobsChannel chan func()
	waitGroup   *sync.WaitGroup
}

func (queue *JobQueue) Add(function func()) error {
	if function == nil {
		return fmt.Errorf("nil function")
	}

	queue.waitGroup.Add(1)
	queue.jobsChannel <- function
	return nil
}

func (queue *JobQueue) Wait() error {
	queue.waitGroup.Wait()
	return nil
}

func (queue *JobQueue) Close() {
	close(queue.jobsChannel)
}

func (queue *JobQueue) worker() {
	for job := range queue.jobsChannel {
		job()
		queue.waitGroup.Done()
	}
}
