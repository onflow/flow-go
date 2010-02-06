/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007, 2008, 2009 RELIC Authors
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
 * Implementation of the prime elliptic curve utilities.
 *
 * @version $Id$
 * @ingroup ep
 */

#include <string.h>

#include "relic_core.h"
#include "relic_ep.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * The A coefficient of the elliptic curve.
 */
static fp_st curve_a;

/**
 * The B coefficient of the elliptic curve.
 */
static fp_st curve_b;

/**
 * The generator of the elliptic curve.
 */
static ep_st curve_g;

/**
 * The order of the group of points in the elliptic curve.
 */
static bn_st curve_r;

/**
 * Optimization identifier for the configured curve derived from the a
 * coefficient.
 */
static int curve_opt_a;

/**
 * Flag that stores if the configured prime elliptic curve is supersingular.
 */
static int curve_is_super;

#ifdef EP_PRECO

/**
 * Precomputation table for generator multiplication.
 */
static ep_st table[EP_TABLE];

/**
 * Array of pointers to the precomputation table.
 */
static ep_st *pointer[EP_TABLE];

#endif

/**
 * Detects an optimization based on the curve coefficients.
 *
 * @param opt		- the resulting optimization.
 * @param a			- the curve coefficient.
 */
static void detect_opt(int *opt, fp_t a) {
	fp_t t;

	fp_null(t);

	TRY {
		fp_new(t);
		fp_prime_conv_dig(t, 3);
		fp_neg(t, t);

		if (fp_cmp(a, t) == CMP_EQ) {
			*opt = OPT_MINUS3;
		} else {
			if (fp_is_zero(a)) {
				*opt = OPT_ZERO;
			} else {
				fp_set_dig(a, 1);
				if (fp_cmp(a, t) == CMP_EQ) {
					*opt = OPT_ONE;
				} else {
					if (fp_bits(a) <= FP_DIGIT) {
						*opt = OPT_DIGIT;
					} else {
						*opt = OPT_NONE;
					}
				}
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(t);
	}
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ep_curve_init(void) {
#ifdef EP_PRECO
	for (int i = 0; i < EP_TABLE; i++) {
		pointer[i] = &(table[i]);
	}
#endif
#if ALLOC == STATIC
	fp_new(curve_g.x);
	fp_new(curve_g.y);
	fp_new(curve_g.z);
	for (int i = 0; i < EP_TABLE; i++) {
		fp_new(table[i].x);
		fp_new(table[i].y);
		fp_new(table[i].z);
	}
#endif
	ep_set_infty(&curve_g);
	bn_init(&curve_r, FP_DIGS);
}

void ep_curve_clean(void) {
#if ALLOC == STATIC
	fp_free(curve_g.x);
	fp_free(curve_g.y);
	fp_free(curve_g.z);
	for (int i = 0; i < EP_TABLE; i++) {
		fp_free(table[i].x);
		fp_free(table[i].y);
		fp_free(table[i].z);
	}
#endif
	bn_clean(&curve_r);
}

dig_t *ep_curve_get_b() {
	return curve_b;
}

dig_t *ep_curve_get_a() {
	return curve_a;
}

int ep_curve_opt_a() {
	return curve_opt_a;
}

int ep_curve_is_super() {
	return curve_is_super;
}

void ep_curve_get_gen(ep_t g) {
	ep_copy(g, &curve_g);
}

void ep_curve_get_ord(bn_t n) {
	bn_copy(n, &curve_r);
}

#if defined(EP_PRECO)

ep_t *ep_curve_get_tab() {
#if ALLOC == AUTO
	return (ep_t*) *pointer;
#else
	return pointer;
#endif
}

#endif

#if defined(EP_ORDIN)

void ep_curve_set_ordin(fp_t a, fp_t b, ep_t g, bn_t r) {
	fp_copy(curve_a, a);
	fp_copy(curve_b, b);

	detect_opt(&curve_opt_a, curve_a);

	ep_norm(g, g);
	ep_copy(&curve_g, g);
	bn_copy(&curve_r, r);
#if defined(EP_PRECO)
	ep_mul_pre(ep_curve_get_tab(), &curve_g);
#endif
}

#endif
