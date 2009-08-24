/*
 * Copyright 2007-2009 RELIC Project
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of useful benchmark routines.
 *
 * @version $Id: relic_bench.c 49 2009-07-04 23:48:19Z dfaranha $
 * @ingroup relic
 */

#include <stdio.h>
#include <string.h>
#include <time.h>

#include "relic_bench.h"
#include "relic_conf.h"
#include "relic_util.h"

#if OPSYS == LINUX || OPSYS == FREEBSD
#include <sys/time.h>
#endif

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Timer type.
 */
#if TIMER == HPROC || TIMER == HREAL || TIMER == HTHRD
typedef struct timespec bench_t;
#elif TIMER == ANSI
typedef clock_t bench_t;
#elif TIMER == POSIX
typedef struct timeval bench_t;
#elif TIMER == CYCLE
typedef unsigned long long bench_t;

#if ARCH == X86
#include <intrin.h>
static inline bench_t cycles(void) {
	unsigned long long int x;
	asm volatile (".byte 0x0f, 0x31\n\t":"=A" (x));
	return x;
}
#elif ARCH == X86_64
static inline bench_t cycles(void) {
	unsigned int hi, lo;
	asm volatile ("rdtsc\n\t":"=a" (lo), "=d"(hi));
	return ((bench_t) lo) | (((bench_t) hi) << 32);
}
#endif

#endif

#if TIMER != NONE

/**
 * Stores the time measured before the execution of the benchmark.
 */
static bench_t before;

/**
 * Stores the time measured after the execution of the benchmark.
 */
static bench_t after;

/**
 * Stores the sum of timings for the current benchmark.
 */
static long long total;

/**
 * Benchmarking overhead to be measured and subtracted from benchmarks.
 */
static long long overhead;

#endif

static void empty(int *a) {
	(*a)++;
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void bench_overhead(void) {
	int a[BENCH + 1];
	int *tmpa;

	do {
		overhead = 0;
		for (int l = 0; l < BENCH; l++) {
			total = 0;
			/* Measure the cost of (n^2 + over). */
			bench_before();
			for (int i = 0; i < BENCH; i++) {
				tmpa = a;
				for (int j = 0; j < BENCH; j++) {
					empty(tmpa++);
				}
			}
			bench_after();
			/* Add the cost of (n^2 + over). */
			overhead += total;
		}
		/* Overhead stores the cost of n*(n^2 + over). */
		for (int l = 0; l < BENCH; l++) {
			total = 0;
			/* Measure the cost of (n^3 + over). */
			bench_before();
			for (int i = 0; i < BENCH; i++) {
				for (int k = 0; k < BENCH; k++) {
					tmpa = a;
					for (int j = 0; j < BENCH; j++) {
						empty(tmpa++);
					}
				}
			}
			bench_after();
			/* Subtract the cost of n^2. */
			overhead -= total / BENCH;
		}
		/* Now overhead stores n*over, so take the average to obtain the overhead
		 * to execute BENCH operations inside a benchmark. */
		overhead /= BENCH;
		/* Divide to obtain the overhead of one operation pair. */
		overhead /= BENCH;
		/* We assume that our overhead estimate is too high (due to cache
		 * effects, per example). The ratio 2/3 was found experimentally. */
		overhead = overhead / 2;
	} while (overhead < 0);

#if TIMER == CYCLE
	util_print("%lld cycles\n", overhead);
#elif TIMER != NONE
	util_print("%lld nanosec\n", overhead);
#endif
}

void bench_reset(char *label) {
#if TIMER != NONE
	total = 0;
	util_print("BENCH: %s %*s = ", label, (int)(25 - strlen(label)), " ");
#else
	(void)label;
#endif
}

void bench_before() {
#if TIMER == HREAL
	clock_gettime(CLOCK_REALTIME, &before);
#elif TIMER == HPROC
	clock_gettime(CLOCK_PROCESS_CPUTIME_ID, &before);
#elif TIMER == HTHRD
	clock_gettime(CLOCK_THREAD_CPUTIME_ID, &before);
#elif TIMER == ANSI
	before = clock();
#elif TIMER == POSIX
	gettimeofday(&before, NULL);
#elif TIMER == CYCLE
	before = cycles();
#endif
}

void bench_after() {
	long long result;
#if TIMER == HREAL
	clock_gettime(CLOCK_REALTIME, &after);
	result = ((long)after.tv_sec - (long)before.tv_sec) * 1000000000;
	result += (after.tv_nsec - before.tv_nsec);
#elif TIMER == HPROC
	clock_gettime(CLOCK_PROCESS_CPUTIME_ID, &after);
	result = ((long)after.tv_sec - (long)before.tv_sec) * 1000000000;
	result += (after.tv_nsec - before.tv_nsec);
#elif TIMER == HTHRD
	clock_gettime(CLOCK_PROCESS_CPUTIME_ID, &after);
	result = ((long)after.tv_sec - (long)before.tv_sec) * 1000000000;
	result += (after.tv_nsec - before.tv_nsec);
#elif TIMER == ANSI
	after = clock();
	result = (after - before) * 1000000000 / CLOCKS_PER_SEC;
#elif TIMER == POSIX
	gettimeofday(&after, NULL);
	result = ((long)after.tv_sec - (long)before.tv_sec) * 1000000000;
	result += (after.tv_usec - before.tv_usec) * 1000;
#elif TIMER == CYCLE
	after = cycles();
	result = (after - before);
#endif

#if TIMER != NONE
	total += result;
#else
	(void)result;
#endif
}

void bench_compute(int benches) {
	total = total / benches - overhead;
#if TIMER == CYCLE
	util_print("%lld cycles", total);
#elif TIMER != NONE
	util_print("%lld nanosec", total);
#else
	(void)benches;
#endif
	if (total < 0) {
		util_print(" (bad overhead estimation)\n");
	} else {
		util_print("\n");
	}
}
