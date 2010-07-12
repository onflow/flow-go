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
 * Implementation of arithmetic in a quadratic extension over a prime field.
 *
 * @version $Id$
 * @ingroup fp2
 */

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_fp.h"
#include "relic_fp_low.h"
#include "relic_pp.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Constant used to compute the Frobenius map in higher extensions.
 */
fp2_st const_frb;

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp2_const_init() {
#if ALLOC == STATIC
	fp2_new(const_frb);
#endif
}

void fp2_const_clean() {
#if ALLOC == STATIC
	fp2_free(const_frb);
#endif
}

void fp2_const_calc() {
	bn_t e;
	fp2_t t;

	bn_null(e);
	fp2_null(t);

	TRY {
		bn_new(e);
		fp2_new(t);
		fp2_zero(t);
		fp_set_dig(t[0], 1);
		fp2_mul_nor(t, t);
		e->used = FP_DIGS;
		dv_copy(e->dp, fp_prime_get(), FP_DIGS);
		bn_sub_dig(e, e, 1);
		bn_div_dig(e, e, 6);
		fp2_exp(t, t, e);
		fp_copy(const_frb[0], t[0]);
		fp_copy(const_frb[1], t[1]);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		bn_free(e);
		fp2_free(t);
	}
}

void fp2_const_get(fp2_t f) {
	fp_copy(f[0], const_frb[0]);
	fp_copy(f[1], const_frb[1]);
}

void fp2_copy(fp2_t c, fp2_t a) {
	fp_copy(c[0], a[0]);
	fp_copy(c[1], a[1]);
}

void fp2_neg(fp2_t c, fp2_t a) {
	fp_neg(c[0], a[0]);
	fp_neg(c[1], a[1]);
}

void fp2_zero(fp2_t a) {
	fp_zero(a[0]);
	fp_zero(a[1]);
}

int fp2_is_zero(fp2_t a) {
	return fp_is_zero(a[0]) && fp_is_zero(a[1]);
}

void fp2_rand(fp2_t a) {
	fp_rand(a[0]);
	fp_rand(a[1]);
}

void fp2_print(fp2_t a) {
	fp_print(a[0]);
	fp_print(a[1]);
}

int fp2_cmp(fp2_t a, fp2_t b) {
	return (fp_cmp(a[0], b[0]) == CMP_EQ) &&
			(fp_cmp(a[1], b[1]) == CMP_EQ) ? CMP_EQ : CMP_NE;
}

void fp2_add(fp2_t c, fp2_t a, fp2_t b) {
  fp_add(c[0], a[0], b[0]);
  fp_add(c[1], a[1], b[1]);
}

void fp2_sub(fp2_t c, fp2_t a, fp2_t b) {
  fp_sub(c[0], a[0], b[0]);
  fp_sub(c[1], a[1], b[1]);
}

void fp2_dbl(fp2_t c, fp2_t a) {
  /* 2 * (a0 + a1 * u) = 2 * a0 + 2 * a1 * u. */
  fp_dbl(c[0], a[0]);
  fp_dbl(c[1], a[1]);
}

void fp2_mul(fp2_t c, fp2_t a, fp2_t b) {
	dv_t t0, t1, t2, t3;

	dv_null(t0);
	dv_null(t1);
	dv_null(t2);
	dv_null(t3);

	TRY {

		dv_new(t0);
		dv_new(t1);
		dv_new(t2);
		dv_new(t3);

		/* Karatsuba algorithm. */

		/* t1 = a0 + a1, c1 = b0 + b1. */
#if (FP_PRIME % WORD) == (WORD - 2)
		fp_addn_low(t2, a[0], a[1]);
		fp_addn_low(t1, b[0], b[1]);
#else
		fp_add(t2, a[0], a[1]);
		fp_add(t1, b[0], b[1]);
#endif
		/* t1 = (a0 + a1) * (b0 + b1). */
		/* t0 = a0 * b0, t1 = a1 * b1. */
		fp_muln_low(t3, t2, t1);
		fp_muln_low(t0, a[0], b[0]);
		fp_muln_low(t1, a[1], b[1]);

		/* t2 = (a0 * a1) + (b0 * b1). */
		fp_addd_low(t2, t0, t1);

		/* t0 = (a0 * a1) + u^2 * (a1 * b1). */
		fp_subc_low(t0, t0, t1);

		/* t1 = u^2 * (a1 * b1). */
		for (int i = -1; i > fp_prime_get_qnr(); i--) {
			fp_subd_low(t0, t0, t1);
		}

		/* c0 = t0 mod p. */
		fp_rdcn_low(c[0], t0);

#if (FP_PRIME % WORD) == (WORD - 2)
		fp_subc_low(t3, t3, t2);
#else
		fp_subd_low(t3, t3, t2);
#endif

		/* c1 = t1 mod p. */
		fp_rdcn_low(c[1], t3);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t0);
		dv_free(t1);
		dv_free(t2);
		dv_free(t3);
	}
}

void fp2_mul_art(fp2_t c, fp2_t a) {
	fp_t t0;

	fp_null(t0);

	TRY {
		fp_new(t0);

		/* (a0 + a1 * u) * u = a1 * u^2 + a0 * u. */
		fp_copy(t0, a[0]);
		fp_neg(c[0], a[1]);
		for (int i = -1; i > fp_prime_get_qnr(); i--) {
			fp_sub(c[0], c[0], a[1]);
		}
		fp_copy(c[1], t0);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(t0);
	}
}

void fp2_mul_nor(fp2_t c, fp2_t a) {
	fp2_t t0;

	fp2_null(t0);

	TRY {
		fp2_new(t0);

		switch (fp_prime_get_mod8()) {
			case 5:
				/* If p = 5 mod 8, x^2 - sqrt(sqrt(-2)) is irreducible. */
				fp2_mul_art(c, a);
				break;
			case 3:
				/* If p = 3 mod 8, x^2 - sqrt(1 + sqrt(-1)) is irreducible. */
				fp_neg(t0[0], a[1]);
				fp_add(c[1], a[0], a[1]);
				fp_add(c[0], t0[0], a[0]);
				break;
			case 7:
				/* If p = 7 mod 8 and p = 2,3 mod 5, x^2 - sqrt(2 + sqrt(-1)) is
				 * irreducible. */
				fp2_mul_art(t0, a);
				fp2_add(c, t0, a);
				break;
			default:
				THROW(ERR_INVALID);
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(t0);
	}
}

void fp2_sqr(fp2_t c, fp2_t a) {
	fp_t t0, t1, t2;

	fp_null(t0);
	fp_null(t1);
	fp_null(t2);

	TRY {
		fp_new(t0);
		fp_new(t1);
		fp_new(t2);

		/* t0 = (a0 + a1). */
#if (FP_PRIME % WORD) == (WORD - 2)
		fp_addn_low(t0, a[0], a[1]);
#else
		fp_add(t0, a[0], a[1]);
#endif
		/* t1 = (a0 - a1). */
		fp_sub(t1, a[0], a[1]);

		/* t1 = u^2 * (a1 * b1). */
		for (int i = -1; i > fp_prime_get_qnr(); i--) {
			fp_sub(t1, t1, a[1]);
		}

		if (fp_prime_get_qnr() == -1) {
			/* t2 = 2 * a0. */
#if (FP_PRIME % WORD) == (WORD - 2)
			fp_dbln_low(t2, a[0]);
#else
			fp_dbl(t2, a[0]);
#endif
			/* c_1 = 2 * a0 * a1. */
			fp_mul(c[1], t2, a[1]);
			/* c_0 = a0^2 + b_0^2 * u^2. */
			fp_mul(c[0], t0, t1);
		} else {
			/* c_1 = a0 * a1. */
			fp_mul(c[1], a[0], a[1]);
			/* c_0 = a0^2 + b_0^2 * u^2. */
			fp_mul(c[0], t0, t1);
			for (int i = -1; i > fp_prime_get_qnr(); i--) {
				fp_add(c[0], c[0], c[1]);
			}
			/* c_1 = 2 * a0 * a1. */
			fp_dbl(c[1], c[1]);
		}

		/* c = c_0 + c_1 * u. */
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(t0);
		fp_free(t1);
		fp_free(t2);
	}
}

void fp2_inv(fp2_t c, fp2_t a) {
	fp_t t0, t1;

	fp_null(t0);
	fp_null(t1);

	TRY {
		fp_new(t0);
		fp_new(t1);

		/* t0 = a0^2, t1 = a1^2. */
		fp_sqr(t0, a[0]);
		fp_sqr(t1, a[1]);

		if (fp_prime_get_qnr() == -2) {
			fp_dbl(t1, t1);
		}

		/* t1 = 1/(a0^2 + a1^2). */
		fp_add(t0, t0, t1);
		fp_inv(t1, t0);

		/* c_0 = a0/(a0^2 + a1^2). */
		fp_mul(c[0], a[0], t1);
		/* c_1 = - a1/(a0^2 + a1^2). */
		fp_mul(c[1], a[1], t1);
		fp_neg(c[1], c[1]);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(t0);
		fp_free(t1);
	}
}

void fp2_frb(fp2_t c, fp2_t a) {
	/* (a0 + a1 * u)^p = a0 - a1 * u. */
	fp_copy(c[0], a[0]);
	fp_neg(c[1], a[1]);
}

void fp2_exp(fp2_t c, fp2_t a, bn_t b) {
	fp2_t t;

	fp2_null(t);

	TRY {
		fp2_new(t);

		fp2_copy(t, a);

		for (int i = bn_bits(b) - 2; i >= 0; i--) {
			fp2_sqr(t, t);
			if (bn_test_bit(b, i)) {
				fp2_mul(t, t, a);
			}
		}
		fp2_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(t);
	}
}

int fp2_srt(fp2_t c, fp2_t a) {
	int r = 0;
	fp_t t1;
	fp_t t2;
	fp_t t3;

	fp_null(t1);
	fp_null(t2);
	fp_null(t3);

	TRY {
		fp_new(t1);
		fp_new(t2);
		fp_new(t3);

		/* t1 = a[0]^2 - u^2 * a[1]^2 */
		fp_sqr(t1, a[0]);
		fp_sqr(t2, a[1]);
		if (fp_prime_get_qnr() == -2) {
			fp_dbl(t2, t2);
		}
		fp_add(t1, t1, t2);

		if (fp_srt(t2, t1)) {
			/* t1 = (a[0] + sqrt(t1)) / 2 */
			fp_add(t1, a[0], t2);
			fp_set_dig(t3, 2);
			fp_inv(t3, t3);
			fp_mul(t1, t1, t3);

			if (!fp_srt(t3, t1)) {
				/* t1 = (a[0] - sqrt(t1)) / 2 */
				fp_sub(t1, a[0], t2);
				fp_set_dig(t3, 2);
				fp_inv(t3, t3);
				fp_mul(t1, t1, t3);
				fp_srt(t3, t1);
			}
			/* c0 = sqrt(t1) */
			fp_copy(c[0], t3);
			/* c1 = a1 / (2 * sqrt(t1)) */
			fp_dbl(t3, t3);
			fp_inv(t3, t3);
			fp_mul(c[1], a[1], t3);
			r = 1;
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(t1);
		fp_free(t2);
		fp_free(t3);
	}
	return r;
}
