/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2013 RELIC Authors
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
#include "relic_hb.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if HB_FIX == BASIC || !defined(STRIP)

void hb_mul_pre_basic(hb_t *t, hb_t p) {
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		hb_curve_get_ord(n);

		hb_copy(t[0], p);
		for (int i = 1; i < bn_bits(n); i++) {
			hb_dbl(t[i], t[i - 1]);
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

void hb_mul_fix_basic(hb_t r, hb_t *t, bn_t k) {
	int i, l;

	l = bn_bits(k);

	hb_set_infty(r);

	for (i = 0; i < l; i++) {
		if (bn_test_bit(k, i)) {
			hb_add(r, r, t[i]);
		}
	}
	hb_norm(r, r);
}

#endif

#if HB_FIX == YAOWI || !defined(STRIP)

void hb_mul_pre_yaowi(hb_t *t, hb_t p) {
	int l;
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		hb_curve_get_ord(n);
		l = bn_bits(n);
		l = ((l % HB_DEPTH) == 0 ? (l / HB_DEPTH) : (l / HB_DEPTH) + 1);

		hb_copy(t[0], p);
		for (int i = 1; i < l; i++) {
			hb_dbl(t[i], t[i - 1]);
			for (int j = 1; j < HB_DEPTH; j++) {
				hb_dbl(t[i], t[i]);
			}
		}
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		bn_free(n);
	}
}

void hb_mul_fix_yaowi(hb_t r, hb_t *t, bn_t k) {
	int i, j, l;
	hb_t a;
	unsigned char win[2 * FB_BITS + 1];

	hb_null(a);

	if (bn_is_zero(k)) {
		hb_set_infty(r);
		return;
	}

	TRY {
		hb_new(a);

		hb_set_infty(r);
		hb_set_infty(a);

		bn_rec_win(win, &l, k, HB_DEPTH);

		for (j = (1 << HB_DEPTH) - 1; j >= 1; j--) {
			for (i = 0; i < l; i++) {
				if (win[i] == j) {
					hb_add(a, a, t[i]);
				}
			}
			hb_add(r, r, a);
		}
		hb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		hb_free(a);
	}
}

#endif

#if HB_FIX == NAFWI || !defined(STRIP)

void hb_mul_pre_nafwi(hb_t *t, hb_t p) {
	bn_t n;

	bn_null(n);

	TRY {
		int l;
		bn_new(n);

		hb_curve_get_ord(n);
		l = bn_bits(n) + 1;
		l = ((l % HB_DEPTH) == 0 ? (l / HB_DEPTH) : (l / HB_DEPTH) + 1);

		hb_copy(t[0], p);
		for (int i = 1; i < l; i++) {
			hb_dbl(t[i], t[i - 1]);
			for (int j = 1; j < HB_DEPTH; j++) {
				hb_dbl(t[i], t[i]);
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

void hb_mul_fix_nafwi(hb_t r, hb_t *t, bn_t k) {
	int i, j, l, d, m;
	hb_t a;
	signed char naf[2 * FB_BITS + 1];
	char w;

	hb_null(a);

	TRY {
		hb_new(a);

		hb_set_infty(r);
		hb_set_infty(a);

		bn_rec_naf(naf, &l, k, 2);

		d = ((l % HB_DEPTH) == 0 ? (l / HB_DEPTH) : (l / HB_DEPTH) + 1);

		for (i = 0; i < d; i++) {
			w = 0;
			for (j = HB_DEPTH - 1; j >= 0; j--) {
				if (i * HB_DEPTH + j < l) {
					w = (char)(w << 1);
					w = (char)(w + naf[i * HB_DEPTH + j]);
				}
			}
			naf[i] = w;
		}

		if (HB_DEPTH % 2 == 0) {
			m = ((1 << (HB_DEPTH + 1)) - 2) / 3;
		} else {
			m = ((1 << (HB_DEPTH + 1)) - 1) / 3;
		}

		for (j = m; j >= 1; j--) {
			for (i = 0; i < d; i++) {
				if (naf[i] == j) {
					hb_add(a, a, t[i]);
				}
				if (naf[i] == -j) {
					hb_sub(a, a, t[i]);
				}
			}
			hb_add(r, r, a);
		}
		hb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		hb_free(a);
	}
}

#endif

#if HB_FIX == COMBS || !defined(STRIP)

void hb_mul_pre_combs(hb_t *t, hb_t p) {
	int i, j, l;
	bn_t ord;

	bn_null(ord);

	TRY {
		bn_new(ord);

		hb_curve_get_ord(ord);
		l = bn_bits(ord);
		l = ((l % HB_DEPTH) == 0 ? (l / HB_DEPTH) : (l / HB_DEPTH) + 1);

		hb_set_infty(t[0]);

		hb_copy(t[1], p);
		for (j = 1; j < HB_DEPTH; j++) {
			hb_dbl(t[1 << j], t[1 << (j - 1)]);
			for (i = 1; i < l; i++) {
				hb_dbl(t[1 << j], t[1 << j]);
			}
			for (i = 1; i < (1 << j); i++) {
				hb_add(t[(1 << j) + i], t[1 << j], t[i]);
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(ord);
	}
}

void hb_mul_fix_combs(hb_t r, hb_t *t, bn_t k) {
	int i, j, l, w, n, p0, p1;
	bn_t ord;

	bn_null(ord);

	TRY {
		bn_new(ord);

		hb_curve_get_ord(ord);
		l = bn_bits(ord);
		l = ((l % HB_DEPTH) == 0 ? (l / HB_DEPTH) : (l / HB_DEPTH) + 1);

		n = bn_bits(k);

		p0 = (HB_DEPTH) * l - 1;

		w = 0;
		p1 = p0--;
		for (j = HB_DEPTH - 1; j >= 0; j--, p1 -= l) {
			w = w << 1;
			if (p1 < n && bn_test_bit(k, p1)) {
				w = w | 1;
			}
		}
		hb_copy(r, t[w]);

		for (i = l - 2; i >= 0; i--) {
			hb_dbl(r, r);

			w = 0;
			p1 = p0--;
			for (j = HB_DEPTH - 1; j >= 0; j--, p1 -= l) {
				w = w << 1;
				if (p1 < n && bn_test_bit(k, p1)) {
					w = w | 1;
				}
			}
			if (w > 0) {
				hb_add(r, r, t[w]);
			}
		}
		hb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(ord);
	}
}

#endif

#if HB_FIX == COMBD || !defined(STRIP)

void hb_mul_pre_combd(hb_t *t, hb_t p) {
	bn_t n;

	bn_null(n);

	TRY {
		int i, j, d, e;
		bn_new(n);

		hb_curve_get_ord(n);
		d = bn_bits(n);
		d = ((d % HB_DEPTH) == 0 ? (d / HB_DEPTH) : (d / HB_DEPTH) + 1);
		e = (d % 2 == 0 ? (d / 2) : (d / 2) + 1);

		hb_set_infty(t[0]);
		hb_copy(t[1], p);
		for (j = 1; j < HB_DEPTH; j++) {
			hb_dbl(t[1 << j], t[1 << (j - 1)]);
			for (i = 1; i < d; i++) {
				hb_dbl(t[1 << j], t[1 << j]);
			}
			for (i = 1; i < (1 << j); i++) {
				hb_add(t[(1 << j) + i], t[1 << j], t[i]);
			}
		}
		hb_set_infty(t[1 << HB_DEPTH]);
		for (j = 1; j < (1 << HB_DEPTH); j++) {
			hb_dbl(t[(1 << HB_DEPTH) + j], t[j]);
			for (i = 1; i < e; i++) {
				hb_dbl(t[(1 << HB_DEPTH) + j], t[(1 << HB_DEPTH) + j]);
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

void hb_mul_fix_combd(hb_t r, hb_t *t, bn_t k) {
	int i, j, d, e, w0, w1, n0, p0, p1;
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		hb_curve_get_ord(n);

		d = bn_bits(n);
		d = ((d % HB_DEPTH) == 0 ? (d / HB_DEPTH) : (d / HB_DEPTH) + 1);
		e = (d % 2 == 0 ? (d / 2) : (d / 2) + 1);

		hb_set_infty(r);
		n0 = bn_bits(k);

		p1 = (e - 1) + (HB_DEPTH - 1) * d;
		for (i = e - 1; i >= 0; i--) {
			hb_dbl(r, r);

			w0 = 0;
			p0 = p1;
			for (j = HB_DEPTH - 1; j >= 0; j--, p0 -= d) {
				w0 = w0 << 1;
				if (p0 < n0 && bn_test_bit(k, p0)) {
					w0 = w0 | 1;
				}
			}

			w1 = 0;
			p0 = p1-- + e;
			for (j = HB_DEPTH - 1; j >= 0; j--, p0 -= d) {
				w1 = w1 << 1;
				if (i + e < d && p0 < n0 && bn_test_bit(k, p0)) {
					w1 = w1 | 1;
				}
			}

			hb_add(r, r, t[w0]);
			hb_add(r, r, t[(1 << HB_DEPTH) + w1]);
		}
		hb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

#endif

#if HB_FIX == LWNAF || !defined(STRIP)

void hb_mul_pre_lwnaf(hb_t *t, hb_t p) {
	int i;

	for (i = 0; i < (1 << (HB_DEPTH - 2)); i++) {
		hb_set_infty(t[i]);
	}

	hb_dbl(t[0], p);

#if HB_DEPTH > 2
	hb_add(t[1], t[0], p);
	for (i = 2; i < (1 << (HB_DEPTH - 2)); i++) {
		hb_add(t[i], t[i - 1], t[0]);
	}
#endif
	hb_copy(t[0], p);
}

void hb_mul_fix_lwnaf(hb_t r, hb_t *t, bn_t k) {
	int len, i, n;
	signed char naf[2 * FB_BITS + 1], *_t;

	/* Compute the w-NAF representation of k. */
	bn_rec_naf(naf, &len, k, HB_DEPTH);

	_t = naf + len - 1;
	hb_set_infty(r);
	for (i = len - 1; i >= 0; i--, _t--) {
		hb_dbl(r, r);

		n = *_t;
		if (n > 0) {
			hb_add(r, r, t[n / 2]);
		}
		if (n < 0) {
			hb_sub(r, r, t[-n / 2]);
		}
	}
	/* Convert r to affine coordinates. */
	hb_norm(r, r);
}

#endif

#if HB_FIX == OCTUP || !defined(STRIP)

void hb_mul_pre_octup(hb_t *t, hb_t p) {
	hb_set_infty(t[0]);
	hb_copy(t[1], p);
	hb_dbl(t[2], t[1]);
	hb_dbl(t[4], t[2]);
	hb_sub(t[3], t[4], p);
	hb_add(t[5], t[4], p);
	hb_dbl(t[6], t[3]);
	hb_add(t[7], t[6], p);
}

void hb_mul_fix_octup(hb_t r, hb_t *t, bn_t k) {
	unsigned char win[2 * FB_BITS/3];
	int i, l;
	hb_t p;

	hb_null(p);

	if (bn_is_zero(k)) {
		hb_set_infty(r);
		return;
	}

	TRY {
		hb_new(p);

		bn_rec_win(win, &l, k, 3);

		hb_copy(p, t[win[l - 1]]);

		for (i = l - 2; i >= 0; i--) {
			hb_oct(p, p);
			hb_add(p, p, t[win[i]]);
		}

		hb_copy(r, p);
		//hb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		hb_free(p);
	}
}

#endif

