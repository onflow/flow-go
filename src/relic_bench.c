/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file
 * for contact information.
 *
 * RELIC is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2.1 of the License, or (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of useful benchmark routines.
 *
 * @version $Id$
 * @ingroup relic
 */

#include <stdio.h>
#include <string.h>

#include "relic_bench.h"
#include "relic_conf.h"
#include "relic_util.h"
#include "relic_arch.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Timer type.
 */
#if TIMER == HREAL || TIMER == HPROC || TIMER == HTHRD

#include <sys/time.h>
#include <time.h>
typedef struct timespec bench_t;

#elif TIMER == ANSI

#include <time.h>
typedef clock_t bench_t;

#elif TIMER == POSIX

#include <sys/time.h>
typedef struct timeval bench_t;

#elif TIMER == CYCLE

typedef unsigned long long bench_t;

#else

typedef unsigned long long bench_t;

#endif

/**
 * Shared parameter for these timer.
 */
#if TIMER == HREAL
#define CLOCK			CLOCK_REALTIME
#elif TIMER == HPROC
#define CLOCK			CLOCK_PROCESS_CPUTIME_ID
#elif TIMER == HTHRD
#define CLOCK			CLOCK_THREAD_CPUTIME_ID
#else
#define CLOCK			NULL
#endif

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
static long long overhead = 0;

#if TIMER != NONE && BENCH > 1

/**
 * Dummy function for measuring benchmarking overhead.
 *
 * @param a				- the dummy parameter.
 */
static void empty(int *a) {
	(*a)++;
}

#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void bench_overhead(void) {
#if TIMER != NONE && BENCH > 1
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
		/* Overhead stores the cost of n*(n^2 + over) = n^3 + n*over. */
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
		/* Subtract the cost of (n^3 + over). */
		overhead -= total;
		/* Now overhead stores (n - 1)*over, so take the average to obtain the
		 * overhead to execute BENCH operations inside a benchmark. */
		overhead /= (BENCH - 1);
		/* Divide to obtain the overhead of one operation pair. */
		overhead /= BENCH;
	} while (overhead < 0);
	total = overhead;
	bench_print();
#endif
}

void bench_reset() {
#if TIMER != NONE
	total = 0;
#else
	(void)before;
	(void)after;
	(void)overhead;
	(void)empty;
#endif
}

void bench_before() {
#if TIMER == HREAL || TIMER == HPROC || TIMER == HTHRD
	clock_gettime(CLOCK, &before);
#elif TIMER == ANSI
	before = clock();
#elif TIMER == POSIX
	gettimeofday(&before, NULL);
#elif TIMER == CYCLE
	before = arch_cycles();
#endif
}

void bench_after() {
	long long result;
#if TIMER == HREAL || TIMER == HPROC || TIMER == HTHRD
	clock_gettime(CLOCK, &after);
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
	after = arch_cycles();
	result = (after - before);
#endif

#if TIMER != NONE
	total += result;
#else
	(void)result;
#endif
}

void bench_compute(int benches) {
#if TIMER != NONE
	total = total / benches - overhead;
#else
	(void)benches;
#endif
}

void bench_print() {
#if TIMER == CYCLE
	util_print("%lld cycles", total);
#else
	util_print("%lld nanosec", total);
#endif
	if (total < 0) {
		util_print(" (bad overhead estimation)\n");
	} else {
		util_print("\n");
	}
}

unsigned long long bench_total() {
	return total;
}
