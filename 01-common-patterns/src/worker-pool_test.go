package perf

import (
	// "log"
	"fmt"
	// "os"
	"runtime"
	"sync"
	"testing"
	"crypto/sha256"
)

const (
	numJobs     = 10000
	workerCount = 10
)

func doWork(n int) [32]byte {
	data := []byte(fmt.Sprintf("payload-%d", n))
	return sha256.Sum256(data)
}

func BenchmarkUnboundedGoroutines(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(numJobs)

		for j := 0; j < numJobs; j++ {
			go func(job int) {
				_ = doWork(job)
				wg.Done()
			}(j)
		}
		wg.Wait()
	}
}

func worker(jobs <-chan int, wg *sync.WaitGroup) {
	for job := range jobs {
		_ = doWork(job)
		wg.Done()
	}
}

func BenchmarkWorkerPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(numJobs)

		jobs := make(chan int, numJobs)
		for w := 0; w < workerCount; w++ {
			go worker(jobs, &wg)
		}

		for j := 0; j < numJobs; j++ {
			jobs <- j
		}

		close(jobs)
		wg.Wait()
	}
}
