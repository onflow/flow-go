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

#include "relic_core.h"
#include "relic_bn_low.h"
#include "relic_fp_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp_prime_init() {
	ctx_t *ctx = core_get();
	ctx->fp_id = ctx->sps_len = 0;
	memset(ctx->sps, 0, sizeof(ctx->sps));
	memset(ctx->var, 0, sizeof(ctx->var));
	bn_init(&(ctx->prime), FP_DIGS);
	bn_init(&(ctx->conv), FP_DIGS);
	bn_init(&(ctx->one), FP_DIGS);
}

void fp_prime_clean() {
	ctx_t *ctx = core_get();
	ctx->fp_id = ctx->sps_len = 0;
	memset(ctx->sps, 0, sizeof(ctx->sps));
	memset(ctx->var, 0, sizeof(ctx->var));
	bn_clean(&(ctx->one));
	bn_clean(&(ctx->conv));
	bn_clean(&(ctx->prime));
}

dig_t *fp_prime_get(void) {
	return core_get()->prime.dp;
}

dig_t *fp_prime_get_rdc(void) {
	return &(core_get()->u);
}

int *fp_prime_get_sps(int *len) {
	ctx_t *ctx = core_get();
	if (ctx->sps_len > 0 && ctx->sps_len < MAX_TERMS ) {
		if (len != NULL) {
			*len = ctx->sps_len;
		}
		return ctx->sps;
	} else {
		if (len != NULL) {
			*len = 0;
		}
		return NULL;
	}
}

dig_t *fp_prime_get_conv(void) {
	return core_get()->conv.dp;
}

dig_t fp_prime_get_mod8() {
	return core_get()->mod8;
}

int fp_prime_get_qnr() {
	return core_get()->qnr;
}

int fp_prime_get_cnr() {
	return core_get()->cnr;
}

void fp_prime_set(bn_t p) {
	dv_t s, q;
	bn_t t;
	ctx_t *ctx = core_get();

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

		bn_copy(&(ctx->prime), p);

		bn_mod_dig(&(ctx->mod8), &(ctx->prime), 8);

		switch (ctx->mod8) {
			case 3:
			case 7:
				ctx->qnr = -1;
				/* The current code for extensions of Fp^3 relies on prime_qnr
				 * being also a cubic non-residue. */
				ctx->cnr = 0;
				break;
			case 1:
			case 5:
				ctx->qnr = ctx->cnr = -2;
				break;
			default:
				ctx->qnr = ctx->cnr = 0;
				THROW(ERR_NO_VALID);
				break;
		}
	#ifdef FP_QNRES
		if (ctx->mod8 != 3) {
			THROW(ERR_NO_VALID);
		}
	#endif

		bn_mod_pre_monty(t, &(ctx->prime));
		ctx->u = t->dp[0];
		dv_zero(s, 2 * FP_DIGS);
		s[2 * FP_DIGS] = 1;
		dv_zero(q, 2 * FP_DIGS + 1);
		dv_copy(q, ctx->prime.dp, FP_DIGS);
		bn_divn_low(t->dp, ctx->conv.dp, s, 2 * FP_DIGS + 1, q, FP_DIGS);
		ctx->conv.used = FP_DIGS;
		bn_trim(&(ctx->conv));
		bn_set_dig(&(ctx->one), 1);
		bn_lsh(&(ctx->one), &(ctx->one), ctx->prime.used * BN_DIGIT);
		bn_mod(&(ctx->one), &(ctx->one), &(ctx->prime));
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
	core_get()->sps_len = 0;

#if FP_RDC == QUICK
	THROW(ERR_NO_CONFIG);
#endif
}

void fp_prime_set_pmers(int *f, int len) {
	bn_t p, t;
	ctx_t *ctx = core_get();

	bn_null(p);
	bn_null(t);

	TRY {
		bn_new(p);
		bn_new(t);

		if (len >= MAX_TERMS) {
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
			ctx->sps[i] = f[i];
		}
		ctx->sps[len] = 0;
		ctx->sps_len = len;
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
		bn_mod(t, a, &(core_get()->prime));
		bn_lsh(t, t, FP_DIGS * FP_DIGIT);
		bn_mod(t, t, &(core_get()->prime));
		dv_copy(c, t->dp, FP_DIGS);
#else
		if (a->used > FP_DIGS) {
			THROW(ERR_NO_PRECI);
		}

		bn_mod(t, a, &(core_get()->prime));

		if (bn_is_zero(t)) {
			fp_zero(c);
		} else {
			int i;
			for (i = 0; i < t->used; i++) {
				c[i] = t->dp[i];
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
	ctx_t *ctx = core_get();

	bn_null(t);

	TRY {
		dv_new(t);

#if FP_RDC == MONTY
		if (a != 1) {
			dv_zero(t, 2 * FP_DIGS + 1);
			t[FP_DIGS] = fp_mul1_low(t, ctx->conv.dp, a);
			fp_rdc(c, t);
		} else {
			dv_copy(c, ctx->one.dp, FP_DIGS);
		}
#else
		(void)ctx;
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
