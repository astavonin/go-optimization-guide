//go:build linux && cgo

package main

/*
#include <time.h>
#include <errno.h>
#include <stdint.h>

// Sleep until absolute time (CLOCK_MONOTONIC), in nanoseconds
int sleep_until_ns(int64_t target_ns) {
	struct timespec ts;
	ts.tv_sec = target_ns / 1000000000;
	ts.tv_nsec = target_ns % 1000000000;

	// TIMER_ABSTIME = 1
	return clock_nanosleep(CLOCK_MONOTONIC, 1, &ts, NULL);
}

// Get CLOCK_MONOTONIC in nanoseconds
int64_t monotonic_ns() {
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return ((int64_t)ts.tv_sec * 1000000000LL) + ts.tv_nsec;
}
*/
import "C"

import (
	"runtime"
	"sort"
	"testing"
)

const (
	interval    = 100_000 // 100µs in nanoseconds
	sampleCount = 10000
)

func BenchmarkTimerJitter_CgoPinned(b *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	jitters := make([]int64, sampleCount)

	for n := 0; n < b.N; n++ {
		start := int64(C.monotonic_ns())

		for i := 0; i < sampleCount; i++ {
			target := start + int64((i+1)*interval)
			if rc := C.sleep_until_ns(C.longlong(target)); rc != 0 {
				b.Fatalf("clock_nanosleep failed: %d", int(rc))
			}
			now := int64(C.monotonic_ns())
			jitters[i] = now - target
		}

		reportJitterStats(b, jitters)
	}
}

func reportJitterStats(b *testing.B, samples []int64) {
	cp := append([]int64(nil), samples...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })

	p50 := cp[len(cp)/2]
	p99 := cp[len(cp)*99/100]
	max := cp[len(cp)-1]

	b.Logf("Jitter (µs): p50=%.2f, p99=%.2f, max=%.2f",
		float64(p50)/1e3, float64(p99)/1e3, float64(max)/1e3)

	b.ReportMetric(float64(p50)/1e3, "jitter_p50_us")
	b.ReportMetric(float64(p99)/1e3, "jitter_p99_us")
	b.ReportMetric(float64(max)/1e3, "jitter_max_us")
}