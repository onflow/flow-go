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
 * Optimization identifier for the configured curve derived from the a
 * coefficient.
 */
static int curve_opt_a;

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
 * Flag that stores if the configured prime elliptic curve is supersingular.
 */
static int curve_is_super;

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
#if ALLOC == STATIC
	fp_new(curve_g.x);
	fp_new(curve_g.y);
	fp_new(curve_g.z);
#endif
	fp_zero(curve_g.x);
	fp_zero(curve_g.y);
	fp_zero(curve_g.z);
	bn_init(&curve_r, FP_DIGS);
}

void ep_curve_clean(void) {
#if ALLOC == STATIC
	fp_free(curve_g.x);
	fp_free(curve_g.y);
	fp_free(curve_g.z);
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

ep_t ep_curve_get_gen() {
	return &curve_g;
}

bn_t ep_curve_get_ord() {
	return &curve_r;
}

#if defined(EP_ORDIN)

void ep_curve_set_ordin(fp_t a, fp_t b, ep_t g, bn_t r) {
	fp_copy(curve_a, a);
	fp_copy(curve_b, b);

	detect_opt(&curve_opt_a, curve_a);
	//detect_opt(&curve_opt_b, curve_b);

	ep_norm(g, g);
	ep_copy(&curve_g, g);
	bn_copy(&curve_r, r);
}

#endif

void ep_curve_set_gen(ep_t g) {
	ep_norm(g, g);
	ep_copy(&curve_g, g);
}

void ep_curve_set_ord(bn_t r) {
	bn_copy(&curve_r, r);
}
