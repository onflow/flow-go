/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
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
#include "relic_pp.h"

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
static dig_t u;

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
#define MAX_EXPS		10

/**
 * Non-zero bits of special form prime.
 */
static int spars[MAX_EXPS + 1] = { 0 };

/**
 * Number of bits of special form prime.
 */
static int spars_len = 0;

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_prime_init() {
	bn_init(&prime, FP_DIGS);
	bn_init(&conv, FP_DIGS);
	bn_init(&one, FP_DIGS);
}

void fp_prime_clean() {
	bn_clean(&one);
	bn_clean(&conv);
	bn_clean(&prime);
}

dig_t *fp_prime_get(void) {
	return prime.dp;
}

dig_t *fp_prime_get_rdc(void) {
	return &u;
}

int *fp_prime_get_sps(int *len) {
	if (spars_len > 0 && spars_len < MAX_EXPS ) {
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

int fp_prime_get_qnr() {
	return prime_qnr;
}

int fp_prime_get_cnr() {
	return prime_cnr;
}

void fp_prime_set(bn_t p) {
	dv_t s, q;
	bn_t t;

	if (p->used != FP_DIGS) {
		THROW(ERR_NO_VALID);
	}

	dv_null(s);
	bn_null(t);
	dv_null(q);

	TRY {
		dv_new(s);
		bn_new(t);
		dv_new(q);

		bn_copy(&prime, p);

		bn_mod_dig(&prime_mod8, &prime, 8);

		switch (prime_mod8) {
			case 3:
			case 7:
				prime_qnr = -1;
				/* The current code for extensions of Fp^3 relies on prime_qnr
				 * being also a cubic non-residue. */
				prime_cnr = 0;
				break;
			case 1:
			case 5:
				prime_qnr = prime_cnr = -2;
				break;
			default:
				prime_qnr = prime_cnr = 0;
				THROW(ERR_NO_VALID);
				break;
		}
	#ifdef FP_QNRES
		if (prime_mod8 != 3) {
			THROW(ERR_NO_VALID);
		}
	#endif

		bn_mod_pre_monty(t, &prime);
		u = t->dp[0];
		dv_zero(s, 2 * FP_DIGS);
		s[2 * FP_DIGS] = 1;
		dv_zero(q, 2 * FP_DIGS + 1);
		dv_copy(q, prime.dp, FP_DIGS);
		bn_divn_low(t->dp, conv.dp, s, 2 * FP_DIGS + 1, q, FP_DIGS);
		conv.used = FP_DIGS;
		bn_trim(&conv);
		bn_set_dig(&one, 1);
		bn_lsh(&one, &one, prime.used * BN_DIGIT);
		bn_mod(&one, &one, &prime);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		bn_free(t);
		dv_free(s);
		dv_free(q);
	}
}

void fp_prime_set_dense(bn_t p) {
	fp_prime_set(p);
	spars_len = 0;
	spars[0] = 0;

#if FP_RDC == QUICK
	THROW(ERR_NO_CONFIG);
#endif
}

void fp_prime_set_pmers(int *f, int len) {
	bn_t p, t;

	bn_null(p);
	bn_null(t);

	TRY {
		bn_new(p);
		bn_new(t);

		if (len >= MAX_EXPS) {
			THROW(ERR_NO_VALID);
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

	bn_null(t);

	TRY {
		bn_new(t);

#if FP_RDC == MONTY
		bn_lsh(t, a, FP_DIGS * FP_DIGIT);
		bn_mod(t, t, &prime);
		dv_copy(c, t->dp, FP_DIGS);
#else
		if (a->used > FP_DIGS) {
			THROW(ERR_NO_PRECI);
		}

		if (bn_is_zero(a)) {
			fp_zero(c);
		} else {
			int i;
			for (i = 0; i < a->used; i++) {
				c[i] = a->dp[i];
			}
			for (; i < FP_DIGS; i++) {
				c[i] = 0;
			}
		}
		(void)t;
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(t);
	}
}

void fp_prime_conv_dig(fp_t c, dig_t a) {
	dv_t t;

	bn_null(t);

	TRY {
		dv_new(t);

#if FP_RDC == MONTY
		if (a != 1) {
			dv_zero(t, 2 * FP_DIGS + 1);
			t[FP_DIGS] = fp_mul1_low(t, conv.dp, a);
			fp_rdc(c, t);
		} else {
			dv_copy(c, one.dp, FP_DIGS);
		}
#else
		fp_zero(c);
		c[0] = a;
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

void fp_prime_back(bn_t c, fp_t a) {
	dv_t t;
	int i;

	dv_null(t);

	TRY {
		dv_new(t);

		bn_grow(c, FP_DIGS);
		for (i = 0; i < FP_DIGS; i++) {
			c->dp[i] = a[i];
		}
#if FP_RDC == MONTY
		dv_zero(t, 2 * FP_DIGS + 1);
		dv_copy(t, a, FP_DIGS);
		fp_rdc(c->dp, t);
#endif
		c->used = FP_DIGS;
		c->sign = BN_POS;
		bn_trim(c);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}
