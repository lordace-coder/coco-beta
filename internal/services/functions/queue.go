package functions

// queue.go — lightweight background task queue per project.
//
// queue.add(fn, data?)    — enqueue a JS function to run after the response returns
// queue.call(file, data?) — enqueue an app.on("queue", file, handler) dispatch
//
// Tasks run in a single goroutine per project (FIFO, no concurrency within a project).
// Max 100 pending tasks per project; tasks beyond that are dropped with a log warning.
// 20-second timeout per task. Not persisted — lost on restart.

import (
	"log"
	"sync"
	"time"
)

const (
	queueMaxTasks   = 100
	queueTaskTimeout = 20 * time.Second
)

// task is a single unit of background work.
type task struct {
	projectID string
	label     string // file name or "inline" for queue.add
	run       func()
}

// projectQueue is a buffered channel + worker goroutine per project.
type projectQueue struct {
	ch   chan task
	once sync.Once
}

var (
	queueMu sync.Mutex
	queues  = map[string]*projectQueue{} // projectID → queue
)

func getProjectQueue(projectID string) *projectQueue {
	queueMu.Lock()
	defer queueMu.Unlock()
	if q, ok := queues[projectID]; ok {
		return q
	}
	q := &projectQueue{ch: make(chan task, queueMaxTasks)}
	queues[projectID] = q
	q.once.Do(func() {
		go runWorker(q)
	})
	return q
}

// runWorker drains the task channel for one project sequentially.
func runWorker(q *projectQueue) {
	for t := range q.ch {
		func() {
			done := make(chan struct{})
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[queue:%s] panic in %s: %v", t.projectID[:8], t.label, r)
					}
					close(done)
				}()
				t.run()
			}()
			select {
			case <-done:
			case <-time.After(queueTaskTimeout):
				log.Printf("[queue:%s] task %s timed out after %s", t.projectID[:8], t.label, queueTaskTimeout)
			}
		}()
	}
}

// enqueueTask adds a task to the project's queue.
// If the queue is full (100 tasks) the task is dropped with a warning.
func enqueueTask(projectID, label string, run func()) {
	q := getProjectQueue(projectID)
	t := task{projectID: projectID, label: label, run: run}
	select {
	case q.ch <- t:
	default:
		log.Printf("[queue:%s] queue full (%d tasks), dropping task %s", projectID[:8], queueMaxTasks, label)
	}
}
