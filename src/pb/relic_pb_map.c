/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
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
 * Implementation of the core functions for computing pairings over binary
 * fields.
 *
 * @version $Id: relic_pb_etat1.c 692 2011-03-18 00:43:02Z dfaranha $
 * @ingroup pb
 */

#include <math.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_bench.h"
#include "relic_pb.h"
#include "relic_util.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#ifdef PB_PARAL

/**
 * Precomputed table for computing fixed 2^i powers in the pairing.
 */
fb_st pb_tab_sqr[CORES][FB_TABLE_MAX];

/**
 * Precomputed table for computing fixed 1/(2^i) powers in the pairing.
 */
fb_st pb_tab_srt[CORES][FB_TABLE_MAX];

/**
 * Partition for the parallel execution.
 */
double pb_par[CORES];

#endif

/**
 * Precomputed table for the final exponentiation.
 */
fb_st pb_tab_exp[FB_TABLE_MAX];

/**
 * Computes a partition of the main loop in the pairing algorithm.
 */
static void pb_compute_par() {
	fb_t a, b;
	long long r;

	fb_null(a);
	fb_null(b);

	TRY {
		fb_new(a);
		fb_new(b);

		fb_rand(a);
		fb_rand(b);

		bench_reset();
		bench_before();
		for (int i = 0; i < BENCH; i++) {
			fb_mul(a, a, b);
		}
		bench_after();
		bench_compute(BENCH);
		r = bench_total();

		bench_reset();
		bench_before();
		for (int i = 0; i < BENCH; i++) {
			fb_sqr(a, b);
		}
		bench_after();
		bench_compute(BENCH);
		r /= bench_total();
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(a);
		fb_free(b);
	}
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void pb_map_init() {

#if PB_MAP == ETATS || PB_MAP == ETATN
	printf("%d\n", 4 * (((FB_BITS + 1) / 2) / 4));
	fb_itr_pre(pb_tab_exp, 4 * (((FB_BITS + 1) / 2) / 4));
#endif

#if PB_MAP == ETAT2 || PB_MAP == ETAT2
	fb_itr_pre(pb_tab_exp, 6 * (((FB_BITS + 1) / 2) / 6));
#endif

#if defined(PB_PARAL) && (PB_MAP == ETATS || PB_MAP == ETATN)

	pb_compute_par();

	int chunk = (int)ceilf((FB_BITS - 1) / (2.0 * CORES));

	for (int i = 0; i < CORES; i++) {
		fb_itr_pre(pb_tab_sqr[i], i * chunk);
		fb_itr_pre(pb_tab_srt[i], -i * chunk);
	}

#endif
}

void pb_map_clean() {

}

fb_t *pb_map_get_tab() {
	return pb_tab_exp;
}

fb_t *pb_map_get_sqr(int core) {
#ifdef PB_PARAL
	return pb_tab_sqr[core];
#else
	return NULL;
#endif
}

fb_t *pb_map_get_srt(int core) {
#ifdef PB_PARAL
	return pb_tab_srt[core];
#else
	return NULL;
#endif
}

int pb_map_get_par(int core) {
	int chunk = (int)ceilf((FB_BITS - 1) / (2.0 * CORES));

	if (core == CORES) {
		return MIN((FB_BITS - 1) / 2, core * chunk);
	} else {
		return core * chunk;
	}
}
