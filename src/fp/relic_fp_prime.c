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
 * Auxiliar value computed as R^2 mod m used to convert small integers
 * to Montgomery form.
 */
static bn_st conv, one;

/**
 * Prime modulus modulo 8.
 */
static dig_t prime_mod8;

/**
 * Prime modulus modulo 5.
 */
static dig_t prime_mod5;

/**
 * Quadratic non-residue.
 */
static int prime_qnr;

/**
 * Cubic non-residue.
 */
static int prime_cnr;

/**
 * Maximum number of powers of 2 used to describe special form moduli.
 */
#define MAX_BITS		10

/**
 * Non-zero bits of special form prime.
 */
static int spars[MAX_BITS + 1] = { 0 };

/**
 * Number of bits of special form prime.
 */
static int spars_len = 0;

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_prime_init() {
	bn_init(&prime, FP_DIGS);
	bn_init(&u, FP_DIGS);
	bn_init(&conv, FP_DIGS);
	bn_init(&one, FP_DIGS);
}

void fp_prime_clean() {
	bn_clean(&prime);
	bn_clean(&u);
	bn_clean(&conv);
	bn_clean(&one);
}

dig_t *fp_prime_get(void) {
	return prime.dp;
}

dig_t *fp_prime_get_rdc(void) {
	return u.dp;
}

int *fp_prime_get_spars(int *len) {
	if (spars_len > 0 && spars_len < MAX_BITS ) {
		if (len != NULL) {
			*len = spars_len;
		}
		return spars;
	} else {
		if (len != NULL) {
			*len = 0;
		}
		return NULL;
	}
}

dig_t *fp_prime_get_conv(void) {
	return conv.dp;
}

dig_t fp_prime_get_mod8() {
	return prime_mod8;
}

dig_t fp_prime_get_mod5() {
	return prime_mod5;
}

int fp_prime_get_qnr() {
	return prime_qnr;
}

int fp_prime_get_cnr() {
	return prime_cnr;
}

void fp_prime_set(bn_t p) {
	if (p->used != FP_DIGS) {
		THROW(ERR_INVALID);
	}

	bn_copy(&prime, p);

	bn_mod_dig(&prime_mod5, &prime, 5);
	bn_mod_dig(&prime_mod8, &prime, 8);

	switch (prime_mod8) {
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
	bn_mod_monty_setup(&u, &prime);
	bn_set_dig(&conv, 1);
	bn_set_dig(&one, 1);
	bn_lsh(&conv, &conv, 2 * prime.used * BN_DIGIT);
	bn_mod(&conv, &conv, &prime);
	bn_lsh(&one, &one, prime.used * BN_DIGIT);
	bn_mod(&one, &one, &prime);
}

void fp_prime_set_dense(bn_t p) {
	fp_prime_set(p);
	spars_len = 0;
	spars[0] = 0;
#if FP_RDC == QUICK
	THROW(ERR_INVALID);
#endif
}

void fp_prime_set_spars(int *f, int len) {
	bn_t p, t;

	bn_null(p);
	bn_null(t);

	TRY {
		bn_new(p);
		bn_new(t);

		if (len >= MAX_BITS) {
			THROW(ERR_INVALID);
		}

		bn_set_2b(p, FP_BITS);
		for (int i = len - 1; i > 0; i--) {
			if (f[i] == FP_BITS) {
				continue;
			}

			if (f[i] > 0) {
				bn_set_2b(t, f[i]);
				bn_add(p, p, t);
			} else {
				bn_set_2b(t, -f[i]);
				bn_sub(p, p, t);
			}
		}
		if (f[0] > 0) {
			bn_add_dig(p, p, f[0]);
		} else {
			bn_sub_dig(p, p, -f[0]);
		}

		fp_prime_set(p);
		for (int i = 0; i < len; i++) {
			spars[i] = f[i];
		}
		spars[len] = 0;
		spars_len = len;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(p);
		bn_free(t);
	}
}

void fp_prime_conv(fp_t c, bn_t a) {
	bn_t t;
	int i;

	bn_null(t);

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
			for (; i < FP_DIGS; i++) {
				c[i] = 0;
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

	bn_null(t);

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

		fp_zero(c);
		for (i = 0; i < t->used; i++) {
			c[i] = t->dp[i];
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

	bn_null(t);

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
