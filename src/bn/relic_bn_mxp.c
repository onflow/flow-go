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
 * Implementation of the multiple precision exponentiation functions.
 *
 * @version $Id$
 * @ingroup bn
 */

#include <string.h>

#include "relic_core.h"
#include "relic_bn.h"
#include "relic_bn_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Size of precomputation table.
 */
#define TABLE_SIZE			32

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if BN_MXP == BASIC || !defined(STRIP)

void bn_mxp_basic(bn_t c, bn_t a, bn_t b, bn_t m) {
	int i, l;
	bn_t t, u, r;

	bn_null(t);
	bn_null(u);
	bn_null(r);

	TRY {
		bn_new(t);
		bn_new(u);
		bn_new(r);

		bn_mod_setup(u, m);

		l = bn_bits(b);

#if BN_MOD == MONTY
		bn_mod_monty_conv(t, a, m);
#else
		bn_copy(t, a);
#endif

		bn_copy(r, t);

		for (i = l - 2; i >= 0; i--) {
			bn_sqr(r, r);
			bn_mod(r, r, m, u);
			if (bn_test_bit(b, i)) {
				bn_mul(r, r, t);
				bn_mod(r, r, m, u);
			}
		}

#if BN_MOD == MONTY
		bn_mod_monty_back(c, r, m);
#else
		bn_copy(c, r);
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
		bn_free(u);
		bn_free(r);
	}
}

#endif

#if BN_MXP == SLIDE || !defined(STRIP)

void bn_mxp_slide(bn_t c, bn_t a, bn_t b, bn_t m) {
	bn_t tab[TABLE_SIZE], t, u, r;
	dig_t buf;
	int bitbuf, bitcpy, bitcnt, mode, digidx, i, j, w = 0;

	bn_null(t);
	bn_null(u);
	bn_null(r);
	for (i = 0; i < TABLE_SIZE; i++) {
		bn_null(tab[i]);
	}

	TRY {

		/* Find window size. */
		i = bn_bits(b);
		if (i <= 21) {
			w = 2;
		} else if (i <= 36) {
			w = 3;
		} else if (i <= 140) {
			w = 4;
		} else if (i <= 450) {
			w = 5;
		} else {
			w = 6;
		}

		/* Initialize table. */
		memset(tab, 0, sizeof(tab));

		for (i = 0; i < (1 << (w - 1)); i++) {
			bn_new(tab[i]);
		}

		bn_new(t);
		bn_new(u);
		bn_new(r);
		bn_mod_setup(u, m);

#if BN_MOD == MONTY
		bn_set_dig(r, 1);
		bn_mod_monty_conv(r, r, m);
		bn_mod_monty_conv(t, a, m);
#else /* BN_MOD == BARRT || BN_MOD == RADIX */
		bn_set_dig(r, 1);
		bn_copy(t, a);
#endif

		bn_copy(tab[0], t);
		/* Compute the value at tab[0] by squaring a (w - 1) times. */
		for (i = 0; i < (w - 1); i++) {
			bn_sqr(tab[0], tab[0]);
			bn_mod(tab[0], tab[0], m, u);
		}

		/* Create upper table. */
		for (i = 1; i < (1 << (w - 1)); i++) {
			bn_mul(tab[i], tab[i - 1], t);
			bn_mod(tab[i], tab[i], m, u);
		}

		/* Set initial mode and bit count. */
		mode = 0;
		bitcnt = 1;
		buf = 0;
		digidx = b->used - 1;
		bitcpy = 0;
		bitbuf = 0;

		for (;;) {
			/* Grab next digit as required. */
			if (--bitcnt == 0) {
				/* If digidx == -1 we are out of digits so break. */
				if (digidx == -1) {
					break;
				}
				/* Read next digit and set bitcnt. */
				buf = b->dp[digidx--];
				bitcnt = (int)BN_DIGIT;
			}

			/* Grab the next most significant bit from the exponent. */
			j = (buf >> (BN_DIGIT - 1)) & 0x01;
			buf <<= (dig_t)1;

			if (mode == 0 && j == 0) {
				continue;
			}

			/* If the bit is zero and mode == 1 then we square. */
			if (mode == 1 && j == 0) {
				bn_sqr(r, r);
				bn_mod(r, r, m, u);
				continue;
			}

			/* Else we add it to the window. */
			bitbuf |= (j << (w - ++bitcpy));
			mode = 2;

			if (bitcpy == w) {
				/* Window is filled so square as required and multiply. */
				for (i = 0; i < w; i++) {
					bn_sqr(r, r);
					bn_mod(r, r, m, u);
				}
				bn_mul(r, r, tab[bitbuf - (1 << (w - 1))]);
				bn_mod(r, r, m, u);
				bitcpy = 0;
				bitbuf = 0;
				mode = 1;
			}
		}

		/* If bits remain then square/multiply. */
		if (mode == 2 && bitcpy > 0) {
			/* Square then multiply if the bit is set. */
			for (i = 0; i < bitcpy; i++) {
				bn_sqr(r, r);
				bn_mod(r, r, m, u);

				/* Get next bit of the window. */
				bitbuf <<= 1;
				if ((bitbuf & (1 << w)) != 0) {
					bn_mul(r, r, t);
					bn_mod(r, r, m, u);
				}
			}
		}

		bn_trim(r);
#if BN_MOD == MONTY
		bn_mod_monty_back(c, r, m);
#else
		bn_copy(c, r);
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		for (i = 0; i < (1 << (w - 1)); i++) {
			bn_free(tab[i]);
		}
		bn_free(u);
		bn_free(t);
		bn_free(r);
	}
}

#endif

#if BN_MXP == CONST || !defined(STRIP)

void bn_mxp_const(bn_t c, bn_t a, bn_t b, bn_t m) {
	bn_t tab[2], u;
	dig_t buf;
	int bitcnt, digidx, j;

	bn_null(tab[0]);
	bn_null(tab[1]);
	bn_null(u);

	TRY {
		bn_new(u);
		bn_mod_setup(u, m);

		bn_new(tab[0]);
		bn_new(tab[1]);

#if BN_MOD == MONTY
		bn_set_dig(tab[0], 1);
		bn_mod_monty_conv(tab[0], tab[0], m);
		bn_mod_monty_conv(tab[1], a, m);
#else /* BN_MOD == BARRT || BN_MOD == RADIX */
		bn_set_dig(tab[0], 1);
		bn_copy(tab[1], a);
#endif

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
				bitcnt = (int)BN_DIGIT;
			}

			/* Grab the next msb from the exponent. */
			j = (buf >> (BN_DIGIT - 1)) & 0x01;
			buf <<= (dig_t)1;

			bn_mul(tab[j ^ 1], tab[0], tab[1]);
			bn_mod(tab[j ^ 1], tab[j ^ 1], m, u);
			bn_sqr(tab[j], tab[j]);
			bn_mod(tab[j], tab[j], m, u);
		}

#if BN_MOD == MONTY
		bn_mod_monty_back(c, tab[0], m);
#else
		bn_copy(c, tab[0]);
#endif

	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(tab[1]);
		bn_free(tab[0]);
		bn_free(u);
	}
}

#endif
