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
 * Implementation of the binary elliptic curve utilities.
 *
 * @version $Id$
 * @ingroup eb
 */

#include <string.h>

#include "relic_core.h"
#include "relic_eb.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * The A coefficient of the elliptic curve.
 */
static fb_st curve_a;

/**
 * Optimization identifier for the configured curve derived from the a
 * coefficient.
 */
static int curve_opt_a;

/**
 * The B coefficient of the elliptic curve.
 */
static fb_st curve_b;

/**
 * Optimization identifier for the configured curve derived from the b
 * coefficient.
 */
static int curve_opt_b;

#if defined(EB_SUPER)

/**
 * The C coefficient of the supersingular elliptic curve.
 */
static fb_st curve_c;

/**
 * Optimization identifier for the configured curve derived from the c
 * coefficient.
 */
static int curve_opt_c;

#endif

/**
 * The generator of the elliptic curve.
 */
static eb_st curve_g;

#ifdef EB_PRECO

/**
 * Precomputation table for generator multiplication.
 */
static eb_st table[EB_TABLE];

/**
 * Array of pointers to the precomputation table.
 */
static eb_st *pointer[EB_TABLE];

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
 * Flag that stores if the configured binary elliptic curve is a Koblitz
 * curve.
 */
static int curve_is_koblitz;

/**
 * Flag that stores if the configured binary elliptic curve is supersingular.
 */
static int curve_is_super;

#if defined(EB_KBLTZ) && (EB_MUL == WTNAF || EB_FIX == WTNAF || EB_SIM == INTER || !defined(STRIP))
/**
 * V_m auxiliary parameter for Koblitz curves.
 */
static bn_st curve_vm;

/**
 * S_0 auxiliary parameter for Koblitz curves.
 */
static bn_st curve_s0;

/**
 * S_1 auxiliary parameter for Koblitz curves.
 */
static bn_st curve_s1;

/**
 * Precomputes additional parameters for Koblitz curves used by the w-TNAF
 * multiplication algorithm.
 */
static void compute_koblitz(void) {
	int u, i;
	bn_t a, b, c;

	bn_null(a);
	bn_null(b);
	bn_null(c);

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);

		if (curve_opt_a == OPT_ZERO) {
			u = -1;
		} else {
			u = 1;
		}

		bn_set_dig(a, 2);
		bn_set_dig(b, 1);
		if (u == -1) {
			bn_neg(b, b);
		}
		for (i = 2; i <= FB_BITS; i++) {
			bn_copy(c, b);
			if (u == -1) {
				bn_neg(b, b);
			}
			bn_dbl(a, a);
			bn_sub(b, b, a);
			bn_copy(a, c);
		}
		bn_copy(&curve_vm, b);

		bn_zero(a);
		bn_set_dig(b, 1);
		for (i = 2; i <= FB_BITS; i++) {
			bn_copy(c, b);
			if (u == -1) {
				bn_neg(b, b);
			}
			bn_dbl(a, a);
			bn_sub(b, b, a);
			bn_add_dig(b, b, 1);
			bn_copy(a, c);
		}
		bn_copy(&curve_s0, b);

		bn_zero(a);
		bn_zero(b);
		for (i = 2; i <= FB_BITS; i++) {
			bn_copy(c, b);
			if (u == -1) {
				bn_neg(b, b);
			}
			bn_dbl(a, a);
			bn_sub(b, b, a);
			bn_sub_dig(b, b, 1);
			bn_copy(a, c);
		}
		bn_copy(&curve_s1, b);

	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(a);
		bn_free(b);
		bn_free(c);
	}
}

#endif

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

void eb_curve_init(void) {
#ifdef EB_PRECO
	for (int i = 0; i < EB_TABLE; i++) {
		pointer[i] = &(table[i]);
	}
#endif
#if ALLOC == STATIC
	fb_new(curve_g.x);
	fb_new(curve_g.y);
	fb_new(curve_g.z);
	for (int i = 0; i < EB_TABLE; i++) {
		fb_new(table[i].x);
		fb_new(table[i].y);
		fb_new(table[i].z);
	}
#endif
	fb_zero(curve_g.x);
	fb_zero(curve_g.y);
	fb_zero(curve_g.z);
	bn_init(&curve_r, FB_DIGS);
#if defined(EB_KBLTZ) && (EB_MUL == WTNAF || !defined(STRIP))
	bn_init(&curve_vm, FB_DIGS);
	bn_init(&curve_s0, FB_DIGS);
	bn_init(&curve_s1, FB_DIGS);
#endif
}

void eb_curve_clean(void) {
#if ALLOC == STATIC
	fb_free(curve_g.x);
	fb_free(curve_g.y);
	fb_free(curve_g.z);
	for (int i = 0; i < EB_TABLE; i++) {
		fb_free(table[i].x);
		fb_free(table[i].y);
		fb_free(table[i].z);
	}
#endif
	bn_clean(&curve_r);
#if defined(EB_KBLTZ) && (EB_MUL == WTNAF || !defined(STRIP))
	bn_clean(&curve_vm);
	bn_clean(&curve_s0);
	bn_clean(&curve_s1);
#endif
}

dig_t *eb_curve_get_a() {
	return curve_a;
}

int eb_curve_opt_a() {
	return curve_opt_a;
}

dig_t *eb_curve_get_b() {
	return curve_b;
}

int eb_curve_opt_b() {
	return curve_opt_b;
}

dig_t *eb_curve_get_c() {
#if defined(EB_SUPER)
	if (curve_is_super) {
		return curve_c;
	}
#endif
	return NULL;
}

int eb_curve_opt_c() {
#if defined(EB_SUPER)
	return curve_opt_c;
#else
	return OPT_NONE;
#endif
}

int eb_curve_is_kbltz() {
	return curve_is_koblitz;
}

int eb_curve_is_super() {
	return curve_is_super;
}

void eb_curve_get_gen(eb_t g) {
	eb_copy(g, &curve_g);
}

void eb_curve_get_ord(bn_t n) {
	bn_copy(n, &curve_r);
}

#if defined(EB_PRECO)

eb_t *eb_curve_get_tab() {
#if ALLOC == AUTO
	return (eb_t *) *pointer;
#else
	return pointer;
#endif
}

#endif

void eb_curve_get_cof(bn_t h) {
	bn_copy(h, &curve_h);
}

#if defined(EB_KBLTZ) && (EB_MUL == WTNAF || EB_FIX == WTNAF || EB_SIM == INTER || !defined(STRIP))
void eb_curve_get_vm(bn_t vm) {
	if (curve_is_koblitz) {
		bn_copy(vm, &curve_vm);
	} else {
		bn_zero(vm);
	}
}

void eb_curve_get_s0(bn_t s0) {
	if (curve_is_koblitz) {
		bn_copy(s0, &curve_s0);
	} else {
		bn_zero(s0);
	}
}

void eb_curve_get_s1(bn_t s1) {
	if (curve_is_koblitz) {
		bn_copy(s1, &curve_s1);
	} else {
		bn_zero(s1);
	}
}
#endif

#if defined(EB_ORDIN)

void eb_curve_set_ordin(fb_t a, fb_t b, eb_t g, bn_t r, bn_t h) {
	fb_copy(curve_a, a);
	fb_copy(curve_b, b);

	detect_opt(&curve_opt_a, curve_a);
	detect_opt(&curve_opt_b, curve_b);

	if (fb_cmp_dig(curve_b, 1) == CMP_EQ) {
		curve_is_koblitz = 1;
	} else {
		curve_is_koblitz = 0;
	}
#if defined(EB_KBLTZ) && (EB_MUL == WTNAF || EB_FIX == WTNAF || EB_SIM == INTER || !defined(STRIP))
	if (curve_is_koblitz) {
		compute_koblitz();
	}
#endif

	eb_norm(g, g);
	eb_copy(&curve_g, g);
	bn_copy(&curve_r, r);
	bn_copy(&curve_h, h);
#if defined(EB_PRECO)
	eb_mul_pre(eb_curve_get_tab(), &curve_g);
#endif
}

#endif

#if defined(EB_KBLTZ)

void eb_curve_set_kbltz(fb_t a, eb_t g, bn_t r, bn_t h) {
	curve_is_koblitz = 1;

	fb_copy(curve_a, a);

	fb_zero(curve_b);
	fb_set_bit(curve_b, 0, 1);

	detect_opt(&curve_opt_a, curve_a);
	detect_opt(&curve_opt_b, curve_b);

#if defined(EB_KBLTZ) && (EB_MUL == WTNAF || EB_FIX == WTNAF || EB_SIM == INTER || !defined(STRIP))
	compute_koblitz();
#endif
	eb_norm(g, g);
	eb_copy(&curve_g, g);
	bn_copy(&curve_r, r);
	bn_copy(&curve_h, h);
#if defined(EB_PRECO)
	eb_mul_pre(eb_curve_get_tab(), &curve_g);
#endif
}

#endif

#if defined(EB_SUPER)

void eb_curve_set_super(fb_t a, fb_t b, fb_t c, eb_t g, bn_t r, bn_t h) {
	curve_is_super = 1;

	fb_copy(curve_a, a);
	fb_copy(curve_b, b);
	fb_copy(curve_c, c);

	detect_opt(&curve_opt_a, curve_a);
	detect_opt(&curve_opt_b, curve_b);
	detect_opt(&curve_opt_c, curve_c);

	eb_norm(g, g);
	eb_copy(&curve_g, g);
	bn_copy(&curve_r, r);
	bn_copy(&curve_h, h);
#if defined(EB_PRECO)
	eb_mul_pre(eb_curve_get_tab(), &curve_g);
#endif
}

#endif
