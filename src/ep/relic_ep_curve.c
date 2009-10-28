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

void ep_curve_set_ordin(fp_t a, fp_t b) {
	fp_copy(curve_a, a);

	if (fp_is_zero(a)) {
		curve_opt_a = OPT_ZERO;
	} else {
		if (fp_cmp_dig(a, 1) == CMP_EQ) {
			curve_opt_a = OPT_ONE;
		} else {
			if (fp_bits(a) < FP_DIGIT) {
				curve_opt_a = OPT_DIGIT;
			} else {
				curve_opt_a = OPT_NONE;
			}
		}
	}

	fp_copy(curve_b, b);
}

void ep_curve_set_gen(ep_t g) {
	ep_norm(g, g);
	ep_copy(&curve_g, g);
}

void ep_curve_set_ord(bn_t r) {
	bn_copy(&curve_r, r);
}
