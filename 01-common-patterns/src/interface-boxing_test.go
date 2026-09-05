
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
func BenchmarkBoxedLargeSlice(b *testing.B) {
    jobs := make([]Worker, 0, 1000)
    for b.Loop() {
        jobs = jobs[:0]
        for j := 0; j < 1000; j++ {
            var job LargeJob
            jobs = append(jobs, job)
        }
    }
}

func BenchmarkPointerLargeSlice(b *testing.B) {
    jobs := make([]Worker, 0, 1000)
    for b.Loop() {
        jobs := jobs[:0]
        for j := 0; j < 1000; j++ {
            job := &LargeJob{}
            jobs = append(jobs, job)
        }
    }
}
// bench-slice-end

// bench-call-start
var sink Worker

func call(w Worker) {
    sink = w
}

func BenchmarkCallWithValue(b *testing.B) {
    for b.Loop() {
        var j LargeJob
        call(j)
    }
}

func BenchmarkCallWithPointer(b *testing.B) {
    for b.Loop() {
        j := &LargeJob{}
        call(j)
    }
}
// bench-call-end
