/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
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
#include "relic_hb.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

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
	ctx_t *ctx = core_get();
#ifdef HB_PRECO
	for (int i = 0; i < HB_TABLE; i++) {
		ctx->hb_ptr[i] = &(ctx->hb_pre[i]);
	}
#endif
#if ALLOC == STATIC
	fb_new(ctx->hb_g.u1);
	fb_new(ctx->hb_g.u0);
	fb_new(ctx->hb_g.v1);
	fb_new(ctx->hb_g.v0);
	for (int i = 0; i < HB_TABLE; i++) {
		fb_new(ctx->hb_pre[i].u1);
		fb_new(ctx->hb_pre[i].u0);
		fb_new(ctx->hb_pre[i].v1);
		fb_new(ctx->hb_pre[i].v0);
	}
#endif
	fb_zero(ctx->hb_g.u1);
	fb_zero(ctx->hb_g.u0);
	fb_zero(ctx->hb_g.v1);
	fb_zero(ctx->hb_g.v0);
	bn_init(&(ctx->hb_r), FB_DIGS);
	bn_init(&(ctx->hb_h), FB_DIGS);
}

void hb_curve_clean(void) {
	ctx_t *ctx = core_get();
#if ALLOC == STATIC
	fb_free(ctx->hb_g.u1);
	fb_free(ctx->hb_g.u0);
	fb_free(ctx->hb_g.v1);
	fb_free(ctx->hb_g.v0);
	for (int i = 0; i < HB_TABLE; i++) {
		fb_free(ctx->hb_pre[i].u1);
		fb_free(ctx->hb_pre[i].u0);
		fb_free(ctx->hb_pre[i].v1);
		fb_free(ctx->hb_pre[i].v0);
	}
#endif
	bn_clean(&(ctx->hb_r));
	bn_clean(&(ctx->hb_h));
}

dig_t *hb_curve_get_f3() {
	return core_get()->hb_f3;
}

int hb_curve_opt_f3() {
	return core_get()->hb_opt_f3;
}

dig_t *hb_curve_get_f1() {
	return core_get()->hb_f1;
}

int hb_curve_opt_f1() {
	return core_get()->hb_opt_f1;
}

dig_t *hb_curve_get_f0() {
#if defined(HB_SUPER)
	if (core_get()->hb_is_super) {
		return core_get()->hb_f0;
	}
#endif
	return NULL;
}

int hb_curve_opt_f0() {
#if defined(HB_SUPER)
	if (core_get()->hb_is_super) {
		return core_get()->hb_opt_f0;
	}
#endif
	return OPT_NONE;
}

int hb_curve_is_super() {
	return core_get()->hb_is_super;
}

void hb_curve_get_gen(hb_t g) {
	hb_copy(g, &(core_get()->hb_g));
}

void hb_curve_get_ord(bn_t n) {
	bn_copy(n, &(core_get()->hb_r));
}

hb_t *hb_curve_get_tab() {
#if defined(HB_PRECO)

	/* Return a meaningful pointer. */
#if ALLOC == AUTO
	return (hb_t *)*(core_get()->hb_ptr);
#else
	return core_get()->hb_ptr;
#endif

#else
	/* Return a null pointer. */
	return NULL;
#endif
}

void hb_curve_get_cof(bn_t h) {
	bn_copy(h, &(core_get()->hb_h));
}

#if defined(HB_SUPER)

void hb_curve_set_super(fb_t f3, fb_t f1, fb_t f0, hb_t g, bn_t r, bn_t h) {
	ctx_t *ctx = core_get();

	ctx->hb_is_super = 1;

	fb_copy(ctx->hb_f3, f3);
	fb_copy(ctx->hb_f1, f1);
	fb_copy(ctx->hb_f0, f0);

	detect_opt(&(ctx->hb_opt_f3), ctx->hb_f3);
	detect_opt(&(ctx->hb_opt_f1), ctx->hb_f1);
	detect_opt(&(ctx->hb_opt_f0), ctx->hb_f0);

	hb_copy(&(ctx->hb_g), g);
	bn_copy(&(ctx->hb_r), r);
	bn_copy(&(ctx->hb_h), h);
#if defined(HB_PRECO)
	hb_mul_pre(hb_curve_get_tab(), &(ctx->hb_g));
#endif
}

#endif
