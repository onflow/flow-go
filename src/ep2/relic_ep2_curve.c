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
 * @ingroup ep2
 */

#include <string.h>

#include "relic_core.h"
#include "relic_ep.h"
#include "relic_ep2.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * The A coefficient of the elliptic curve.
 */
static fp2_t curve_a;

/**
 * Optimization identifier for the configured curve derived from the a
 * coefficient.
 */
static int curve_opt_a;

/**
 * The B coefficient of the elliptic curve.
 */
static fp2_t curve_b;

/**
 * The generator of the elliptic curve.
 */
static ep2_st curve_g;

/**
 * The order of the group of points in the elliptic curve.
 */
static bn_st curve_r;

/**
 * Flag that stores if the configured prime elliptic curve is supersingular.
 */
static int curve_is_super;

/**
 * Flag that stores if the configured prime elliptic curve is twisted.
 */
static int curve_is_twist;

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ep2_curve_init(void) {
	fp2_new(curve_a);
	fp2_new(curve_b);
	fp2_zero(curve_g.x);
	fp2_zero(curve_g.y);
	fp2_zero(curve_g.z);
	bn_init(&curve_r, FP_DIGS);
}

void ep2_curve_clean(void) {
	fp2_free(curve_a);
	fp2_free(curve_b);
	fp2_free(curve_g.x);
	fp2_free(curve_g.y);
	fp2_free(curve_g.z);
	bn_clean(&curve_r);
}

int ep2_curve_opt_a() {
	return curve_opt_a;
}

int ep2_curve_is_super() {
	return curve_is_super;
}

int ep2_curve_is_twist() {
	return curve_is_twist;
}

ep2_t ep2_curve_get_gen() {
	return &curve_g;
}

bn_t ep2_curve_get_ord() {
	return &curve_r;
}

void ep2_curve_set_gen(ep2_t g) {
	ep2_norm(g, g);
	ep2_copy(&curve_g, g);
}

void ep2_curve_set_ord(bn_t r) {
	bn_copy(&curve_r, r);
}

void ep2_curve_set_twist(int twist) {
	curve_is_twist = twist;
}
