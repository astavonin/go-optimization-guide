
package perf

import "testing"


// interface-start

type Worker interface {
    Work()
}

type LargeJob struct {
    payload [4096]byte
}

func (LargeJob) Work() {}
// interface-end

// bench-slice-start
var sink []Worker

func BenchmarkBoxedLargeSlice(b *testing.B) {
    jobs := make([]Worker, 0, 1000)
    for i := 0; i < b.N; i++ {
        jobs = jobs[:0]
        for j := 0; j < 1000; j++ {
            var job LargeJob
            jobs = append(jobs, job)
        }
        sink = jobs
    }
}

func BenchmarkPointerLargeSlice(b *testing.B) {
    jobs := make([]Worker, 0, 1000)
    for i := 0; i < b.N; i++ {
        jobs := jobs[:0]
        for j := 0; j < 1000; j++ {
            job := &LargeJob{}
            jobs = append(jobs, job)
        }
        sink = jobs
    }
}
// bench-slice-end

// bench-call-start
var sinkOne Worker

func call(w Worker) {
    sinkOne = w
}

func BenchmarkCallWithValue(b *testing.B) {
    for i := 0; i < b.N; i++ {
        var j LargeJob
        call(j)
    }
}

func BenchmarkCallWithPointer(b *testing.B) {
    for i := 0; i < b.N; i++ {
        j := &LargeJob{}
        call(j)
    }
}
// bench-call-end
