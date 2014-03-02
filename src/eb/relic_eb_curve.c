/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2014 RELIC Authors
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

#include "relic_core.h"
#include "relic_eb.h"
#include "relic_conf.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if defined(EB_KBLTZ) && (EB_MUL == LWNAF || EB_MUL == RWNAF || EB_FIX == LWNAF || EB_SIM == INTER || !defined(STRIP))

/**
 * Precomputes additional parameters for Koblitz curves used by the w-TNAF
 * multiplication algorithm.
 */
static void compute_kbltz(void) {
	int u, i;
	bn_t a, b, c;
	ctx_t *ctx = core_get();

	bn_null(a);
	bn_null(b);
	bn_null(c);

	TRY {
		bn_new(a);
		bn_new(b);
		bn_new(c);

		if (ctx->eb_opt_a == OPT_ZERO) {
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
		bn_copy(&(ctx->eb_vm), b);

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
		bn_copy(&(ctx->eb_s0), b);

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
		bn_copy(&(ctx->eb_s1), b);
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
	ctx_t *ctx = core_get();
#ifdef EB_PRECO
	for (int i = 0; i < EB_TABLE; i++) {
		ctx->eb_ptr[i] = &(ctx->eb_pre[i]);
	}
#endif
#if ALLOC == STATIC
	fb_new(ctx->eb_g.x);
	fb_new(ctx->eb_g.y);
	fb_new(ctx->eb_g.z);
	for (int i = 0; i < EB_TABLE; i++) {
		fb_new(ctx->eb_pre[i].x);
		fb_new(ctx->eb_pre[i].y);
		fb_new(ctx->eb_pre[i].z);
	}
#endif
	fb_zero(ctx->eb_g.x);
	fb_zero(ctx->eb_g.y);
	fb_zero(ctx->eb_g.z);
	bn_init(&(ctx->eb_r), FB_DIGS);
	bn_init(&(ctx->eb_h), FB_DIGS);
#if defined(EB_KBLTZ) && (EB_MUL == LWNAF || !defined(STRIP))
	bn_init(&(ctx->eb_vm), FB_DIGS);
	bn_init(&(ctx->eb_s0), FB_DIGS);
	bn_init(&(ctx->eb_s1), FB_DIGS);
#endif
}

void eb_curve_clean(void) {
	ctx_t *ctx = core_get();
#if ALLOC == STATIC
	fb_free(ctx->eb_g.x);
	fb_free(ctx->eb_g.y);
	fb_free(ctx->eb_g.z);
	for (int i = 0; i < EB_TABLE; i++) {
		fb_free(ctx->eb_pre[i].x);
		fb_free(ctx->eb_pre[i].y);
		fb_free(ctx->eb_pre[i].z);
	}
#endif
	bn_clean(&(ctx->eb_r));
	bn_clean(&(ctx->eb_h));
#if defined(EB_KBLTZ) && (EB_MUL == LWNAF || !defined(STRIP))
	bn_clean(&(ctx->eb_vm));
	bn_clean(&(ctx->eb_s0));
	bn_clean(&(ctx->eb_s1));
#endif
}

dig_t *eb_curve_get_a() {
	return core_get()->eb_a;
}

int eb_curve_opt_a() {
	return core_get()->eb_opt_a;
}

dig_t *eb_curve_get_b() {
	return core_get()->eb_b;
}

int eb_curve_opt_b() {
	return core_get()->eb_opt_b;
}

int eb_curve_is_kbltz() {
	return core_get()->eb_is_kbltz;
}

void eb_curve_get_gen(eb_t g) {
	eb_copy(g, &(core_get()->eb_g));
}

void eb_curve_get_ord(bn_t n) {
	bn_copy(n, &(core_get()->eb_r));
}

void eb_curve_get_cof(bn_t h) {
	bn_copy(h, &(core_get()->eb_h));
}

#if defined(EB_KBLTZ) && (EB_MUL == LWNAF || EB_FIX == LWNAF || EB_SIM == INTER || !defined(STRIP))
void eb_curve_get_vm(bn_t vm) {
	if (core_get()->eb_is_kbltz) {
		bn_copy(vm, &(core_get()->eb_vm));
	} else {
		bn_zero(vm);
	}
}

void eb_curve_get_s0(bn_t s0) {
	if (core_get()->eb_is_kbltz) {
		bn_copy(s0, &(core_get()->eb_s0));
	} else {
		bn_zero(s0);
	}
}

void eb_curve_get_s1(bn_t s1) {
	if (core_get()->eb_is_kbltz) {
		bn_copy(s1, &(core_get()->eb_s1));
	} else {
		bn_zero(s1);
	}
}
#endif

const eb_t *eb_curve_get_tab() {
#if defined(EB_PRECO)

	/* Return a meaningful pointer. */
#if ALLOC == AUTO
	return (const eb_t *)*(core_get()->eb_ptr);
#else
	return (const eb_t *)core_get()->eb_ptr;
#endif

#else
	/* Return a null pointer. */
	return NULL;
#endif
}

void eb_curve_set(const fb_t a, const fb_t b, const eb_t g, const bn_t r, 
		const bn_t h) {
	ctx_t *ctx = core_get();
	fb_copy(ctx->eb_a, a);
	fb_copy(ctx->eb_b, b);

	detect_opt(&(ctx->eb_opt_a), ctx->eb_a);
	detect_opt(&(ctx->eb_opt_b), ctx->eb_b);

	if (fb_cmp_dig(ctx->eb_b, 1) == CMP_EQ) {
		ctx->eb_is_kbltz = 1;
	} else {
		ctx->eb_is_kbltz = 0;
	}
#if defined(EB_KBLTZ) && (EB_MUL == LWNAF || EB_FIX == LWNAF || EB_SIM == INTER || !defined(STRIP))
	if (ctx->eb_is_kbltz) {
		compute_kbltz();
	}
#endif
	eb_norm(&(ctx->eb_g), g);
	bn_copy(&(ctx->eb_r), r);
	bn_copy(&(ctx->eb_h), h);
#if defined(EB_PRECO)
	eb_mul_pre((eb_t *)eb_curve_get_tab(), &(ctx->eb_g));
#endif
}
