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
 * Implementation of the multiple precision arithmetic squaring
 * functions.
 *
 * @version $Id: relic_bn_sqr.c 10 2009-04-16 02:20:12Z dfaranha $
 * @ingroup bn
 */

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_bn.h"
#include "relic_bn_low.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if BN_KARAT > 0 || !defined(STRIP)

/**
 * Computes the square of a multiple precision integer using recursive Karatsuba
 * squaring.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the multiple precision integer to square.
 * @param[in] level			- the number of Karatsuba steps to apply.
 */
static void bn_sqr_karat_impl(bn_t c, bn_t a, int level) {
	int h;
	bn_t a0 = NULL, a1 = NULL, a0a0 = NULL, a1a1 = NULL, t = NULL;
	dig_t *tmpa, *tmpb, *t0;

	/* Compute half the digits of a or b. */
	h = a->used >> 1;

	TRY {
		/* Allocate the temp variables. */
		bn_new(a0);
		bn_new(a1);
		bn_new(a0a0);
		bn_new(a1a1);
		bn_new(t);

		a0->used = h;
		a1->used = a->used - h;

		tmpa = a->dp;
		tmpb = a->dp;

		/* a = a1 || a0 */
		t0 = a0->dp;
		for (int i = 0; i < h; i++, t0++, tmpa++)
			*t0 = *tmpa;
		t0 = a1->dp;
		for (int i = 0; i < a1->used; i++, t0++, tmpa++)
			*t0 = *tmpa;

		bn_trim(a0);

		if (level <= 1) {
			/* a0a0 = a0 * a0 and a1a1 = a1 * a1 */
#if BN_SQR == BASIC
			bn_sqr_basic(a0a0, a0);
			bn_sqr_basic(a1a1, a1);
#elif BN_SQR == COMBA
			bn_sqr_comba(a0a0, a0);
			bn_sqr_comba(a1a1, a1);
#endif
		} else {
			bn_sqr_karat_impl(a0a0, a0, level - 1);
			bn_sqr_karat_impl(a1a1, a1, level - 1);
		}

		/* t = (a1 + a0) */
		bn_add(t, a1, a0);

		if (level <= 1) {
			/* t = (a1 + a0)*(a1 + a0) */
#if BN_SQR == BASIC
			bn_sqr_basic(t, t);
#elif BN_SQR == COMBA
			bn_sqr_comba(t, t);
#endif
		} else {
			bn_sqr_karat_impl(t, t, level - 1);
		}

		/* t2 = (a0*a0 + a1*a1) */
		bn_add(a0, a0a0, a1a1);
		/* t = (a1 + a0)*(b1 + b0) - (a0*a0 + a1*a1) */
		bn_sub(t, t, a0);

		/* t = (a1 + a0)*(a1 + a0) - (a0*a0 + a1*a1) << h digits */
		bn_lsh(t, t, h * BN_DIGIT);

		/* t2 = a1 * b1 << 2*h digits */
		bn_lsh(a1a1, a1a1, 2 * h * BN_DIGIT);

		/* t = t + a0*a0 */
		bn_add(t, t, a0a0);
		/* c = t + a1*a1 */
		bn_add(t, t, a1a1);
		t->sign = a->sign;

		bn_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(a0);
		bn_free(a1);
		bn_free(a0a0);
		bn_free(a1a1);
		bn_free(t);
	}
}

#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if BN_SQR == BASIC || !defined(STRIP)

void bn_sqr_basic(bn_t c, bn_t a) {
	int i, digits;
	bn_t t = NULL;

	digits = 2 * a->used;

	TRY {
		bn_new_size(t, digits);
		bn_zero(t);
		t->used = digits;

		for (i = 0; i < a->used; i++) {
			bn_sqradd_low(t->dp + (2 * i), a->dp + i, a->used - i);
		}

		bn_trim(t);
		bn_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
	}
}

#endif

#if BN_SQR == COMBA || !defined(STRIP)

void bn_sqr_comba(bn_t c, bn_t a) {
	int digits;
	bn_t t = NULL;

	digits = 2 * a->used;

	TRY {
		/* We need a temporary variable so that c can be a or b. */
		bn_new_size(t, digits);
		t->used = digits;

		bn_sqrn_low(t->dp, a->dp, a->used);

		t->sign = a->sign ^ a->sign;
		bn_trim(t);

		bn_copy(c, t);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
	}
}

#endif

#if BN_KARAT > 0 || !defined(STRIP)

void bn_sqr_karat(bn_t c, bn_t a) {
	bn_sqr_karat_impl(c, a, BN_KARAT);
}

#endif
