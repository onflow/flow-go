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
 * Implementation of multiplication in a quadratic extension of a prime field.
 *
 * @version $Id$
 * @ingroup fpx
 */

#include "relic_core.h"
#include "relic_fp_low.h"
#include "relic_pp_low.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Constant used to compute the Frobenius map in higher extensions.
 */
static fp2_st const_frb[5];

/**
 * Constant used to compute consecutive Frobenius maps in higher extensions.
 */
static fp_st const_sqr[3];

/**
 * Constant used to compute the Frobenius map in higher extensions.
 */
static fp2_st const_cub[5];

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp2_const_calc() {
	bn_t e;
	fp2_t t0;
	fp2_t t1;

	bn_null(e);
	fp2_null(t0);
	fp2_null(t1);

	TRY {
		bn_new(e);
		fp2_new(t0);
		fp2_new(t1);

		fp2_zero(t0);
		fp_set_dig(t0[0], 1);
		fp2_mul_nor(t0, t0);
		e->used = FP_DIGS;
		dv_copy(e->dp, fp_prime_get(), FP_DIGS);
		bn_sub_dig(e, e, 1);
		bn_div_dig(e, e, 6);
		fp2_exp(t0, t0, e);
#if ALLOC == AUTO
		fp2_copy(const_frb[0], t0);
		fp2_sqr(const_frb[1], const_frb[0]);
		fp2_mul(const_frb[2], const_frb[1], const_frb[0]);
		fp2_sqr(const_frb[3], const_frb[1]);
		fp2_mul(const_frb[4], const_frb[3], const_frb[0]);
#else
		fp_copy(const_frb[0][0], t0[0]);
		fp_copy(const_frb[0][1], t0[1]);
		fp2_sqr(t1, t0);
		fp_copy(const_frb[1][0], t1[0]);
		fp_copy(const_frb[1][1], t1[1]);
		fp2_mul(t1, t1, t0);
		fp_copy(const_frb[2][0], t1[0]);
		fp_copy(const_frb[2][1], t1[1]);
		fp2_sqr(t1, t0);
		fp2_sqr(t1, t1);
		fp_copy(const_frb[3][0], t1[0]);
		fp_copy(const_frb[3][1], t1[1]);
		fp2_mul(t1, t1, t0);
		fp_copy(const_frb[4][0], t1[0]);
		fp_copy(const_frb[4][1], t1[1]);
#endif
		fp2_frb(t1, t0, 1);
		fp2_mul(t0, t1, t0);
		fp_copy(const_sqr[0], t0[0]);
		fp_sqr(const_sqr[1], const_sqr[0]);
		fp_mul(const_sqr[2], const_sqr[1], const_sqr[0]);

		for (int i = 0; i < 5; i++) {
			fp_mul(const_cub[i][0], const_sqr[i % 3], const_frb[i][0]);
			fp_mul(const_cub[i][1], const_sqr[i % 3], const_frb[i][1]);
		}
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		bn_free(e);
		fp2_free(t0);
		fp2_free(t1);
	}
}

void fp2_mul_frb(fp2_t c, fp2_t a, int i, int j) {
	if (i == 2) {
		fp_mul(c[0], a[0], const_sqr[j - 1]);
		fp_mul(c[1], a[1], const_sqr[j - 1]);
	} else {
#if ALLOC == AUTO
		if (i == 1) {
			fp2_mul(c, a, const_frb[j - 1]);
		} else {
			fp2_mul(c, a, const_cub[j - 1]);
		}
#else
		fp2_t t;

		fp2_null(t);

		TRY {
			fp2_new(t);
			if (i == 1) {
				fp_copy(t[0], const_frb[j - 1][0]);
				fp_copy(t[1], const_frb[j - 1][1]);
			} else {
				fp_copy(t[0], const_cub[j - 1][0]);
				fp_copy(t[1], const_cub[j - 1][1]);
			}
			fp2_mul(c, a, t);
		}
		CATCH_ANY {
			THROW(ERR_CAUGHT);
		}
		FINALLY {
			fp2_free(t);
		}
#endif
	}
}

#if PP_QDR == BASIC || !defined(STRIP)

void fp2_mul_basic(fp2_t c, fp2_t a, fp2_t b) {
	dv_t t0, t1, t2, t3, t4;

	dv_null(t0);
	dv_null(t1);
	dv_null(t2);
	dv_null(t3);
	dv_null(t4);

	TRY {
		dv_new(t0);
		dv_new(t1);
		dv_new(t2);
		dv_new(t3);
		dv_new(t4);

		/* Karatsuba algorithm. */

		/* t2 = a_0 + a_1, t1 = b0 + b1. */
		fp_add(t2, a[0], a[1]);
		fp_add(t1, b[0], b[1]);

		/* t3 = (a_0 + a_1) * (b0 + b1). */
		fp_muln_low(t3, t2, t1);

		/* t0 = a_0 * b0, t4 = a_1 * b1. */
		fp_muln_low(t0, a[0], b[0]);
		fp_muln_low(t4, a[1], b[1]);

		/* t2 = (a_0 * b0) + (a_1 * b1). */
		fp_addc_low(t2, t0, t4);

		/* t1 = (a_0 * b0) + u^2 * (a_1 * b1). */
		fp_subc_low(t1, t0, t4);

		/* t1 = u^2 * (a_1 * b1). */
		for (int i = -1; i > fp_prime_get_qnr(); i--) {
			fp_subc_low(t1, t1, t4);
		}
		/* c_0 = t1 mod p. */
		fp_rdc(c[0], t1);

		/* t4 = t3 - t2. */
		fp_subc_low(t4, t3, t2);

		/* c_1 = t4 mod p. */
		fp_rdc(c[1], t4);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t0);
		dv_free(t1);
		dv_free(t2);
		dv_free(t3);
		dv_free(t4);
	}
}

void fp2_mul_nor_basic(fp2_t c, fp2_t a) {
	fp2_t t;
	bn_t b;

	fp2_null(t);
	bn_null(b);

	TRY {
		fp2_new(t);
		bn_new(b);

#ifdef FP_QNRES
		/* If p = 3 mod 8, (1 + i) is a QNR/CNR. */
		fp_neg(t[0], a[1]);
		fp_add(c[1], a[0], a[1]);
		fp_add(c[0], t[0], a[0]);
#else
		switch (fp_prime_get_mod8()) {
			case 3:
				/* If p = 3 mod 8, (1 + u) is a QNR/CNR. */
				fp_neg(t[0], a[1]);
				fp_add(c[1], a[0], a[1]);
				fp_add(c[0], t[0], a[0]);
				break;
			case 5:
				/* If p = 5 mod 8, (u) is a QNR/CNR. */
				fp2_mul_art(c, a);
				break;
			case 7:
				/* If p = 7 mod 8, we choose (2^log_4(b-1) + u) is a QNR/CNR. */
				fp2_mul_art(t, a);
				fp2_dbl(c, a);
				fp_prime_back(b, ep_curve_get_b());
				for (int i = 1; i < bn_bits(b) / 2; i++) {
					fp2_dbl(c, c);
				}
				fp2_add(c, c, t);
				break;
			default:
				THROW(ERR_NO_VALID);
		}
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(t);
		bn_free(b);
	}
}

#endif

#if PP_QDR == INTEG || !defined(STRIP)

void fp2_mul_integ(fp2_t c, fp2_t a, fp2_t b) {
	fp2_mulm_low(c, a, b);
}

void fp2_mul_nor_integ(fp2_t c, fp2_t a) {
	fp2_norm_low(c, a);
}

#endif

void fp2_mul_art(fp2_t c, fp2_t a) {
	fp_t t;

	fp_null(t);

	TRY {
		fp_new(t);

#ifdef FP_QNRES
		/* (a_0 + a_1 * i) * i = -a_1 + a_0 * i. */
		fp_copy(t, a[0]);
		fp_neg(c[0], a[1]);
		fp_copy(c[1], t);
#else
		/* (a_0 + a_1 * u) * u = (a_1 * u^2) + a_0 * u. */
		fp_copy(t, a[0]);
		fp_neg(c[0], a[1]);
		for (int i = -1; i > fp_prime_get_qnr(); i--) {
			fp_sub(c[0], c[0], a[1]);
		}
		fp_copy(c[1], t);
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(t);
	}
}
