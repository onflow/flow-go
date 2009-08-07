/*
 * Copyright 2007 Project RELIC
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
#if TIMER == HRES
typedef struct timespec bench_t;
#elif TIMER == ANSI
typedef clock_t bench_t;
#elif TIMER == POSIX
typedef struct timeval bench_t;
#elif TIMER == CYCLE
typedef unsigned long long bench_t;

#if ARCH == X86
static inline bench_t cycles(void) {
	unsigned long long int x;
	asm volatile (""
		".byte 0x0f, 0x31\n\t"
		:"=A" (x)
	);
	return x;
}
#elif ARCH == X86_64
static inline bench_t cycles(void) {
	unsigned int hi, lo;
	asm volatile(
		"rdtsc\n\t"
		: "=a" (lo), "=d"(hi)
	);
	return ((bench_t)lo) | (((bench_t)hi) << 32);
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

#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void bench_timing_reset(char *label) {
#if TIMER != NONE
	total = 0;
	util_print("BENCH: %s %*s = ", label, (int)(25 - strlen(label)), " ");
#else
	(void)label;
#endif
}

void bench_timing_before() {
#if TIMER == HRES
	clock_gettime(CLOCK_REALTIME, &before);
#elif TIMER == ANSI
	before = clock();
#elif TIMER == POSIX
	gettimeofday(&before, NULL);
#elif TIMER == CYCLE
	before = cycles();
#endif
}

void bench_timing_after() {
	unsigned long long result;
#if TIMER == HRES
	clock_gettime(CLOCK_REALTIME, &after);
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

void bench_timing_compute(int benches) {
#if TIMER == CYCLE
	util_print("%llu cycles\n", total / (benches));
#elif TIMER != NONE
	util_print("%llu nanosec\n", total / (benches));
#else
	(void)benches;
#endif
}
