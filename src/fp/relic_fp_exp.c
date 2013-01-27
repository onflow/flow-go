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
 * Implementation of prime field exponentiation functions.
 *
 * @version $Id$
 * @ingroup fp
 */

#include "relic_core.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if FP_EXP == BASIC || !defined(STRIP)

void fp_exp_basic(fp_t c, fp_t a, bn_t b) {
	int i, l;
	fp_t r;

	fp_null(r);

	TRY {
		fp_new(r);

		l = bn_bits(b);

		fp_copy(r, a);

		for (i = l - 2; i >= 0; i--) {
			fp_sqr(r, r);
			if (bn_test_bit(b, i)) {
				fp_mul(r, r, a);
			}
		}

		fp_copy(c, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(r);
	}
}

#endif

#if FP_EXP == SLIDE || !defined(STRIP)

void fp_exp_slide(fp_t c, fp_t a, bn_t b) {
	fp_t tab[1 << (FP_WIDTH - 1)], t;
	int i, j, l;
	unsigned char win[FP_BITS];

	fp_null(t);

	/* Initialize table. */
	for (i = 0; i < (1 << (FP_WIDTH - 1)); i++) {
		fp_null(tab[i]);
	}

	TRY {
		for (i = 0; i < (1 << (FP_WIDTH - 1)); i ++) {
			fp_new(tab[i]);
		}
		fp_new(t);

		fp_copy(tab[0], a);
		fp_sqr(t, a);

		/* Create table. */
		for (i = 1; i < 1 << (FP_WIDTH - 1); i++) {
			fp_mul(tab[i], tab[i - 1], t);
		}

		fp_set_dig(t, 1);
		bn_rec_slw(win, &l, b, FP_WIDTH);
		for (i = 0; i < l; i++) {
			if (win[i] == 0) {
				fp_sqr(t, t);
			} else {
				for (j = 0; j < util_bits_dig(win[i]); j++) {
					fp_sqr(t, t);
				}
				fp_mul(t, t, tab[win[i] >> 1]);
			}
		}

		fp_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		for (i = 0; i < (1 << (FP_WIDTH - 1)); i++) {
			fp_free(tab[i]);
		}
		fp_free(t);
	}
}

#endif

#if FP_EXP == MONTY || !defined(STRIP)

void fp_exp_monty(fp_t c, fp_t a, bn_t b) {
	fp_t tab[2];
	dig_t buf;
	int bitcnt, digidx, j;

	fp_null(tab[0]);
	fp_null(tab[1]);

	TRY {
		fp_new(tab[0]);
		fp_new(tab[1]);

		fp_set_dig(tab[0], 1);
		fp_copy(tab[1], a);

		/* Set initial mode and bitcnt, */
		bitcnt = 1;
		buf = 0;
		digidx = b->used - 1;

		for (;;) {
			/* Grab next digit as required. */
			if (--bitcnt == 0) {
				/* If digidx == -1 we are out of digits so break. */
				if (digidx == -1) {
					break;
				}
				/* Read next digit and reset bitcnt. */
				buf = b->dp[digidx--];
				bitcnt = (int)FP_DIGIT;
			}

			/* Grab the next msb from the exponent. */
			j = (buf >> (FP_DIGIT - 1)) & 0x01;
			buf <<= (dig_t)1;

			fp_mul(tab[j ^ 1], tab[0], tab[1]);
			fp_sqr(tab[j], tab[j]);
		}

		fp_copy(c, tab[0]);

	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(tab[1]);
		fp_free(tab[0]);
	}
}

#endif
