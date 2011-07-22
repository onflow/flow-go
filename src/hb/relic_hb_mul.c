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
 * Implementation of divisor class multiplication on binary hyperelliptic
 * curves.
 *
 * @version $Id$
 * @ingroup hb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_hb.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if HB_MUL == LWNAF || !defined(STRIP)

/* Support for ordinary curves. */
#if defined(HB_SUPER)

/**
 * Precomputes a table for a point multiplication on an ordinary curve.
 *
 * @param[out] t				- the destination table.
 * @param[in] p					- the point to multiply.
 */
static void table_init(hb_t *t, hb_t p) {
	int i;

	for (i = 0; i < (1 << (HB_WIDTH - 2)); i++) {
		hb_set_infty(t[i]);
	}

	hb_dbl_tab(t[0], p);

#if HB_WIDTH > 2
	hb_add_tab(t[1], t[0], p);
	for (i = 2; i < (1 << (HB_WIDTH - 2)); i++) {
		hb_add_tab(t[i], t[i - 1], t[0]);
	}
#endif
	hb_copy(t[0], p);
}

#endif /* HB_SUPER */
#endif /* HB_MUL == LWNAF */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void hb_mul_basic(hb_t r, hb_t p, bn_t k) {
	int i, l;
	hb_t t;

	hb_null(t);

	if (bn_is_zero(k)) {
		hb_set_infty(r);
		return;
	}

	TRY {
		hb_new(t);

		l = bn_bits(k);

		hb_copy(t, p);

		for (i = l - 2; i >= 0; i--) {
			hb_dbl(t, t);
			if (bn_test_bit(k, i)) {
				hb_add(t, t, p);
			}
		}

		hb_copy(r, t);
		//hb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		hb_free(t);
	}
}

void hb_mul_octup(hb_t r, hb_t p, bn_t k) {
	hb_t table[8];
	unsigned char win[2 * FB_BITS/3];
	int i, l;
	hb_t t;

	hb_null(t);

	if (bn_is_zero(k)) {
		hb_set_infty(r);
		return;
	}

	TRY {
		hb_new(t);

		bn_rec_win(win, &l, k, 3);

		hb_set_infty(table[0]);
		hb_copy(table[1], p);
		hb_dbl(table[2], table[1]);
		hb_dbl(table[4], table[2]);
		hb_sub(table[3], table[4], p);
		hb_add(table[5], table[4], p);
		hb_dbl(table[6], table[3]);
		hb_add(table[7], table[6], p);

		hb_copy(t, table[win[l - 1]]);

		for (i = l - 2; i >= 0; i--) {
			hb_oct(t, t);
			hb_add(t, t, table[win[i]]);
		}

		hb_copy(r, t);
		//hb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		hb_free(t);
	}
}

#if HB_MUL == LWNAF || !defined(STRIP)

void hb_mul_lwnaf(hb_t r, hb_t p, bn_t k) {
	int len, i, n;
	signed char naf[2 * FB_BITS + 1], *t;
	hb_t table[1 << (HB_WIDTH - 2)];

	for (i = 0; i < (1 << (HB_WIDTH - 2)); i++) {
		hb_null(table[i]);
	}

	TRY {
		/* Prepare the precomputation table. */
		for (i = 0; i < (1 << (HB_WIDTH - 2)); i++) {
			hb_new(table[i]);
		}
		/* Compute the precomputation table. */
		table_init(table, p);

		/* Compute the w-TNAF representation of k. */
		bn_rec_naf(naf, &len, k, HB_WIDTH);

		t = naf + len - 1;

		hb_set_infty(r);
		for (i = len - 1; i >= 0; i--, t--) {
			hb_dbl(r, r);

			n = *t;
			if (n > 0) {
				hb_add(r, r, table[n / 2]);
			}
			if (n < 0) {
				hb_sub(r, r, table[-n / 2]);
			}
		}
		/* Convert r to affine coordinates. */
		hb_norm(r, r);

	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		/* Free the precomputation table. */
		for (i = 0; i < (1 << (HB_WIDTH - 2)); i++) {
			hb_free(table[i]);
		}
	}
}

#endif

void hb_mul_gen(hb_t r, bn_t k) {
#ifdef HB_PRECO
	hb_mul_fix(r, hb_curve_get_tab(), k);
#else
	hb_t gen;

	hb_null(gen);

	TRY {
		hb_new(gen);
		hb_curve_get_gen(gen);
		hb_mul(r, gen, k);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		hb_free(gen);
	}
#endif
}

void hb_mul_dig(hb_t r, hb_t p, dig_t k) {
	int i, l;
	hb_t t;

	hb_null(t);

	TRY {
		hb_new(t);

		l = util_bits_dig(k);

		hb_copy(t, p);

		for (i = l - 2; i >= 0; i--) {
			hb_dbl(t, t);
			if (k & ((dig_t)1 << i)) {
				hb_add(t, t, p);
			}
		}

		hb_copy(r, t);
		//hb_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		hb_free(t);
	}
}
