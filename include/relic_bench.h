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
 * @defgroup bench Automated benchmarks.
 */

/**
 * @file
 *
 * Useful routines for benchmarking.
 *
 * @version $Id: relic_bench.h 38 2009-06-03 18:15:41Z dfaranha $
 * @ingroup bench
 */

#ifndef RELIC_BENCH_H
#define RELIC_BENCH_H

#include "relic_core.h"
#include "relic_conf.h"

/**
 * Runs a new benchmark once.
 *
 * @param[in] LABEL			- the label for this benchmark.
 */
#define BENCH_ONCE(LABEL, FUNCTION)											\
	bench_timing_reset(LABEL);												\
	bench_timing_before();													\
	FUNCTION;																\
	bench_timing_after();													\
	bench_timing_compute(1);												\

/**
 * Runs a new benchmark.
 *
 * @param[in] LABEL			- the label for this benchmark.
 */
#define BENCH_BEGIN(LABEL)													\
	bench_timing_reset(LABEL);												\
	for (int i = 0; i < BENCH; i++)	{										\

/**
 * Prints the mean timing of each execution in nanoseconds.
 */
#define BENCH_END															\
	}																		\
	bench_timing_compute(BENCH * BENCH);									\

/**
 * Measures the time of one execution and adds it to the benchmark total.
 *
 * @param[in] FUNCTION		- the function executed.
 */
#define BENCH_ADD(FUNCTION)													\
	FUNCTION;																\
	bench_timing_before();													\
	for (int j = 0; j < BENCH; j++) {										\
		FUNCTION;															\
	}																		\
	bench_timing_after();													\

/**
 * Resets the benchmark data.
 *
 * @param[in] label			- the benchmark label.
 */
void bench_timing_reset(char *label);

/**
 * Measures the time before a benchmark is executed.
 */
void bench_timing_before(void);

/**
 * Measures the time after a benchmark was started and adds it to the total.
 */
void bench_timing_after(void);

/**
 * Prints the mean elapsed time between the start and the end of a benchmark.
 *
 * @param benches			- the number of executed benchmarks.
 */
void bench_timing_compute(int benches);

#endif /* !RELIC_BENCH_H */
