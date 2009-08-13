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
 * Implementation of useful configuration routines.
 *
 * @version $Id: relic_conf.c 13 2009-04-16 02:24:55Z dfaranha $
 * @ingroup relic
 */

#include <stdio.h>

#include "relic_conf.h"
#include "relic_bn.h"
#include "relic_fb.h"
#include "relic_core.h"
#include "relic_bench.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void conf_print(void) {
#ifndef QUIET
	util_print("-- RELIC %s configuration:\n\n", VERSION);
#if ALLOC == STATIC
	util_print("** Allocation mode: STATIC\n\n");
#elif ALLOC == DYNAMIC
	util_print("** Allocation mode: DYNAMIC\n\n");
#elif ALLOC == STACK
	util_print("** Allocation mode: STACK\n\n");
#endif

#if ARITH == EASY
	util_print("** Arithmetic backend: EASY\n\n");
#elif ARITH == X86_64
	util_print("** Arithmetic backend: X86_64\n\n");
#elif ARITH == GMP
	util_print("** Arithmetic backend: GMP\n\n");
#endif

#if BENCH > 1
	util_print("** Benchmarking options:\n");
	util_print("   Number of times: %d\n", BENCH * BENCH);
	util_print("   Estimated overhead: ");
	bench_overhead();
	util_print("\n");
#endif

#ifdef WITH_BN
	util_print("** Multiple precision module options:\n");
	util_print("   Precision: %d bits, %d words\n", BN_BITS, BN_DIGS);
	util_print("   Arithmetic method: %s\n\n", BN_METHD);
#endif

#ifdef WITH_FP
	util_print("** Prime field module options:\n");
	util_print("   Prime size: %d bits, %d words\n", FP_BITS, FP_DIGS);
	util_print("   Arithmetic method: %s\n\n", FP_METHD);
#endif

#ifdef WITH_FB
	util_print("** Binary field module options:\n");
	util_print("   Polynomial size: %d bits, %d words\n", FB_BITS, FB_DIGS);
	util_print("   Arithmetic method: %s\n", FB_METHD);
#endif

#endif
}
