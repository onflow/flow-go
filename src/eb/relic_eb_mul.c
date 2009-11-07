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
 * Implementation of point multiplication on binary elliptic curves.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_eb.h"
#include "relic_error.h"

#if EB_MUL == WTNAF || !defined(STRIP)

#if defined(EB_KBLTZ)

/**
 * Precomputes a table for a point multiplication on a Koblitz curve.
 *
 * @param[out] t				- the destination table.
 * @param[in] p					- the point to multiply.
 */
static void table_init_koblitz(eb_t *t, eb_t p) {
	int u;

	if (eb_curve_opt_a() == OPT_ZERO) {
		u = -1;
	} else {
		u = 1;
	}

	/* Prepare the precomputation table. */
	for (int i = 0; i < 1 << (EB_WIDTH - 2); i++) {
		eb_set_infty(t[i]);
		fb_set_bit(t[i]->z, 0, 1);
		t[i]->norm = 1;
	}

	eb_copy(t[0], p);

	/* The minimum table depth for WTNAF is 3. */
#if EB_WIDTH == 3
	eb_frb_tab(t[1], t[0]);
	if (u == 1) {
		eb_sub_tab(t[1], t[0], t[1]);
	} else {
		eb_add_tab(t[1], t[0], t[1]);
	}
#endif

#if EB_WIDTH == 4
	eb_frb_tab(t[3], t[0]);
	eb_frb_tab(t[3], t[3]);

	eb_sub_tab(t[1], t[3], p);
	eb_add_tab(t[2], t[3], p);
	eb_frb_tab(t[3], t[3]);

	if (u == 1) {
		eb_neg_tab(t[3], t[3]);
	}
	eb_sub_tab(t[3], t[3], p);
#endif

#if EB_WIDTH == 5
	eb_frb_tab(t[3], t[0]);
	eb_frb_tab(t[3], t[3]);

	eb_sub_tab(t[1], t[3], p);
	eb_add_tab(t[2], t[3], p);
	eb_frb_tab(t[3], t[3]);

	if (u == 1) {
		eb_neg_tab(t[3], t[3]);
	}
	eb_sub_tab(t[3], t[3], p);

	eb_frb_tab(t[4], t[2]);
	eb_frb_tab(t[4], t[4]);

	eb_sub_tab(t[7], t[4], t[2]);

	eb_neg_tab(t[4], t[4]);
	eb_sub_tab(t[5], t[4], p);
	eb_add_tab(t[6], t[4], p);

	eb_frb_tab(t[4], t[4]);
	if (u == -1) {
		eb_neg_tab(t[4], t[4]);
	}
	eb_add_tab(t[4], t[4], p);
#endif

#if EB_WIDTH == 6
	eb_frb_tab(t[0], t[0]);
	eb_frb_tab(t[0], t[0]);
	eb_neg_tab(t[14], t[0]);

	eb_sub_tab(t[13], t[14], p);
	eb_add_tab(t[14], t[14], p);

	eb_frb_tab(t[0], t[0]);
	if (u == -1) {
		eb_neg_tab(t[0], t[0]);
	}
	eb_sub_tab(t[11], t[0], p);
	eb_add_tab(t[12], t[0], p);

	eb_frb_tab(t[0], t[12]);
	eb_frb_tab(t[0], t[0]);
	eb_sub_tab(t[1], t[0], p);
	eb_add_tab(t[2], t[0], p);

	eb_add_tab(t[15], t[0], t[13]);

	eb_frb_tab(t[0], t[13]);
	eb_frb_tab(t[0], t[0]);
	eb_sub_tab(t[5], t[0], p);
	eb_add_tab(t[6], t[0], p);

	eb_neg_tab(t[8], t[0]);
	eb_add_tab(t[7], t[8], t[13]);
	eb_add_tab(t[8], t[8], t[14]);

	eb_frb_tab(t[0], t[0]);
	if (u == -1) {
		eb_neg_tab(t[0], t[0]);
	}
	eb_sub_tab(t[3], t[0], p);
	eb_add_tab(t[4], t[0], p);

	eb_frb_tab(t[0], t[1]);
	eb_frb_tab(t[0], t[0]);

	eb_neg_tab(t[9], t[0]);
	eb_sub_tab(t[9], t[9], p);

	eb_frb_tab(t[0], t[14]);
	eb_frb_tab(t[0], t[0]);
	eb_add_tab(t[10], t[0], p);

	eb_copy(t[0], p);
#endif
}

/**
 * Multiplies a binary elliptic curve point by an integer using the w-TNAF
 * method.
 *
 * @param[out] r 				- the result.
 * @param[in] p					- the point to multiply.
 * @param[in] k					- the integer.
 */
static void eb_mul_tnaf_tab(eb_t r, eb_t p, bn_t k) {
	int len, i, n;
	signed char tnaf[FB_BITS + 8], *t, u;
	eb_t table[1 << (EB_WIDTH - 2)];
	bn_t vm, s0, s1;

	bn_null(vm);
	bn_null(s0);
	bn_null(s1);

	if (eb_curve_opt_a() == OPT_ZERO) {
		u = -1;
	} else {
		u = 1;
	}

	TRY {

		/* Prepare the precomputation table. */
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_new(table[i]);
		}
		/* Compute the precomputation table. */
		table_init_koblitz(table, p);

		vm = eb_curve_get_vm();
		s0 = eb_curve_get_s0();
		s1 = eb_curve_get_s1();
		/* Compute the w-TNAF representation of k. */
		bn_rec_tnaf(tnaf, &len, k, vm, s0, s1, u, FB_BITS, EB_WIDTH);

		t = tnaf + len - 1;
		eb_set_infty(r);
		for (i = len - 1; i >= 0; i--, t--) {
			eb_frb(r, r);

			n = *t;
			if (n > 0) {
				eb_add(r, r, table[n / 2]);
			}
			if (n < 0) {
				eb_sub(r, r, table[-n / 2]);
			}
		}
		/* Convert r to affine coordinates. */
		eb_norm(r, r);

	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		/* Free the precomputation table. */
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_free(table[i]);
		}
	}
}

#endif /* EB_KBLTZ */

/* Support for ordinary curves. */
#if defined(EB_ORDIN) || defined(EB_SUPER)

/**
 * Precomputes a table for a point multiplication on an ordinary curve.
 *
 * @param[out] t				- the destination table.
 * @param[in] p					- the point to multiply.
 */
static void table_init_ordin(eb_t *t, eb_t p) {
	int i;

	for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
		eb_set_infty(t[i]);
		fb_set_bit(t[i]->z, 0, 1);
		t[i]->norm = 1;
	}

	eb_dbl_tab(t[0], p);

#if EB_WIDTH > 2
	eb_add_tab(t[1], t[0], p);
	for (i = 2; i < (1 << (EB_WIDTH - 2)); i++) {
		eb_add_tab(t[i], t[i - 1], t[0]);
	}
#endif

	eb_copy(t[0], p);
}

static void eb_mul_naf_tab(eb_t r, eb_t p, bn_t k) {
	int len, i, n;
	signed char naf[FB_BITS + 1], *t;
	eb_t table[1 << (EB_WIDTH - 2)];

	for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
		eb_null(table[i]);
	}

	TRY {
		/* Prepare the precomputation table. */
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_new(table[i]);
		}
		/* Compute the precomputation table. */
		table_init_ordin(table, p);

		/* Compute the w-TNAF representation of k. */
		bn_rec_naf(naf, &len, k, EB_WIDTH);

		t = naf + len - 1;

		eb_set_infty(r);
		for (i = len - 1; i >= 0; i--, t--) {
			eb_dbl(r, r);

			n = *t;
			if (n > 0) {
				eb_add(r, r, table[n / 2]);
			}
			if (n < 0) {
				eb_sub(r, r, table[-n / 2]);
			}
		}
		/* Convert r to affine coordinates. */
		eb_norm(r, r);

	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		/* Free the precomputation table. */
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_free(table[i]);
		}
	}
}

#endif /* EB_ORDIN || EB_SUPER */
#endif /* EB_MUL == WTNAF */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EB_MUL == BASIC || !defined(STRIP)

void eb_mul_basic(eb_t r, eb_t p, bn_t k) {
	int i, l;
	eb_t t;

	eb_null(t);

	TRY {
		eb_new(t);

		l = bn_bits(k);

		eb_copy(t, p);

		for (i = l - 2; i >= 0; i--) {
			eb_dbl(t, t);
			if (bn_test_bit(k, i)) {
				eb_add(t, t, p);
			}
		}

		eb_copy(r, t);
		eb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		eb_free(t);
	}
}

#endif

#if defined(EB_ORDIN) || defined(EB_KBLTZ)

#if EB_MUL == CONST || !defined(STRIP)

void eb_mul_const(eb_t r, eb_t p, bn_t k) {
	int i, t, koblitz;
	fb_t x1, z1, x2, z2, r1, r2, r3, r4, b;

	fb_null(x1);
	fb_null(z1);
	fb_null(x2);
	fb_null(z2);
	fb_null(r1);
	fb_null(r2);
	fb_null(r3);
	fb_null(r4);
	fb_null(b);

	TRY {
		fb_new(x1);
		fb_new(z1);
		fb_new(x2);
		fb_new(z2);
		fb_new(r1);
		fb_new(r2);
		fb_new(r3);
		fb_new(r4);

		fb_copy(x1, p->x);
		fb_zero(z1);
		fb_set_bit(z1, 0, 1);
		fb_sqr(z2, p->x);
		fb_sqr(x2, z2);

		b = eb_curve_get_b();

		koblitz = eb_curve_is_kbltz();

		if (!koblitz) {
			fb_add(x2, x2, b);
		} else {
			fb_add_dig(x2, x2, (dig_t)1);
		}

		t = bn_bits(k);
		for (i = t - 2; i >= 0; i--) {
			fb_mul(r1, x1, z2);
			fb_mul(r2, x2, z1);
			fb_add(r3, r1, r2);
			fb_mul(r4, r1, r2);
			if (bn_get_bit(k, i) == 1) {
				fb_sqr(z1, r3);
				fb_mul(r1, z1, p->x);
				fb_add(x1, r1, r4);
				fb_sqr(r1, z2);
				fb_sqr(r2, x2);
				fb_mul(z2, r1, r2);
				if (!koblitz) {
					fb_sqr(x2, r2);
					fb_sqr(r1, r1);
					fb_mul(r2, r1, b);
					fb_add(x2, x2, r2);
				} else {
					fb_add(r1, r1, r2);
					fb_sqr(x2, r1);
				}
			} else {
				fb_sqr(z2, r3);
				fb_mul(r1, z2, p->x);
				fb_add(x2, r1, r4);
				fb_sqr(r1, z1);
				fb_sqr(r2, x1);
				fb_mul(z1, r1, r2);
				if (!koblitz) {
					fb_sqr(x1, r2);
					fb_sqr(r1, r1);
					fb_mul(r2, r1, b);
					fb_add(x1, x1, r2);
				} else {
					fb_add(r1, r1, r2);
					fb_sqr(x1, r1);
				}
			}
		}

		if (fb_is_zero(z1)) {
			/* The point q is at infinity. */
			eb_set_infty(r);
		} else {
			if (fb_is_zero(z2)) {
				fb_copy(r->x, p->x);
				fb_add(r->y, p->x, p->y);
				fb_zero(r->z);
				fb_set_bit(r->z, 0, 1);
			} else {
				fb_mul(r3, z1, z2);
				fb_mul(z1, z1, p->x);
				fb_add(z1, z1, x1);
				fb_mul(z2, z2, p->x);
				fb_mul(x1, x1, z2);
				fb_add(z2, z2, x2);

				fb_mul(z2, z2, z1);
				fb_sqr(r4, p->x);
				fb_add(r4, r4, p->y);
				fb_mul(r4, r4, r3);
				fb_add(r4, r4, z2);

				fb_mul(r3, r3, p->x);
				fb_inv(r3, r3);
				fb_mul(r4, r4, r3);
				fb_mul(x2, x1, r3);
				fb_add(z2, x2, p->x);

				fb_mul(z2, z2, r4);
				fb_add(z2, z2, p->y);

				fb_copy(r->x, x2);
				fb_copy(r->y, z2);
				fb_zero(r->z);
				fb_set_bit(r->z, 0, 1);

				r->norm = 1;
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(x1);
		fb_free(z1);
		fb_free(x2);
		fb_free(z2);
		fb_free(r1);
		fb_free(r2);
		fb_free(r3);
		fb_free(r4);
	}
}

#endif /* EB_ORDIN || EB_KBLTZ */
#endif /* EB_MUL == CONST */

#if EB_MUL == WTNAF || !defined(STRIP)

void eb_mul_wtnaf(eb_t r, eb_t p, bn_t k) {
#if defined(EB_KBLTZ)
	if (eb_curve_is_kbltz()) {
		eb_mul_tnaf_tab(r, p, k);
		return;
	}
#endif

#if defined(EB_ORDIN) || defined(EB_SUPER)
	eb_mul_naf_tab(r, p, k);
#endif
}

#endif

void eb_mul_gen(eb_t r, bn_t k) {
#ifdef EB_PRECO
	eb_mul_fix(r, eb_curve_get_tab(), k);
#else
	eb_mul(r, eb_curve_get_gen(), k);
#endif
}
