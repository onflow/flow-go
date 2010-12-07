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

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if EB_MUL == LWNAF || !defined(STRIP)

#if defined(EB_KBLTZ)

/**
 * Precomputes a table for a point multiplication on a Koblitz curve.
 *
 * @param[out] t				- the destination table.
 * @param[in] p					- the point to multiply.
 */
static void eb_table_kbltz(eb_t *t, eb_t p) {
	int u;

	if (eb_curve_opt_a() == OPT_ZERO) {
		u = -1;
	} else {
		u = 1;
	}

	/* Prepare the precomputation table. */
	for (int i = 0; i < 1 << (EB_WIDTH - 2); i++) {
		eb_set_infty(t[i]);
		fb_set_dig(t[i]->z, 1);
		t[i]->norm = 1;
	}

	eb_copy(t[0], p);

	/* The minimum table depth for WTNAF is 3. */
#if EB_WIDTH == 3
	eb_frb(t[1], t[0]);
	if (u == 1) {
		eb_sub(t[1], t[0], t[1]);
	} else {
		eb_add(t[1], t[0], t[1]);
	}
#endif

#if EB_WIDTH == 4
	eb_frb(t[3], t[0]);
	eb_frb(t[3], t[3]);

	eb_sub(t[1], t[3], p);
	eb_add(t[2], t[3], p);
	eb_frb(t[3], t[3]);

	if (u == 1) {
		eb_neg(t[3], t[3]);
	}
	eb_sub(t[3], t[3], p);
#endif

#if EB_WIDTH == 5
	eb_frb(t[3], t[0]);
	eb_frb(t[3], t[3]);

	eb_sub(t[1], t[3], p);
	eb_add(t[2], t[3], p);
	eb_frb(t[3], t[3]);

	if (u == 1) {
		eb_neg(t[3], t[3]);
	}
	eb_sub(t[3], t[3], p);

	eb_frb(t[4], t[2]);
	eb_frb(t[4], t[4]);

	eb_sub(t[7], t[4], t[2]);

	eb_neg(t[4], t[4]);
	eb_sub(t[5], t[4], p);
	eb_add(t[6], t[4], p);

	eb_frb(t[4], t[4]);
	if (u == -1) {
		eb_neg(t[4], t[4]);
	}
	eb_add(t[4], t[4], p);
#endif

#if EB_WIDTH == 6
	eb_frb(t[0], t[0]);
	eb_frb(t[0], t[0]);
	eb_neg(t[14], t[0]);

	eb_sub(t[13], t[14], p);
	eb_add(t[14], t[14], p);

	eb_frb(t[0], t[0]);
	if (u == -1) {
		eb_neg(t[0], t[0]);
	}
	eb_sub(t[11], t[0], p);
	eb_add(t[12], t[0], p);

	eb_frb(t[0], t[12]);
	eb_frb(t[0], t[0]);
	eb_sub(t[1], t[0], p);
	eb_add(t[2], t[0], p);

	eb_add(t[15], t[0], t[13]);

	eb_frb(t[0], t[13]);
	eb_frb(t[0], t[0]);
	eb_sub(t[5], t[0], p);
	eb_add(t[6], t[0], p);

	eb_neg(t[8], t[0]);
	eb_add(t[7], t[8], t[13]);
	eb_add(t[8], t[8], t[14]);

	eb_frb(t[0], t[0]);
	if (u == -1) {
		eb_neg(t[0], t[0]);
	}
	eb_sub(t[3], t[0], p);
	eb_add(t[4], t[0], p);

	eb_frb(t[0], t[1]);
	eb_frb(t[0], t[0]);

	eb_neg(t[9], t[0]);
	eb_sub(t[9], t[9], p);

	eb_frb(t[0], t[14]);
	eb_frb(t[0], t[0]);
	eb_add(t[10], t[0], p);

	eb_copy(t[0], p);
#endif

#if EB_WIDTH > 2 && defined(EB_MIXED)
	eb_norm_sim(t + 1, t + 1, (1 << (EB_WIDTH - 2)) - 1);
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
static void eb_mul_ltnaf_impl(eb_t r, eb_t p, bn_t k) {
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
		bn_new(vm);
		bn_new(s0);
		bn_new(s1);

		/* Prepare the precomputation table. */
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_new(table[i]);
		}
		/* Compute the precomputation table. */
		eb_table_kbltz(table, p);

		eb_curve_get_vm(vm);
		eb_curve_get_s0(s0);
		eb_curve_get_s1(s1);
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
		bn_free(vm);
		bn_free(s0);
		bn_free(s1);

		/* Free the precomputation table. */
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_free(table[i]);
		}
	}
}

#endif

#if defined(EB_ORDIN) || defined(EB_SUPER)

/**
 * Precomputes a table for a point multiplication on an ordinary curve.
 *
 * @param[out] t				- the destination table.
 * @param[in] p					- the point to multiply.
 */
static void eb_table_ordin(eb_t *t, eb_t p) {
#if EB_WIDTH > 2
	eb_dbl(t[0], p);
	eb_add(t[1], t[0], p);
	for (int i = 2; i < (1 << (EB_WIDTH - 2)); i++) {
		eb_add(t[i], t[i - 1], t[0]);
	}
#if defined(EB_MIXED)
	eb_norm_sim(t + 1, t + 1, (1 << (EB_WIDTH - 2)) - 1);
#endif
#endif
	eb_copy(t[0], p);
}

/**
 * Multiplies a binary elliptic curve point by an integer using the
 * left-to-right w-NAF method.
 *
 * @param[out] r 				- the result.
 * @param[in] p					- the point to multiply.
 * @param[in] k					- the integer.
 */
static void eb_mul_lnaf_impl(eb_t r, eb_t p, bn_t k) {
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
			eb_set_infty(table[i]);
			fb_set_dig(table[i]->z, 1);
			table[i]->norm = 1;
		}
		/* Compute the precomputation table. */
		eb_table_ordin(table, p);

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
#endif /* EB_MUL == LWNAF */

#if EB_MUL == RWNAF || !defined(STRIP)

#if defined(EB_KBLTZ)

/**
 * Multiplies a binary elliptic curve point by an integer using the w-TNAF
 * method.
 *
 * @param[out] r 				- the result.
 * @param[in] p					- the point to multiply.
 * @param[in] k					- the integer.
 */
static void eb_mul_rtnaf_impl(eb_t r, eb_t p, bn_t k) {
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
		bn_new(vm);
		bn_new(s0);
		bn_new(s1);

		/* Prepare the precomputation table. */
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_new(table[i]);
			eb_set_infty(table[i]);
		}
		/* Compute the precomputation table. */

		eb_curve_get_vm(vm);
		eb_curve_get_s0(s0);
		eb_curve_get_s1(s1);
		/* Compute the w-TNAF representation of k. */
		bn_rec_tnaf(tnaf, &len, k, vm, s0, s1, u, FB_BITS, EB_WIDTH);

		t = tnaf;
		eb_copy(r, p);
		for (i = 0; i < len; i++, t++) {
			n = *t;
			if (n > 0) {
				eb_add(table[n / 2], table[n / 2], r);
			}
			if (n < 0) {
				eb_sub(table[-n / 2], table[-n / 2], r);
			}

			/* We can avoid a function call here. */
			fb_sqr(r->x, r->x);
			fb_sqr(r->y, r->y);
		}

		eb_copy(r, table[0]);

#if EB_WIDTH == 3
		eb_frb(table[0], table[1]);
		if (u == 1) {
			eb_sub(table[1], table[1], table[0]);
		} else {
			eb_add(table[1], table[1], table[0]);
		}
#endif

#if EB_WIDTH == 4
		eb_frb(table[0], table[3]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);

		if (u == 1) {
			eb_neg(table[0], table[0]);
		}
		eb_sub(table[3], table[0], table[3]);

		eb_frb(table[0], table[1]);
		eb_frb(table[0], table[0]);
		eb_sub(table[1], table[0], table[1]);

		eb_frb(table[0], table[2]);
		eb_frb(table[0], table[0]);
		eb_add(table[2], table[0], table[2]);
#endif

#if EB_WIDTH == 5
		eb_frb(table[0], table[3]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		if (u == 1) {
			eb_neg(table[0], table[0]);
		}
		eb_sub(table[3], table[0], table[3]);

		eb_frb(table[0], table[1]);
		eb_frb(table[0], table[0]);
		eb_sub(table[1], table[0], table[1]);

		eb_frb(table[0], table[2]);
		eb_frb(table[0], table[0]);
		eb_add(table[2], table[0], table[2]);

		eb_frb(table[0], table[4]);
		eb_frb(table[0], table[0]);
		eb_add(table[0], table[0], table[4]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		if (u == 1) {
			eb_neg(table[0], table[0]);
		}
		eb_add(table[4], table[0], table[4]);

		eb_frb(table[0], table[5]);
		eb_frb(table[0], table[0]);
		eb_add(table[0], table[0], table[5]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_neg(table[0], table[0]);
		eb_sub(table[5], table[0], table[5]);

		eb_frb(table[0], table[6]);
		eb_frb(table[0], table[0]);
		eb_add(table[0], table[0], table[6]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_neg(table[0], table[0]);
		eb_add(table[6], table[0], table[6]);

		eb_frb(table[0], table[7]);
		eb_frb(table[0], table[0]);
		eb_add(table[0], table[0], table[7]);
		eb_frb(table[7], table[0]);
		eb_frb(table[7], table[7]);
		eb_sub(table[7], table[7], table[0]);
#endif

#if EB_WIDTH == 6
		eb_frb(table[0], table[1]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		if (u == -1) {
			eb_neg(table[0], table[0]);
		}
		eb_add(table[0], table[0], table[1]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_sub(table[1], table[0], table[1]);

		eb_frb(table[0], table[2]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		if (u == -1) {
			eb_neg(table[0], table[0]);
		}
		eb_add(table[0], table[0], table[2]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_add(table[2], table[0], table[2]);

		eb_frb(table[0], table[3]);
		eb_frb(table[0], table[0]);
		eb_add(table[0], table[0], table[3]);
		eb_neg(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		if (u == -1) {
			eb_neg(table[0], table[0]);
		}
		eb_sub(table[3], table[0], table[3]);

		eb_frb(table[0], table[4]);
		eb_frb(table[0], table[0]);
		eb_add(table[0], table[0], table[4]);
		eb_neg(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		if (u == -1) {
			eb_neg(table[0], table[0]);
		}
		eb_add(table[4], table[0], table[4]);

		eb_frb(table[0], table[5]);
		eb_frb(table[0], table[0]);
		eb_add(table[0], table[0], table[5]);
		eb_neg(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_sub(table[5], table[0], table[5]);

		eb_frb(table[0], table[6]);
		eb_frb(table[0], table[0]);
		eb_add(table[0], table[0], table[6]);
		eb_neg(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_add(table[6], table[0], table[6]);

		eb_frb(table[0], table[7]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_sub(table[7], table[0], table[7]);

		eb_frb(table[0], table[8]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_add(table[8], table[0], table[8]);

		eb_frb(table[0], table[9]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		if (u == -1) {
			eb_neg(table[0], table[0]);
		}
		eb_add(table[0], table[0], table[9]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_sub(table[0], table[0], table[9]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_add(table[0], table[0], table[9]);
		eb_neg(table[9], table[0]);

		eb_frb(table[0], table[10]);
		eb_frb(table[0], table[0]);
		eb_neg(table[0], table[0]);
		eb_add(table[0], table[0], table[10]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_add(table[10], table[0], table[10]);

		eb_frb(table[0], table[11]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		if (u == -1) {
			eb_neg(table[0], table[0]);
		}
		eb_sub(table[11], table[0], table[11]);

		eb_frb(table[0], table[12]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		if (u == -1) {
			eb_neg(table[0], table[0]);
		}
		eb_add(table[12], table[0], table[12]);

		eb_frb(table[0], table[13]);
		eb_frb(table[0], table[0]);
		eb_add(table[0], table[0], table[13]);
		eb_neg(table[13], table[0]);

		eb_frb(table[0], table[14]);
		eb_frb(table[0], table[0]);
		eb_neg(table[0], table[0]);
		eb_add(table[14], table[0], table[14]);

		eb_frb(table[0], table[15]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		eb_frb(table[0], table[0]);
		if (u == -1) {
			eb_neg(table[0], table[0]);
		}
		eb_sub(table[15], table[0], table[15]);
#endif

		/* Sum accumulators */
		for (i = 1; i < (1 << (EB_WIDTH - 2)); i++) {
			if (r->norm) {
				eb_add(r, table[i], r);
			} else {
				eb_add(r, r, table[i]);
			}
		}
		/* Convert r to affine coordinates. */
		eb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(vm);
		bn_free(s0);
		bn_free(s1);

		/* Free the precomputation table. */
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_free(table[i]);
		}
	}
}

#endif /* EB_KBLTZ */

#if defined(EB_ORDIN) || defined(EB_SUPER)

/**
 * Multiplies a binary elliptic curve point by an integer using the
 * right-to-left w-NAF method.
 *
 * @param[out] r 				- the result.
 * @param[in] p					- the point to multiply.
 * @param[in] k					- the integer.
 */
static void eb_mul_rnaf_impl(eb_t r, eb_t p, bn_t k) {
	int len, i, n;
	signed char naf[FB_BITS + 1], *t;
	eb_t table[1 << (EB_WIDTH - 2)];

	for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
		eb_null(table[i]);
	}

	TRY {
		/* Prepare the accumulator table. */
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_new(table[i]);
			eb_set_infty(table[i]);
		}

		/* Compute the w-TNAF representation of k. */
		bn_rec_naf(naf, &len, k, EB_WIDTH);

		t = naf;

		eb_copy(r, p);
		for (i = 0; i < len; i++, t++) {
			n = *t;
			if (n > 0) {
				eb_add(table[n / 2], table[n / 2], r);
			}
			if (n < 0) {
				eb_sub(table[-n / 2], table[-n / 2], r);
			}

			eb_dbl(r, r);
		}

		eb_copy(r, table[0]);

#if EB_WIDTH >= 3
		/* Compute 3 * T[1]. */
		eb_dbl(table[0], table[1]);
		eb_add(table[1], table[0], table[1]);
#endif
#if EB_WIDTH >= 4
		/* Compute 5 * T[2]. */
		eb_dbl(table[0], table[2]);
		eb_dbl(table[0], table[0]);
		eb_add(table[2], table[0], table[2]);

		/* Compute 7 * T[3]. */
		eb_dbl(table[0], table[3]);
		eb_dbl(table[0], table[0]);
		eb_dbl(table[0], table[0]);
		eb_sub(table[3], table[0], table[3]);
#endif
#if EB_WIDTH >= 5
		/* Compute 9 * T[4]. */
		eb_dbl(table[0], table[4]);
		eb_dbl(table[0], table[0]);
		eb_dbl(table[0], table[0]);
		eb_add(table[4], table[0], table[4]);

		/* Compute 11 * T[5]. */
		eb_dbl(table[0], table[5]);
		eb_dbl(table[0], table[0]);
		eb_add(table[0], table[0], table[5]);
		eb_dbl(table[0], table[0]);
		eb_add(table[5], table[0], table[5]);

		/* Compute 13 * T[6]. */
		eb_dbl(table[0], table[6]);
		eb_add(table[0], table[0], table[6]);
		eb_dbl(table[0], table[0]);
		eb_dbl(table[0], table[0]);
		eb_add(table[6], table[0], table[6]);

		/* Compute 15 * T[7]. */
		eb_dbl(table[0], table[7]);
		eb_dbl(table[0], table[0]);
		eb_dbl(table[0], table[0]);
		eb_dbl(table[0], table[0]);
		eb_sub(table[7], table[0], table[7]);
#endif
#if EB_WIDTH == 6
		for (i = 8; i < 15; i++) {
			eb_mul_dig(table[i], table[i], 2 * i + 1);
		}
		eb_dbl(table[0], table[15]);
		eb_dbl(table[0], table[0]);
		eb_dbl(table[0], table[0]);
		eb_dbl(table[0], table[0]);
		eb_dbl(table[0], table[0]);
		eb_sub(table[15], table[0], table[15]);
#endif

		/* Sum accumulators */
		for (i = 1; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_add(r, r, table[i]);
		}
		/* Convert r to affine coordinates. */
		eb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		/* Free the accumulator table. */
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_free(table[i]);
		}
	}
}

#endif /* EB_ORDIN || EB_SUPER */
#endif /* EB_MUL == RWNAF */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EB_MUL == BASIC || !defined(STRIP)

void eb_mul_basic(eb_t r, eb_t p, bn_t k) {
	int i, l;
	eb_t t;

	eb_null(t);

	if (bn_is_zero(k)) {
		eb_set_infty(r);
		return;
	}

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

		eb_norm(r, t);
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

#if EB_MUL == LODAH || !defined(STRIP)

void eb_mul_lodah(eb_t r, eb_t p, bn_t k) {
	int i, t;
	fb_t x1, z1, x2, z2, r1, r2, r3, r4;
	dig_t *b;

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

		switch (eb_curve_opt_b()) {
			case OPT_ZERO:
				break;
			case OPT_ONE:
				fb_add_dig(x2, x2, (dig_t)1);
				break;
			case OPT_DIGIT:
				fb_add_dig(x2, x2, b[0]);
				break;
			default:
				fb_add(x2, x2, b);
				break;
		}

		t = bn_bits(k);
		for (i = t - 2; i >= 0; i--) {
			fb_mul(r1, x1, z2);
			fb_mul(r2, x2, z1);
			fb_add(r3, r1, r2);
			fb_mul(r4, r1, r2);
			if (bn_test_bit(k, i) == 1) {
				fb_sqr(z1, r3);
				fb_mul(r1, z1, p->x);
				fb_add(x1, r1, r4);
				fb_sqr(r1, z2);
				fb_sqr(r2, x2);
				fb_mul(z2, r1, r2);
				switch (eb_curve_opt_b()) {
					case OPT_ZERO:
						fb_sqr(x2, r2);
						break;
					case OPT_ONE:
						fb_add(r1, r1, r2);
						fb_sqr(x2, r1);
						break;
					case OPT_DIGIT:
						fb_sqr(x2, r2);
						fb_sqr(r1, r1);
						fb_mul_dig(r2, r1, b[0]);
						fb_add(x2, x2, r2);
						break;
					default:
						fb_sqr(x2, r2);
						fb_sqr(r1, r1);
						fb_mul(r2, r1, b);
						fb_add(x2, x2, r2);
						break;
				}
			} else {
				fb_sqr(z2, r3);
				fb_mul(r1, z2, p->x);
				fb_add(x2, r1, r4);
				fb_sqr(r1, z1);
				fb_sqr(r2, x1);
				fb_mul(z1, r1, r2);
				switch (eb_curve_opt_b()) {
					case OPT_ZERO:
						fb_sqr(x1, r2);
						break;
					case OPT_ONE:
						fb_add(r1, r1, r2);
						fb_sqr(x1, r1);
						break;
					case OPT_DIGIT:
						fb_sqr(x1, r2);
						fb_sqr(r1, r1);
						fb_mul_dig(r2, r1, b[0]);
						fb_add(x1, x1, r2);
						break;
					default:
						fb_sqr(x1, r2);
						fb_sqr(r1, r1);
						fb_mul(r2, r1, b);
						fb_add(x1, x1, r2);
						break;
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
				/* r3 = z1 * z2. */
				fb_mul(r3, z1, z2);
				/* z1 = (x1 + x * z1). */
				fb_mul(z1, z1, p->x);
				fb_add(z1, z1, x1);
				/* z2 = x * z2. */
				fb_mul(z2, z2, p->x);
				/* x1 = x1 * z2. */
				fb_mul(x1, x1, z2);
				/* z2 = (x2 + x * z2)(x1 + x * z1). */
				fb_add(z2, z2, x2);
				fb_mul(z2, z2, z1);

				/* r4 = (x^2 + y) * z1 * z2 + (x2 + x * z2)(x1 + x * z1). */
				fb_sqr(r4, p->x);
				fb_add(r4, r4, p->y);
				fb_mul(r4, r4, r3);
				fb_add(r4, r4, z2);

				/* r3 = (z1 * z2 * x)^{-1}. */
				fb_mul(r3, r3, p->x);
				fb_inv(r3, r3);
				/* r4 = (x^2 + y) * z1 * z2 + (x2 + x * z2)(x1 + x * z1) * r3. */
				fb_mul(r4, r4, r3);
				/* x2 = x1 * x * z2 * (z1 * z2 * x)^{-1} = x1/z1. */
				fb_mul(x2, x1, r3);
				/* z2 = x + x1/z1. */
				fb_add(z2, x2, p->x);

				/* z2 = z2 * r4 + y. */
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
#endif /* EB_MUL == LODAH */

#if EB_MUL == LWNAF || !defined(STRIP)

void eb_mul_lwnaf(eb_t r, eb_t p, bn_t k) {
#if defined(EB_KBLTZ)
	if (eb_curve_is_kbltz()) {
		eb_mul_ltnaf_impl(r, p, k);
		return;
	}
#endif

#if defined(EB_ORDIN) || defined(EB_SUPER)
	eb_mul_lnaf_impl(r, p, k);
#endif
}

#endif

#if EB_MUL == RWNAF || !defined(STRIP)

void eb_mul_rwnaf(eb_t r, eb_t p, bn_t k) {
#if defined(EB_KBLTZ)
	if (eb_curve_is_kbltz()) {
		eb_mul_rtnaf_impl(r, p, k);
		return;
	}
#endif

#if defined(EB_ORDIN) || defined(EB_SUPER)
	eb_mul_rnaf_impl(r, p, k);
#endif
}

#endif

#if EB_MUL == HALVE || !defined(STRIP)

void eb_mul_halve(eb_t r, eb_t p, bn_t k) {
	int len, i, j;
	signed char naf[FB_BITS + 1] = { 0 }, *tmp;
	eb_t q, table[1 << (EB_WIDTH - 2)];
	bn_t n, _k;

	bn_null(_k);
	bn_null(n);
	eb_null(q);
	for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
		eb_null(table[i]);
	}

	if (fb_is_zero(eb_curve_get_a())) {
		THROW(ERR_INVALID);
	}

	TRY {
		bn_new(n);
		bn_new(_k);
		eb_new(q);

		/* Prepare the precomputation table. */
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_new(table[i]);
			eb_set_infty(table[i]);
		}

		/* Convert k to alternate representation k' = (2^{t-1}k mod n). */
		eb_curve_get_ord(n);
		bn_lsh(_k, k, bn_bits(n) - 1);
		bn_mod(_k, _k, n);

		/* Compute the w-NAF representation of k'. */
		bn_rec_naf(naf, &len, _k, EB_WIDTH);

		for (i = len; i <= bn_bits(n); i++) {
			naf[i] = 0;
		}
		if (naf[bn_bits(n)] == 1) {
			eb_dbl(table[0], p);
		}
		len = bn_bits(n);
		tmp = naf + len - 1;

		eb_copy(q, p);
		for (i = len - 1; i >= 0; i--, tmp--) {
			j = *tmp;
			if (j > 0) {
				eb_norm(q, q);
				eb_add(table[j / 2], table[j / 2], q);
			}
			if (j < 0) {
				eb_norm(q, q);
				eb_sub(table[-j / 2], table[-j / 2], q);
			}
			eb_hlv(q, q);
		}

#if EB_WIDTH == 2
		eb_norm(r, table[0]);
#else
		/* Compute Q_i = Q_i + Q_{i+2} for i from 2^{w-1}-3 to 1. */
		for (i = (1 << (EB_WIDTH - 1)) - 3; i >= 1; i -= 2) {
			eb_add(table[i / 2], table[i / 2], table[(i + 2) / 2]);
		}
		/* Compute R = Q_1 + 2 * sum_{i != 1}Q_i. */
		eb_copy(r, table[1]);
		for (i = 2; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_add(r, r, table[i]);
		}
		eb_dbl(r, r);
		eb_add(r, r, table[0]);
		eb_norm(r, r);
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		/* Free the precomputation table. */
		for (i = 0; i < (1 << (EB_WIDTH - 2)); i++) {
			eb_free(table[i]);
		}
		bn_free(n);
		bn_free(_k);
	}
}

#endif

void eb_mul_gen(eb_t r, bn_t k) {
#ifdef EB_PRECO
	eb_mul_fix(r, eb_curve_get_tab(), k);
#else
	eb_t gen;

	eb_null(gen);

	TRY {
		eb_new(gen);
		eb_curve_get_gen(gen);
		eb_mul(r, gen, k);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		eb_free(gen);
	}
#endif
}

void eb_mul_dig(eb_t r, eb_t p, dig_t k) {
	int i, l;
	eb_t t;

	eb_null(t);

	if (k == 0) {
		eb_set_infty(r);
		return;
	}

	TRY {
		eb_new(t);

		l = util_bits_dig(k);

		eb_copy(t, p);

		for (i = l - 2; i >= 0; i--) {
			eb_dbl(t, t);
			if (k & ((dig_t)1 << i)) {
				eb_add(t, t, p);
			}
		}

		eb_norm(r, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		eb_free(t);
	}
}
