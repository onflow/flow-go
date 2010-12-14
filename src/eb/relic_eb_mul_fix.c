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
 * Implementation of fixed point multiplication on binary elliptic curves.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_eb.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if EB_FIX == LWNAF || !defined(STRIP)

#if defined(EB_KBLTZ)

/**
 * Precomputes a table for a point multiplication on a Koblitz curve.
 *
 * @param[out] t				- the destination table.
 * @param[in] p					- the point to multiply.
 */
static void eb_mul_pre_kbltz(eb_t *t, eb_t p) {
	int u;

	if (eb_curve_opt_a() == OPT_ZERO) {
		u = -1;
	} else {
		u = 1;
	}

	/* Prepare the precomputation table. */
	for (int i = 0; i < 1 << (EB_DEPTH - 2); i++) {
		eb_set_infty(t[i]);
		fb_set_bit(t[i]->z, 0, 1);
		t[i]->norm = 1;
	}

	eb_copy(t[0], p);

	/* The minimum table depth for LWNAF is 3. */
#if EB_DEPTH == 3
	eb_frb(t[1], t[0]);
	if (u == 1) {
		eb_sub(t[1], t[0], t[1]);
	} else {
		eb_add(t[1], t[0], t[1]);
	}
#endif

#if EB_DEPTH == 4
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

#if EB_DEPTH == 5
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

#if EB_DEPTH == 6
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

#endif

	eb_norm_sim(t + 1, t + 1, EB_TABLE_LWNAF - 1);

	eb_copy(t[0], p);
}

/**
 * Multiplies a binary elliptic curve point by an integer using the w-TNAF
 * method.
 *
 * @param[out] r 				- the result.
 * @param[in] p					- the point to multiply.
 * @param[in] k					- the integer.
 * @param[in] w					- the window size.
 */
static void eb_mul_fix_kbltz(eb_t r, eb_t *table, bn_t k) {
	int len, i, n;
	signed char u, tnaf[FB_BITS + 8], *t;
	bn_t vm, s0, s1;

	bn_null(vm);
	bn_null(s0);
	bn_null(s1);

	TRY {
		bn_new(vm);
		bn_new(s0);
		bn_new(s1);

		/* Compute the w-TNAF representation of k. */
		if (eb_curve_opt_a() == OPT_ZERO) {
			u = -1;
		} else {
			u = 1;
		}

		eb_curve_get_vm(vm);
		eb_curve_get_s0(s0);
		eb_curve_get_s1(s1);
		/* Compute the w-TNAF representation of k. */
		bn_rec_tnaf(tnaf, &len, k, vm, s0, s1, u, FB_BITS, EB_DEPTH);

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
	}
}

#endif /* EB_KBLTZ */

#if defined(EB_ORDIN) || defined(EB_SUPER)

/**
 * Precomputes a table for a point multiplication on an ordinary curve.
 *
 * @param[out] t				- the destination table.
 * @param[in] p					- the point to multiply.
 */
static void eb_mul_pre_ordin(eb_t *t, eb_t p) {
	int i;

	for (i = 0; i < (1 << (EB_DEPTH - 2)); i++) {
		eb_set_infty(t[i]);
		fb_set_bit(t[i]->z, 0, 1);
		t[i]->norm = 1;
	}

	eb_dbl(t[0], p);

#if EB_DEPTH > 2
	eb_add(t[1], t[0], p);
	for (i = 2; i < (1 << (EB_DEPTH - 2)); i++) {
		eb_add(t[i], t[i - 1], t[0]);
	}
#endif

	eb_norm_sim(t + 1, t + 1, EB_TABLE_LWNAF - 1);

	eb_copy(t[0], p);
}

/**
 * Multiplies a binary elliptic curve point by an integer using the w-NAF
 * method.
 *
 * @param[out] r 				- the result.
 * @param[in] p					- the point to multiply.
 * @param[in] k					- the integer.
 */
static void eb_mul_fix_ordin(eb_t r, eb_t *table, bn_t k) {
	int len, i, n;
	signed char naf[FB_BITS + 1], *t;

	/* Compute the w-TNAF representation of k. */
	bn_rec_naf(naf, &len, k, EB_DEPTH);

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

#endif /* EB_ORDIN || EB_SUPER */
#endif /* EB_FIX == LWNAF */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EB_FIX == BASIC || !defined(STRIP)

void eb_mul_pre_basic(eb_t *t, eb_t p) {
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		eb_curve_get_ord(n);

		eb_copy(t[0], p);
		for (int i = 1; i < bn_bits(n); i++) {
			eb_dbl(t[i], t[i - 1]);
		}

		eb_norm_sim(t + 1, t + 1, EB_TABLE_BASIC - 1);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

void eb_mul_fix_basic(eb_t r, eb_t *t, bn_t k) {
	int i, l;

	l = bn_bits(k);

	eb_set_infty(r);

	for (i = 0; i < l; i++) {
		if (bn_test_bit(k, i)) {
			eb_add(r, r, t[i]);
		}
	}
	eb_norm(r, r);
}

#endif

#if EB_FIX == YAOWI || !defined(STRIP)

void eb_mul_pre_yaowi(eb_t *t, eb_t p) {
	int l;
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		eb_curve_get_ord(n);
		l = bn_bits(n);
		l = ((l % EB_DEPTH) == 0 ? (l / EB_DEPTH) : (l / EB_DEPTH) + 1);

		eb_copy(t[0], p);
		for (int i = 1; i < l; i++) {
			eb_dbl(t[i], t[i - 1]);
			for (int j = 1; j < EB_DEPTH; j++) {
				eb_dbl(t[i], t[i]);
			}
		}

		eb_norm_sim(t + 1, t + 1, EB_TABLE_YAOWI - 1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		bn_free(n);
	}
}

void eb_mul_fix_yaowi(eb_t r, eb_t *t, bn_t k) {
	int i, j, l;
	eb_t a;
	unsigned char win[FB_BITS + 1];

	eb_null(a);

	if (bn_is_zero(k)) {
		eb_set_infty(r);
		return;
	}

	TRY {
		eb_new(a);

		eb_set_infty(r);
		eb_set_infty(a);

		bn_rec_win(win, &l, k, EB_DEPTH);

		for (j = (1 << EB_DEPTH) - 1; j >= 1; j--) {
			for (i = 0; i < l; i++) {
				if (win[i] == j) {
					eb_add(a, a, t[i]);
				}
			}
			eb_add(r, r, a);
		}
		eb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		eb_free(a);
	}
}

#endif

#if EB_FIX == NAFWI || !defined(STRIP)

void eb_mul_pre_nafwi(eb_t *t, eb_t p) {
	bn_t n;

	bn_null(n);

	TRY {
		int l;
		bn_new(n);

		eb_curve_get_ord(n);
		l = bn_bits(n) + 1;
		l = ((l % EB_DEPTH) == 0 ? (l / EB_DEPTH) : (l / EB_DEPTH) + 1);

		eb_copy(t[0], p);
		for (int i = 1; i < l; i++) {
			eb_dbl(t[i], t[i - 1]);
			for (int j = 1; j < EB_DEPTH; j++) {
				eb_dbl(t[i], t[i]);
			}
		}

		eb_norm_sim(t + 1, t + 1, EB_TABLE_NAFWI - 1);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

void eb_mul_fix_nafwi(eb_t r, eb_t *t, bn_t k) {
	int i, j, l, d, m;
	eb_t a;
	signed char naf[FB_BITS + 1];
	char w;

	eb_null(a);

	TRY {
		eb_new(a);

		eb_set_infty(r);
		eb_set_infty(a);

		bn_rec_naf(naf, &l, k, 2);

		d = ((l % EB_DEPTH) == 0 ? (l / EB_DEPTH) : (l / EB_DEPTH) + 1);

		for (i = 0; i < d; i++) {
			w = 0;
			for (j = EB_DEPTH - 1; j >= 0; j--) {
				if (i * EB_DEPTH + j < l) {
					w = (char)(w << 1);
					w = (char)(w + naf[i * EB_DEPTH + j]);
				}
			}
			naf[i] = w;
		}

		if (EB_DEPTH % 2 == 0) {
			m = ((1 << (EB_DEPTH + 1)) - 2) / 3;
		} else {
			m = ((1 << (EB_DEPTH + 1)) - 1) / 3;
		}

		for (j = m; j >= 1; j--) {
			for (i = 0; i < d; i++) {
				if (naf[i] == j) {
					eb_add(a, a, t[i]);
				}
				if (naf[i] == -j) {
					eb_sub(a, a, t[i]);
				}
			}
			eb_add(r, r, a);
		}
		eb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		eb_free(a);
	}
}

#endif

#if EB_FIX == COMBS || !defined(STRIP)

void eb_mul_pre_combs(eb_t *t, eb_t p) {
	int i, j, l;
	bn_t ord;

	bn_null(ord);

	TRY {
		bn_new(ord);

		eb_curve_get_ord(ord);
		l = bn_bits(ord);
		l = ((l % EB_DEPTH) == 0 ? (l / EB_DEPTH) : (l / EB_DEPTH) + 1);

		eb_set_infty(t[0]);

		eb_copy(t[1], p);
		for (j = 1; j < EB_DEPTH; j++) {
			eb_dbl(t[1 << j], t[1 << (j - 1)]);
			for (i = 1; i < l; i++) {
				eb_dbl(t[1 << j], t[1 << j]);
			}
			for (i = 1; i < (1 << j); i++) {
				eb_add(t[(1 << j) + i], t[1 << j], t[i]);
			}
		}

		eb_norm_sim(t + 2, t + 2, EB_TABLE_COMBS - 2);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(ord);
	}
}

void eb_mul_fix_combs(eb_t r, eb_t *t, bn_t k) {
	int i, j, l, w, n, p0, p1;
	bn_t ord;

	bn_null(ord);

	TRY {
		bn_new(ord);

		eb_curve_get_ord(ord);
		l = bn_bits(ord);
		l = ((l % EB_DEPTH) == 0 ? (l / EB_DEPTH) : (l / EB_DEPTH) + 1);

		n = bn_bits(k);

		p0 = (EB_DEPTH) * l - 1;

		w = 0;
		p1 = p0--;
		for (j = EB_DEPTH - 1; j >= 0; j--, p1 -= l) {
			w = w << 1;
			if (p1 < n && bn_test_bit(k, p1)) {
				w = w | 1;
			}
		}
		eb_copy(r, t[w]);

		for (i = l - 2; i >= 0; i--) {
			eb_dbl(r, r);

			w = 0;
			p1 = p0--;
			for (j = EB_DEPTH - 1; j >= 0; j--, p1 -= l) {
				w = w << 1;
				if (p1 < n && bn_test_bit(k, p1)) {
					w = w | 1;
				}
			}
			if (w > 0) {
				eb_add(r, r, t[w]);
			}
		}
		eb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(ord);
	}
}

#endif

#if EB_FIX == COMBD || !defined(STRIP)

void eb_mul_pre_combd(eb_t *t, eb_t p) {
	bn_t n;

	bn_null(n);

	TRY {
		int i, j, d, e;
		bn_new(n);

		eb_curve_get_ord(n);
		d = bn_bits(n);
		d = ((d % EB_DEPTH) == 0 ? (d / EB_DEPTH) : (d / EB_DEPTH) + 1);
		e = (d % 2 == 0 ? (d / 2) : (d / 2) + 1);

		eb_set_infty(t[0]);
		eb_copy(t[1], p);
		for (j = 1; j < EB_DEPTH; j++) {
			eb_dbl(t[1 << j], t[1 << (j - 1)]);
			for (i = 1; i < d; i++) {
				eb_dbl(t[1 << j], t[1 << j]);
			}
			for (i = 1; i < (1 << j); i++) {
				eb_add(t[(1 << j) + i], t[1 << j], t[i]);
			}
		}
		eb_set_infty(t[1 << EB_DEPTH]);
		for (j = 1; j < (1 << EB_DEPTH); j++) {
			eb_dbl(t[(1 << EB_DEPTH) + j], t[j]);
			for (i = 1; i < e; i++) {
				eb_dbl(t[(1 << EB_DEPTH) + j], t[(1 << EB_DEPTH) + j]);
			}
		}

		eb_norm_sim(t + 2, t + 2, (1 << EB_DEPTH) - 2);
		eb_norm_sim(t + (1 << EB_DEPTH) + 1, t + (1 << EB_DEPTH) + 1, (1 << EB_DEPTH) - 1);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

void eb_mul_fix_combd(eb_t r, eb_t *t, bn_t k) {
	int i, j, d, e, w0, w1, n0, p0, p1;
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		eb_curve_get_ord(n);

		d = bn_bits(n);
		d = ((d % EB_DEPTH) == 0 ? (d / EB_DEPTH) : (d / EB_DEPTH) + 1);
		e = (d % 2 == 0 ? (d / 2) : (d / 2) + 1);

		eb_set_infty(r);
		n0 = bn_bits(k);

		p1 = (e - 1) + (EB_DEPTH - 1) * d;
		for (i = e - 1; i >= 0; i--) {
			eb_dbl(r, r);

			w0 = 0;
			p0 = p1;
			for (j = EB_DEPTH - 1; j >= 0; j--, p0 -= d) {
				w0 = w0 << 1;
				if (p0 < n0 && bn_test_bit(k, p0)) {
					w0 = w0 | 1;
				}
			}

			w1 = 0;
			p0 = p1-- + e;
			for (j = EB_DEPTH - 1; j >= 0; j--, p0 -= d) {
				w1 = w1 << 1;
				if (i + e < d && p0 < n0 && bn_test_bit(k, p0)) {
					w1 = w1 | 1;
				}
			}

			eb_add(r, r, t[w0]);
			eb_add(r, r, t[(1 << EB_DEPTH) + w1]);
		}
		eb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

#endif

#if EB_FIX == LWNAF || !defined(STRIP)

void eb_mul_pre_lwnaf(eb_t *t, eb_t p) {
#if defined(EB_KBLTZ)
	if (eb_curve_is_kbltz()) {
		eb_mul_pre_kbltz(t, p);
		return;
	}
#endif

#if defined(EB_ORDIN) || defined(EB_SUPER)
	eb_mul_pre_ordin(t, p);
#endif
}

void eb_mul_fix_lwnaf(eb_t r, eb_t *t, bn_t k) {
#if defined(EB_KBLTZ)
	if (eb_curve_is_kbltz()) {
		eb_mul_fix_kbltz(r, t, k);
		return;
	}
#endif

#if defined(EB_ORDIN) || defined(EB_SUPER)
	eb_mul_fix_ordin(r, t, k);
#endif
}
#endif
