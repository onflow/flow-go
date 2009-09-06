/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of the prime field prime manipulation functions.
 *
 * @version $Id$
 * @ingroup fp
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_bn.h"
#include "relic_bn_low.h"
#include "relic_fp.h"
#include "relic_fp_low.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Prime modulus.
 */
static bn_st prime;

/**
 * Auxiliar value derived from the prime used in modular reduction.
 */
static bn_st u;

/**
 * Prime modulus modulo 8.
 */
static bn_st prime_mod8;

#if FP_RDC == MONTY

/**
 * Auxiliar value computed as R^2 mod m used to convert small integers
 * to Montgomery form.
 */
static bn_st conv, one;

#endif

/**
 * Quadratic non-residue.
 */
int prime_qnr;

/**
 * Cubic non-residue.
 *
 */
int prime_cnr;

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_prime_init() {
	bn_init(&prime, FP_DIGS);
	bn_init(&u, FP_DIGS);
	bn_init(&prime_mod8, FP_DIGS);
#if FP_RDC == MONTY
	bn_init(&conv, FP_DIGS);
	bn_init(&one, FP_DIGS);
#endif
}

void fp_prime_clean() {
	bn_clean(&prime);
	bn_clean(&u);
	bn_clean(&prime_mod8);
#if FP_RDC == MONTY
	bn_clean(&conv);
	bn_clean(&one);
#endif
}

dig_t *fp_prime_get(void) {
	return prime.dp;
}

dig_t *fp_prime_get_rdc(void) {
	return u.dp;
}

dig_t *fp_prime_get_conv(void) {
	return conv.dp;
}

int fp_prime_get_qnr() {
	return prime_qnr;
}

int fp_prime_get_cnr() {
	return prime_cnr;
}

dig_t *fp_prime_get_mod8() {
	return prime_mod8.dp;
}

void fp_prime_set(bn_t p) {
	if (p->used != FP_DIGS) {
		THROW(ERR_INVALID);
	}

	bn_copy(&prime, p);

	bn_mod_2b(&prime_mod8, &prime, 3);

	switch (prime_mod8.dp[0]) {
		case 3:
			prime_qnr = -1;
			break;
		case 5:
		case 7:
			prime_qnr = -2;
			break;
		default:
			prime_qnr = 0;
			break;
	}
#if FP_RDC == MONTY
	bn_mod_monty_setup(&u, &prime);
	bn_set_dig(&conv, 1);
	bn_lsh(&conv, &conv, 2 * prime.used * BN_DIGIT);
	bn_mod_basic(&conv, &conv, &prime);
	bn_set_dig(&one, 1);
	bn_lsh(&one, &one, prime.used * BN_DIGIT);
	bn_mod_basic(&one, &one, &prime);
#elif FP_RDC == RADIX
	bn_mod_radix_setup(&u, &prime);
#endif
}

void fp_prime_conv(fp_t c, bn_t a) {
	bn_t t;
	int i;

	TRY {
		bn_new(t);
		bn_zero(t);

#if FP_RDC == MONTY
		bn_mod_monty_conv(t, a, &prime);
#else
		bn_copy(t, a);
#endif
		if (t->used > FP_DIGS) {
			THROW(ERR_NO_PRECISION);
		}

		if (bn_is_zero(t)) {
			fp_zero(c);
		} else {
			for (i = 0; i < t->used; i++) {
				c[i] = t->dp[i];
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
	}
}

void fp_prime_conv_dig(fp_t c, dig_t a) {
	bn_t t;
	int i;

	TRY {
		bn_new(t);
		bn_zero(t);

#if FP_RDC == MONTY
		if (a != 1) {
			bn_mul_dig(t, &conv, a);
			bn_mod_monty(t, t, &prime, &u);
		} else {
			bn_copy(t, &one);
		}
#else
		bn_set_dig(t, a);
#endif
		if (t->used > FP_DIGS) {
			THROW(ERR_NO_PRECISION);
		}

		if (bn_is_zero(t)) {
			fp_zero(c);
		} else {
			for (i = 0; i < t->used; i++) {
				c[i] = t->dp[i];
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
	}
}

void fp_prime_back(bn_t c, fp_t a) {
	bn_t t;
	int i;

	TRY {
		bn_new(t);

		for (i = 0; i < FP_DIGS; i++) {
			c->dp[i] = a[i];
		}
		c->used = FP_DIGS;

#if FP_RDC == MONTY
		bn_mod_monty_back(c, c, &prime);
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
	}
}
