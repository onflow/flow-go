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
 * Implementation of the dodecic extension prime field arithmetic module.
 *
 * @version $Id$
 * @ingroup fp12
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_pp.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Squaring in the internal quadratic extension over fp2.
 *
 * @param c					- the first component of the result.
 * @param d					- the second component of the result.
 * @param a					- the first component of the input.
 * @param b					- the second component of the input.
 */
static void fp4_sqr_uni(fp2_t c, fp2_t d, fp2_t a, fp2_t b) {
	fp2_t t0, t1;

	fp2_null(t0);
	fp2_null(t1);

	TRY {
		fp2_new(t0);
		fp2_new(t1);

		/* t0 = a + b. */
		fp2_add(t0, a, b);
		/* t1 = a + b * E. */
		fp2_mul_nor(t1, b);
		fp2_add(t1, t1, a);
		/* t0 = t0 * t2. */
		fp2_mul(t0, t0, t1);
		/* d = a * b. */
		fp2_mul(d, a, b);
		/* c = t0 - d - d * E. */
		fp2_sub(t0, t0, d);
		fp2_mul_nor(t1, d);
		fp2_sub(c, t0, t1);
		/* d = 2 * a * b. */
		fp2_dbl(d, d);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
	}
}

static void fp4_sqr(fp2_t c, fp2_t d, fp2_t a, fp2_t b) {
	fp2_t t0, t1;

	fp2_null(t0);
	fp2_null(t1);

	TRY {
		fp2_new(t0);
		fp2_new(t1);

		/* t0 = a + b. */
        fp2_add(t0, a, b);
        /* t1 = a + b * E. */
        fp2_mul_nor(t1, b);
        fp2_add(t1, a, t1);
        /* t0 = t0 * t1. */
        fp2_mul(t0, t0, t1);
        /* d = a * b. */
        fp2_mul(d, a, b);
        /* t0 = t0 - a * b. */
        fp2_sub(t0, t0, d);
        /* c = t0 - d * E. */
        fp2_mul_nor(t1, d);
        fp2_sub(c, t0, t1);
        /* d = 2 * a * b. */
        fp2_dbl(d, d);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
	}
}

static void fp4_mul(fp2_t e, fp2_t f, fp2_t a, fp2_t b, fp2_t c, fp2_t d) {
	fp2_t t0, t1, t2;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);

	TRY {
		fp2_new(t0);
		fp2_new(t1);

		fp2_mul(t0, a, c);
		fp2_mul(t1, b, d);
		fp2_add(t2, c, d);
		fp2_add(f, a, b);
		fp2_mul(f, f, t2);
		fp2_sub(f, f, t0);
		fp2_sub(f, f, t1);
		fp2_mul_nor(t1, t1);
		fp2_add(e, t0, t1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
	}
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp12_copy(fp12_t c, fp12_t a) {
	fp6_copy(c[0], a[0]);
	fp6_copy(c[1], a[1]);
}

void fp12_neg(fp12_t c, fp12_t a) {
	fp6_neg(c[0], a[0]);
	fp6_neg(c[1], a[1]);
}

void fp12_zero(fp12_t a) {
	fp6_zero(a[0]);
	fp6_zero(a[1]);
}

int fp12_is_zero(fp12_t a) {
	return (fp6_is_zero(a[0]) && fp6_is_zero(a[1]));
}

void fp12_rand(fp12_t a) {
	fp6_rand(a[0]);
	fp6_rand(a[1]);
}

void fp12_print(fp12_t a) {
	fp6_print(a[0]);
	fp6_print(a[1]);
}

int fp12_cmp(fp12_t a, fp12_t b) {
	return ((fp6_cmp(a[0], b[0]) == CMP_EQ) &&
			(fp6_cmp(a[1], b[1]) == CMP_EQ) ? CMP_EQ : CMP_NE);
}

int fp12_cmp_dig(fp12_t a, dig_t b) {
	return ((fp_cmp_dig(a[0][0][0], b) == CMP_EQ) &&
			fp_is_zero(a[0][0][1]) && fp2_is_zero(a[0][1]) &&
			fp2_is_zero(a[0][2]) && fp6_is_zero(a[1]));
}

void fp12_set_dig(fp12_t a, dig_t b) {
	fp12_zero(a);
	fp_set_dig(a[0][0][0], b);
}

void fp12_add(fp12_t c, fp12_t a, fp12_t b) {
	fp6_add(c[0], a[0], b[0]);
	fp6_add(c[1], a[1], b[1]);
}

void fp12_sub(fp12_t c, fp12_t a, fp12_t b) {
	fp6_sub(c[0], a[0], b[0]);
	fp6_sub(c[1], a[1], b[1]);
}

void fp12_mul(fp12_t c, fp12_t a, fp12_t b) {
	fp6_t t0, t1, t2;

	fp6_null(t0);
	fp6_null(t1);
	fp6_null(t2);

	TRY {
		fp6_new(t0);
		fp6_new(t1);
		fp6_new(t2);

		/* Karatsuba algorithm. */

		/* t0 = a0 * b0. */
		fp6_mul(t0, a[0], b[0]);
		/* t1 = a1 * b1. */
		fp6_mul(t1, a[1], b[1]);
		/* t2 = b0 + b1. */
		fp6_add(t2, b[0], b[1]);
		/* c1 = a0 + a1. */
		fp6_add(c[1], a[0], a[1]);
		/* c1 = (a0 + a1) * (b0 + b1) */
		fp6_mul(c[1], c[1], t2);
		fp6_sub(c[1], c[1], t0);
		fp6_sub(c[1], c[1], t1);
		fp6_mul_art(t1, t1);
		fp6_add(c[0], t0, t1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp6_free(t0);
		fp6_free(t1);
		fp6_free(t2);
	}
}

void fp12_mul_dxs(fp12_t c, fp12_t a, fp12_t b) {
	fp6_t v0, v1, t0;

	fp6_null(v0);
	fp6_null(v1);
	fp6_null(t0);

	TRY {
		fp6_new(v0);
		fp6_new(v1);
		fp6_new(t0);

		/* c1 = (a0 + a1)(b0 + b1) */
		fp6_add(v0, a[0], a[1]);
		fp2_add(v1[0], b[0][0], b[1][0]);
		fp2_copy(v1[1], b[1][1]);
		fp6_mul_dxs(t0, v0, v1);

		/* v0 = a0b0 */
		fp6_mul_dxq(v0, a[0], b[0][0]);

		/* v1 = a1b1 */
		fp6_mul_dxs(v1, a[1], b[1]);

		/* c1 = c1 - v0 - v1 */
		fp6_sub(c[1], t0, v0);
		fp6_sub(c[1], c[1], v1);

		/* c0 = v0 + v * v1 */
		fp6_mul_art(v1, v1);
		fp6_add(c[0], v0, v1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp6_free(v0);
		fp6_free(v1);
		fp6_free(t0);
	}
}

void fp12_sqr(fp12_t c, fp12_t a) {
	fp6_t t0, t1;

	fp6_null(t0);
	fp6_null(t1);

	TRY {
		fp6_new(t0);
		fp6_new(t1);

		fp6_add(t0, a[0], a[1]);
		fp6_mul_art(t1, a[1]);
		fp6_add(t1, a[0], t1);
		fp6_mul(t0, t0, t1);
		fp6_mul(c[1], a[0], a[1]);
		fp6_sub(c[0], t0, c[1]);
		fp6_mul_art(t1, c[1]);
		fp6_sub(c[0], c[0], t1);
		fp6_dbl(c[1], c[1]);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp6_free(t0);
		fp6_free(t1);
	}
}

void fp12_sqr_dbl(fp12_t c, fp12_t a) {
	fp2_t t0, t1, t2, t3, t4, t5, t6, t7, t8, t9;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t3);
	fp2_null(t4);
	fp2_null(t5);
	fp2_null(t6);
	fp2_null(t7);
	fp2_null(t8);
	fp2_null(t9);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);
		fp2_new(t4);
		fp2_new(t5);
		fp2_new(t6);
		fp2_new(t7);
		fp2_new(t8);
		fp2_new(t9);

		/* a0 = (a00, a11). */
		/* a1 = (a10, a02). */
		/* a2 = (a01, a12). */

		/* (t0,t1) = a0^2 */
		fp4_sqr(t0, t1, a[0][0], a[1][1]);

		/* (t2,t3) = 2 * a1 * a2 */
		fp4_mul(t2, t3, a[1][0], a[0][2], a[0][1], a[1][2]);
		fp2_dbl(t2, t2);
		fp2_dbl(t3, t3);

		/* (t4,t5) = a2^2. */
		fp4_sqr(t4, t5, a[0][1], a[1][2]);

		/* c2 = a0 + a2. */
		fp2_add(c[0][1], a[0][0], a[0][1]);
		fp2_add(c[1][2], a[1][1], a[1][2]);

		/* (t6, t7) = (a0 + a2 + a1)^2. */
		fp2_add(t6, c[0][1], a[1][0]);
		fp2_add(t7, c[1][2], a[0][2]);
		fp4_sqr(t6, t7, t6, t7);

		/* c2 = (a0 + a2 - a1)^2. */
		fp2_sub(c[0][1], c[0][1], a[1][0]);
		fp2_sub(c[1][2], c[1][2], a[0][2]);
		fp4_sqr(t8, t9, c[0][1], c[1][2]);

		/* (t0,t1) = 2 * (t0,t1). */
		/* (t2,t3) = 2 * (t2,t3). */
		/* (t4,t5) = 2 * (t4,t5). */
		fp2_dbl(t0, t0);
		fp2_dbl(t1, t1);
		fp2_dbl(t2, t2);
		fp2_dbl(t3, t3);
		fp2_dbl(t4, t4);
		fp2_dbl(t5, t5);

		/* c2 = (t8,t9) + (t6,t7) - (t0,t1) - (t4,t5). */
		fp2_add(c[0][1], t8, t6);
		fp2_add(c[1][2], t9, t7);
		fp2_sub(c[0][1], c[0][1], t0);
		fp2_sub(c[1][2], c[1][2], t1);
		fp2_sub(c[0][1], c[0][1], t4);
		fp2_sub(c[1][2], c[1][2], t5);

		/* b2 = (t6,t7) - (t8,t9) - (t2,t3) + (t4,t5) * E. */
		fp2_mul_nor(t5, t5);
		fp2_sub(c[1][0], t6, t8);
		fp2_sub(c[0][2], t7, t9);
		fp2_sub(c[1][0], c[1][0], t2);
		fp2_sub(c[0][2], c[0][2], t3);
		fp2_add(c[1][0], c[1][0], t5);
		fp2_add(c[0][2], c[0][2], t4);

		/* a0 = (t0,t1) - (t2,t3) * E. */
		fp2_mul_nor(t3, t3);
		fp2_add(c[0][0], t0, t3);
		fp2_add(c[1][1], t1, t2);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
		fp2_free(t3);
		fp2_free(t4);
		fp2_free(t5);
	}
}

void fp12_sqr_cyc(fp12_t c, fp12_t a) {
	fp2_t t0, t1, t2, t3;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);
	fp2_null(t3);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);

		/* Define z = sqrt(E). */

		/* Now a is seen as (t0,t1) + (t2,t3) * w + (t4,t5) * w^2. */

		/* (t0, t1) = (a00 + a11*z)^2. */
		/* (t2, t3) = (a10 + a02*z)^2. */
		/* (t4, t5) = (a01 + a12*z)^2. */

		/* (t0, t1) = (a00 + a11*z)^2. */
		fp4_sqr_uni(t0, t1, a[0][0], a[1][1]);

		fp2_sub(c[0][0], t0, a[0][0]);
		fp2_add(c[0][0], c[0][0], c[0][0]);
		fp2_add(c[0][0], t0, c[0][0]);

		fp2_add(c[1][1], t1, a[1][1]);
		fp2_add(c[1][1], c[1][1], c[1][1]);
		fp2_add(c[1][1], t1, c[1][1]);

		/* (t4, t5) = (a01 + a12*z)^2. */
		fp4_sqr_uni(t2, t0, a[0][1], a[1][2]);
		fp2_mul_nor(t3, t0);
		/* (t2, t3) = (a10 + a02*z)^2. */
		fp4_sqr_uni(t0, t1, a[1][0], a[0][2]);

		fp2_sub(c[0][1], t0, a[0][1]);
		fp2_add(c[0][1], c[0][1], c[0][1]);
		fp2_add(c[0][1], t0, c[0][1]);

		fp2_add(c[1][2], t1, a[1][2]);
		fp2_add(c[1][2], c[1][2], c[1][2]);
		fp2_add(c[1][2], t1, c[1][2]);

		fp2_sub(c[0][2], t2, a[0][2]);
		fp2_add(c[0][2], c[0][2], c[0][2]);
		fp2_add(c[0][2], t2, c[0][2]);

		fp2_add(c[1][0], t3, a[1][0]);
		fp2_add(c[1][0], c[1][0], c[1][0]);
		fp2_add(c[1][0], t3, c[1][0]);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
		fp2_free(t3);
	}
}

void fp12_inv(fp12_t c, fp12_t a) {
	fp6_t t0;
	fp6_t t1;

	fp6_null(t0);
	fp6_null(t1);

	TRY {
		fp6_new(t0);
		fp6_new(t1);

		fp6_sqr(t0, a[0]);
		fp6_sqr(t1, a[1]);
		fp6_mul_art(t1, t1);
		fp6_sub(t0, t0, t1);
		fp6_inv(t0, t0);

		fp6_mul(c[0], a[0], t0);
		fp6_neg(c[1], a[1]);
		fp6_mul(c[1], c[1], t0);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp6_free(t0);
		fp6_free(t1);
	}
}

void fp12_inv_cyc(fp12_t c, fp12_t a) {
	/* In this case, it's a simple conjugate. */
	fp6_copy(c[0], a[0]);
	fp6_neg(c[1], a[1]);
}

void fp12_frb(fp12_t c, fp12_t a) {
	/* t1 = conj(a00). */
	fp2_frb(c[0][0], a[0][0]);
	/* t2 = conj(a10). */
	fp2_frb(c[1][0], a[1][0]);
	/* t3 = conj(a01). */
	fp2_frb(c[0][1], a[0][1]);
	/* t4 = conj(a11). */
	fp2_frb(c[1][1], a[1][1]);
	/* t5 = conj(a02). */
	fp2_frb(c[0][2], a[0][2]);
	/* t6 = conj(a12). */
	fp2_frb(c[1][2], a[1][2]);

	fp2_mul_frb(c[1][0], c[1][0], 1);
	fp2_mul_frb(c[0][1], c[0][1], 2);
	fp2_mul_frb(c[1][1], c[1][1], 3);
	fp2_mul_frb(c[0][2], c[0][2], 4);
	fp2_mul_frb(c[1][2], c[1][2], 5);
}

void fp12_frb_sqr(fp12_t c, fp12_t a) {
	fp2_copy(c[0][0], a[0][0]);
	fp2_mul_frb_sqr(c[0][1], a[0][1], 2);
	fp2_mul_frb_sqr(c[0][2], a[0][2], 1);
	fp2_neg(c[0][2], c[0][2]);
	fp2_mul_frb_sqr(c[1][0], a[1][0], 1);
	fp2_mul_frb_sqr(c[1][1], a[1][1], 3);
	fp2_mul_frb_sqr(c[1][2], a[1][2], 2);
	fp2_neg(c[1][2], c[1][2]);
}

void fp12_exp(fp12_t c, fp12_t a, bn_t b) {
	fp12_t t;

	fp12_null(t);

	TRY {
		fp12_new(t);

		fp12_copy(t, a);

		for (int i = bn_bits(b) - 2; i >= 0; i--) {
			fp12_sqr(t, t);
			if (bn_test_bit(b, i)) {
				fp12_mul(t, t, a);
			}
		}
		fp12_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp12_free(t);
	}
}

void fp12_exp_cyc(fp12_t c, fp12_t a, bn_t b) {
	fp12_t t;

	fp12_null(t);

	TRY {
		fp12_new(t);

		fp12_copy(t, a);

		for (int i = bn_bits(b) - 2; i >= 0; i--) {
			fp12_sqr_cyc(t, t);
			if (bn_test_bit(b, i)) {
				fp12_mul(t, t, a);
			}
		}
		fp12_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp12_free(t);
	}
}
