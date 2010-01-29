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
 * Implementation of the point multiplication on prime elliptic curves.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_ep.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if EB_MUL == WTNAF || !defined(STRIP)

/**
 * Precomputes a table for a point multiplication on an ordinary curve.
 *
 * @param[out] t				- the destination table.
 * @param[in] p					- the point to multiply.
 */
static void table_init(ep_t * t, ep_t p) {
	int i;

	for (i = 0; i < (1 << (EP_WIDTH - 2)); i++) {
		ep_set_infty(t[i]);
		fp_set_dig(t[i]->z, 1);
		t[i]->norm = 1;
	}

	ep_dbl_tab(t[0], p);

#if EP_WIDTH > 2
	ep_add_tab(t[1], t[0], p);
	for (i = 2; i < (1 << (EP_WIDTH - 2)); i++) {
		ep_add_tab(t[i], t[i - 1], t[0]);
	}
#endif

	ep_copy(t[0], p);
}

static void ep_mul_naf_tab(ep_t r, ep_t p, bn_t k) {
	int len, i, n;
	signed char naf[FP_BITS + 1], *t;
	ep_t table[1 << (EP_WIDTH - 2)];

	for (i = 0; i < (1 << (EP_WIDTH - 2)); i++) {
		ep_null(table[i]);
	}

	TRY {
		/* Prepare the precomputation table. */
		for (i = 0; i < (1 << (EP_WIDTH - 2)); i++) {
			ep_new(table[i]);
		}
		/* Compute the precomputation table. */
		table_init(table, p);

		/* Compute the w-TNAF representation of k. */
		bn_rec_naf(naf, &len, k, EP_WIDTH);

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
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		/* Free the precomputation table. */
		for (i = 0; i < (1 << (EP_WIDTH - 2)); i++) {
			ep_free(table[i]);
		}
	}
}

#endif /* EP_MUL == WTNAF */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EP_MUL == BASIC || !defined(STRIP)

void ep_mul_basic(ep_t r, ep_t p, bn_t k) {
	int i, l;
	ep_t t;

	ep_null(t);

	TRY {
		ep_new(t);
		l = bn_bits(k);

		if (bn_test_bit(k, l - 1)) {
			ep_copy(t, p);
		} else {
			ep_set_infty(t);
		}

		for (i = l - 2; i >= 0; i--) {
			ep_dbl(t, t);
			if (bn_test_bit(k, i)) {
				ep_add(t, t, p);
			}
		}

		ep_copy(r, t);
		ep_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep_free(t);
	}
}

#endif

#if EP_MUL == WTNAF || !defined(STRIP)

void ep_mul_wtnaf(ep_t r, ep_t p, bn_t k) {
	ep_mul_naf_tab(r, p, k);
}

#endif

void ep_mul_gen(ep_t r, bn_t k) {
#ifdef EP_PRECO
	ep_mul_fix(r, ep_curve_get_tab(), k);
#else
	ep_mul(r, ep_curve_get_gen(), k);
#endif
}
