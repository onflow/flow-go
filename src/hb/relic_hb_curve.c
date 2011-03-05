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
 * Implementation of the binary elliptic curve utilities.
 *
 * @version $Id: relic_eb_curve.c 390 2010-06-05 22:15:02Z dfaranha $
 * @ingroup eb
 */

#include <string.h>

#include "relic_core.h"
#include "relic_hb.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * The f3 coefficient of the elliptic curve.
 */
static fb_st curve_f3;

/**
 * Optimization identifier for the configured curve derived from the f3
 * coefficient.
 */
static int curve_opt_f3;

/**
 * The f1 coefficient of the elliptic curve.
 */
static fb_st curve_f1;

/**
 * Optimization identifier for the configured curve derived from the f1
 * coefficient.
 */
static int curve_opt_f1;

/**
 * The f0 coefficient of the elliptic curve.
 */
static fb_st curve_f0;

/**
 * Optimization identifier for the configured curve derived from the f0
 * coefficient.
 */
static int curve_opt_f0;

/**
 * The generator of the elliptic curve.
 */
static hb_st curve_g;

#ifdef HB_PRECO

/**
 * Precomputation table for generator multiplication.
 */
static hb_st table[HB_TABLE];

/**
 * Array of pointers to the precomputation table.
 */
static hb_st *pointer[HB_TABLE];

#endif

/**
 * The order of the group of points in the elliptic curve.
 */
static bn_st curve_r;

/**
 * The cofactor of the group order in the elliptic curve.
 */
static bn_st curve_h;

/**
 * Flag that stores if the configured binary elliptic curve is supersingular.
 */
static int curve_is_super;

/**
 * Detects an optimization based on the curve coefficients.
 *
 * @param opt		- the resulting optimization.
 * @param a			- the curve coefficient.
 */
static void detect_opt(int *opt, fb_t a) {
	if (fb_is_zero(a)) {
		*opt = OPT_ZERO;
	} else {
		if (fb_cmp_dig(a, 1) == CMP_EQ) {
			*opt = OPT_ONE;
		} else {
			if (fb_bits(a) <= FB_DIGIT) {
				*opt = OPT_DIGIT;
			} else {
				*opt = OPT_NONE;
			}
		}
	}
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void hb_curve_init(void) {
#ifdef HB_PRECO
	for (int i = 0; i < HB_TABLE; i++) {
		pointer[i] = &(table[i]);
	}
#endif
#if ALLOC == STATIC
	fb_new(curve_g.u1);
	fb_new(curve_g.u0);
	fb_new(curve_g.v1);
	fb_new(curve_g.v0);
	for (int i = 0; i < HB_TABLE; i++) {
		fb_new(table[i].u1);
		fb_new(table[i].u0);
		fb_new(table[i].v1);
		fb_new(table[i].v0);
	}
#endif
	fb_zero(curve_g.u1);
	fb_zero(curve_g.u0);
	fb_zero(curve_g.v1);
	fb_zero(curve_g.v0);
	bn_init(&curve_r, FB_DIGS);
}

void hb_curve_clean(void) {
#if ALLOC == STATIC
	fb_free(curve_g.u1);
	fb_free(curve_g.u0);
	fb_free(curve_g.v1);
	fb_free(curve_g.v0);
	for (int i = 0; i < HB_TABLE; i++) {
		fb_free(table[i].u1);
		fb_free(table[i].u0);
		fb_free(table[i].v1);
		fb_free(table[i].v0);
	}
#endif
	bn_clean(&curve_r);
}

dig_t *hb_curve_get_f3() {
	return curve_f3;
}

int hb_curve_opt_f3() {
	return curve_opt_f3;
}

dig_t *hb_curve_get_f1() {
	return curve_f1;
}

int hb_curve_opt_f1() {
	return curve_opt_f1;
}

dig_t *hb_curve_get_f0() {
	if (curve_is_super) {
		return curve_f0;
	}
	return NULL;
}

int hb_curve_opt_f0() {
	return curve_opt_f0;
}

int hb_curve_is_super() {
	return curve_is_super;
}

void hb_curve_get_gen(hb_t g) {
	hb_copy(g, &curve_g);
}

void hb_curve_get_ord(bn_t n) {
	bn_copy(n, &curve_r);
}

#if defined(HB_PRECO)

hb_t *hb_curve_get_tab() {
#if ALLOC == AUTO
	return (hb_t *) *pointer;
#else
	return pointer;
#endif
}

#endif

void hb_curve_get_cof(bn_t h) {
	bn_copy(h, &curve_h);
}

#if defined(HB_SUPER)

void hb_curve_set_super(fb_t f3, fb_t f1, fb_t f0, hb_t g, bn_t r, bn_t h) {
	curve_is_super = 1;

	fb_copy(curve_f3, f3);
	fb_copy(curve_f1, f1);
	fb_copy(curve_f0, f0);

	detect_opt(&curve_opt_f3, curve_f3);
	detect_opt(&curve_opt_f1, curve_f1);
	detect_opt(&curve_opt_f0, curve_f0);

	hb_copy(&curve_g, g);
	bn_copy(&curve_r, r);
	bn_copy(&curve_h, h);
#if defined(HB_PRECO)
	hb_mul_pre(hb_curve_get_tab(), &curve_g);
#endif
}

#endif
