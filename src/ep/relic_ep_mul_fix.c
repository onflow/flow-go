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
 * @ingroup ep
 */

#include "string.h"

#include "relic_core.h"
#include "relic_ep.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if EP_FIX == WTNAF || !defined(STRIP)

/**
 * Precomputes a table for a point multiplication on an ordinary curve.
 *
 * @param[out] t				- the destination table.
 * @param[in] p					- the point to multiply.
 */
static void ep_mul_pre_ordin(ep_t * t, ep_t p) {
	int i;

	for (i = 0; i < (1 << (EP_DEPTH - 2)); i++) {
		ep_set_infty(t[i]);
		fp_set_dig(t[i]->z, 1);
		t[i]->norm = 1;
	}

	ep_dbl_tab(t[0], p);

#if EP_DEPTH > 2
	ep_add_tab(t[1], t[0], p);
	for (i = 2; i < (1 << (EP_DEPTH - 2)); i++) {
		ep_add_tab(t[i], t[i - 1], t[0]);
	}
#endif
	ep_copy(t[0], p);
}

/**
 * Multiplies a binary elliptic curve point by an integer using the w-NAF
 * method.
 *
 * @param[out] r 				- the result.
 * @param[in] p					- the point to multiply.
 * @param[in] k					- the integer.
 */
static void ep_mul_fix_ordin(ep_t r, ep_t * table, bn_t k) {
	int len, i, n;
	signed char naf[FP_BITS + 1], *t;

	/* Compute the w-TNAF representation of k. */
	bn_rec_naf(naf, &len, k, EP_DEPTH);

	t = naf + len - 1;
	ep_set_infty(r);
	for (i = len - 1; i >= 0; i--, t--) {
		ep_dbl(r, r);

		n = *t;
		if (n > 0) {
			ep_add(r, r, table[n / 2]);
		}
		if (n < 0) {
			ep_sub(r, r, table[-n / 2]);
		}
	}
	/* Convert r to affine coordinates. */
	ep_norm(r, r);
}

#endif /* EP_FIX == WTNAF */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EP_FIX == BASIC || !defined(STRIP)

void ep_mul_pre_basic(ep_t * t, ep_t p) {
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		ep_curve_get_ord(n);

		ep_copy(t[0], p);
		for (int i = 1; i < bn_bits(n); i++) {
			ep_dbl_tab(t[i], t[i - 1]);
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

void ep_mul_fix_basic(ep_t r, ep_t * t, bn_t k) {
	int i, l;

	l = bn_bits(k);

	ep_set_infty(r);

	for (i = 0; i < l; i++) {
		if (bn_test_bit(k, i)) {
			ep_add(r, r, t[i]);
		}
	}
	ep_norm(r, r);
}

#endif

#if EP_FIX == YAOWI || !defined(STRIP)

void ep_mul_pre_yaowi(ep_t *t, ep_t p) {
	int l;
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		ep_curve_get_ord(n);
		l = bn_bits(n);
		l = ((l % EB_DEPTH) == 0 ? (l / EB_DEPTH) : (l / EB_DEPTH) + 1);

		ep_copy(t[0], p);
		for (int i = 1; i < l; i++) {
			ep_dbl_tab(t[i], t[i - 1]);
			for (int j = 1; j < EB_DEPTH; j++) {
				ep_dbl_tab(t[i], t[i]);
			}
		}
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		bn_free(n);
	}
}

void ep_mul_fix_yaowi(ep_t r, ep_t *t, bn_t k) {
	int i, j, l;
	ep_t a;
	unsigned char win[FP_BITS + 1];

	ep_null(a);

	TRY {
		ep_new(a);

		ep_set_infty(r);
		ep_set_infty(a);

		bn_rec_win(win, &l, k, EB_DEPTH);

		for (j = (1 << EB_DEPTH) - 1; j >= 1; j--) {
			for (i = 0; i < l; i++) {
				if (win[i] == j) {
					ep_add(a, a, t[i]);
				}
			}
			ep_add(r, r, a);
		}
		ep_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep_free(a);
	}
}

#endif

#if EP_FIX == NAFWI || !defined(STRIP)

void ep_mul_pre_nafwi(ep_t * t, ep_t p) {
	int l;
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		ep_curve_get_ord(n);
		l = bn_bits(n) + 1;
		l = ((l % EP_DEPTH) == 0 ? (l / EP_DEPTH) : (l / EP_DEPTH) + 1);

		ep_copy(t[0], p);
		for (int i = 1; i < l; i++) {
			ep_dbl_tab(t[i], t[i - 1]);
			for (int j = 1; j < EP_DEPTH; j++) {
				ep_dbl_tab(t[i], t[i]);
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

void ep_mul_fix_nafwi(ep_t r, ep_t * t, bn_t k) {
	int i, j, l, d, m;
	ep_t a;
	signed char naf[FP_BITS + 1];
	char w;

	ep_null(a);

	TRY {
		ep_new(a);

		ep_set_infty(r);
		ep_set_infty(a);

		bn_rec_naf(naf, &l, k, 2);

		d = ((l % EP_DEPTH) == 0 ? (l / EP_DEPTH) : (l / EP_DEPTH) + 1);

		for (i = 0; i < d; i++) {
			w = 0;
			for (j = EP_DEPTH - 1; j >= 0; j--) {
				if (i * EP_DEPTH + j < l) {
					w = (char)(w << 1);
					w = (char)(w + naf[i * EP_DEPTH + j]);
				}
			}
			naf[i] = w;
		}

		if (EP_DEPTH % 2 == 0) {
			m = ((1 << (EP_DEPTH + 1)) - 2) / 3;
		} else {
			m = ((1 << (EP_DEPTH + 1)) - 1) / 3;
		}

		for (j = m; j >= 1; j--) {
			for (i = 0; i < d; i++) {
				if (naf[i] == j) {
					ep_add(a, a, t[i]);
				}
				if (naf[i] == -j) {
					ep_sub(a, a, t[i]);
				}
			}
			ep_add(r, r, a);
		}
		ep_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep_free(a);
	}
}

#endif

#if EP_FIX == COMBS || !defined(STRIP)

void ep_mul_pre_combs(ep_t * t, ep_t p) {
	int i, j, l;
	bn_t ord;

	bn_null(ord);

	TRY {
		bn_new(ord);

		ep_curve_get_ord(ord);
		l = bn_bits(ord);
		l = ((l % EP_DEPTH) == 0 ? (l / EP_DEPTH) : (l / EP_DEPTH) + 1);

		ep_set_infty(t[0]);

		ep_copy(t[1], p);
		for (j = 1; j < EP_DEPTH; j++) {
			ep_dbl_tab(t[1 << j], t[1 << (j - 1)]);
			for (i = 1; i < l; i++) {
				ep_dbl_tab(t[1 << j], t[1 << j]);
			}
			for (i = 1; i < (1 << j); i++) {
				ep_add_tab(t[(1 << j) + i], t[1 << j], t[i]);
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

void ep_mul_fix_combs(ep_t r, ep_t * t, bn_t k) {
	int i, j, l, w, n0, p0, p1;
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		ep_curve_get_ord(n);
		l = bn_bits(n);
		l = ((l % EP_DEPTH) == 0 ? (l / EP_DEPTH) : (l / EP_DEPTH) + 1);

		n0 = bn_bits(k);

		p0 = (EP_DEPTH) * l - 1;

		w = 0;
		p1 = p0--;
		for (j = EP_DEPTH - 1; j >= 0; j--, p1 -= l) {
			w = w << 1;
			if (p1 < n0 && bn_test_bit(k, p1)) {
				w = w | 1;
			}
		}
		ep_copy(r, t[w]);

		for (i = l - 2; i >= 0; i--) {
			ep_dbl(r, r);

			w = 0;
			p1 = p0--;
			for (j = EP_DEPTH - 1; j >= 0; j--, p1 -= l) {
				w = w << 1;
				if (p1 < n0 && bn_test_bit(k, p1)) {
					w = w | 1;
				}
			}
			if (w > 0) {
				ep_add(r, r, t[w]);
			}
		}
		ep_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

#endif

#if EP_FIX == COMBD || !defined(STRIP)

void ep_mul_pre_combd(ep_t * t, ep_t p) {
	int i, j, d, e;
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		ep_curve_get_ord(n);
		d = bn_bits(n);
		d = ((d % EP_DEPTH) == 0 ? (d / EP_DEPTH) : (d / EP_DEPTH) + 1);
		e = (d % 2 == 0 ? (d / 2) : (d / 2) + 1);

		ep_set_infty(t[0]);
		ep_copy(t[1], p);
		for (j = 1; j < EP_DEPTH; j++) {
			ep_dbl_tab(t[1 << j], t[1 << (j - 1)]);
			for (i = 1; i < d; i++) {
				ep_dbl_tab(t[1 << j], t[1 << j]);
			}
			for (i = 1; i < (1 << j); i++) {
				ep_add_tab(t[(1 << j) + i], t[1 << j], t[i]);
			}
		}
		ep_set_infty(t[1 << EP_DEPTH]);
		for (j = 1; j < (1 << EP_DEPTH); j++) {
			ep_dbl_tab(t[(1 << EP_DEPTH) + j], t[j]);
			for (i = 1; i < e; i++) {
				ep_dbl_tab(t[(1 << EP_DEPTH) + j], t[(1 << EP_DEPTH) + j]);
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

void ep_mul_fix_combd(ep_t r, ep_t * t, bn_t k) {
	int i, j, d, e, w0, w1, n0, p0, p1;
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		ep_curve_get_ord(n);
		d = bn_bits(n);
		d = ((d % EP_DEPTH) == 0 ? (d / EP_DEPTH) : (d / EP_DEPTH) + 1);
		e = (d % 2 == 0 ? (d / 2) : (d / 2) + 1);

		ep_set_infty(r);
		n0 = bn_bits(k);

		p1 = (e - 1) + (EP_DEPTH - 1) * d;
		for (i = e - 1; i >= 0; i--) {
			ep_dbl(r, r);

			w0 = 0;
			p0 = p1;
			for (j = EP_DEPTH - 1; j >= 0; j--, p0 -= d) {
				w0 = w0 << 1;
				if (p0 < n0 && bn_test_bit(k, p0)) {
					w0 = w0 | 1;
				}
			}

			w1 = 0;
			p0 = p1-- + e;
			for (j = EP_DEPTH - 1; j >= 0; j--, p0 -= d) {
				w1 = w1 << 1;
				if (i + e < d && p0 < n0 && bn_test_bit(k, p0)) {
					w1 = w1 | 1;
				}
			}

			ep_add(r, r, t[w0]);
			ep_add(r, r, t[(1 << EP_DEPTH) + w1]);
		}
		ep_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

#endif

#if EP_FIX == WTNAF || !defined(STRIP)

void ep_mul_pre_wtnaf(ep_t * t, ep_t p) {
	ep_mul_pre_ordin(t, p);
}

void ep_mul_fix_wtnaf(ep_t r, ep_t * t, bn_t k) {
	ep_mul_fix_ordin(r, t, k);
}
#endif
