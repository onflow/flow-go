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
 * Implementation of the dodecic extension prime field arithmetic module.
 *
 * @version $Id$
 * @ingroup fp12
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_pp.h"
#include "relic_pp_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if PP_EXT == BASIC || !defined(STRIP)

static void fp4_mul(fp2_t e, fp2_t f, fp2_t a, fp2_t b, fp2_t c, fp2_t d) {
	fp2_t t0, t1, t2;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);

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
		fp2_free(t2);
	}
}

#endif

#if PP_EXT == LAZYR || !defined(STRIP)

static void fp4_mul_unr(dv2_t e, dv2_t f, fp2_t a, fp2_t b, fp2_t c, fp2_t d) {
	fp2_t t0, t1;
	dv2_t u0, u1;

	fp2_null(t0);
	fp2_null(t1);
	dv2_null(u0);
	dv2_null(u1);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		dv2_new(u0);
		dv2_new(u1);

#ifdef FP_SPACE
		fp2_mulc_low(u0, a, c);
		fp2_mulc_low(u1, b, d);
		fp2_addn_low(t0, c, d);
		fp2_addn_low(t1, a, b);
#else
		fp2_muln_low(u0, a, c);
		fp2_muln_low(u1, b, d);
		fp2_addm_low(t0, c, d);
		fp2_addm_low(t1, a, b);
#endif
		fp2_muln_low(f, t1, t0);
		fp2_subc_low(f, f, u0);
		fp2_subc_low(f, f, u1);
		fp2_nord_low(e, u1);
		fp2_addc_low(e, e, u0);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		dv2_free(t1);
		dv2_free(u0);
		dv2_free(u1);
	}
}

#endif

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

#if PP_EXT == BASIC || !defined(STRIP)

void fp12_mul_basic(fp12_t c, fp12_t a, fp12_t b) {
	fp6_t t0, t1, t2;

	fp6_null(t0);
	fp6_null(t1);
	fp6_null(t2);

	TRY {
		fp6_new(t0);
		fp6_new(t1);
		fp6_new(t2);

		/* Karatsuba algorithm. */

		/* t0 = a_0 * b_0. */
		fp6_mul(t0, a[0], b[0]);
		/* t1 = a_1 * b_1. */
		fp6_mul(t1, a[1], b[1]);
		/* t2 = b_0 + b_1. */
		fp6_add(t2, b[0], b[1]);
		/* c_1 = a_0 + a_1. */
		fp6_add(c[1], a[0], a[1]);
		/* c_1 = (a_0 + a_1) * (b_0 + b_1) */
		fp6_mul(c[1], c[1], t2);
		fp6_sub(c[1], c[1], t0);
		fp6_sub(c[1], c[1], t1);
		/* c_0 = a_0b_0 + v * a_1b_1. */
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

void fp12_mul_dxs_basic(fp12_t c, fp12_t a, fp12_t b) {
#if EP_ADD == BASIC
	fp6_t v0, v1, t0;

	fp6_null(v0);
	fp6_null(v1);
	fp6_null(t0);

	TRY {
		fp6_new(v0);
		fp6_new(v1);
		fp6_new(t0);

		/* c_1 = (a0 + a1)(b0 + b1) */
		fp6_add(v0, a[0], a[1]);
		fp2_add(v1[0], b[0][0], b[1][0]);
		fp2_copy(v1[1], b[1][1]);
		fp6_mul_dxs(t0, v0, v1);

		/* v0 = a0b0 */
		fp6_mul_dxq(v0, a[0], b[0][0]);

		/* v1 = a1b1 */
		fp6_mul_dxs(v1, a[1], b[1]);

		/* c_1 = c_1 - v0 - v1 */
		fp6_sub(c[1], t0, v0);
		fp6_sub(c[1], c[1], v1);

		/* c_0 = v0 + v * v1 */
		fp6_mul_art(v1, v1);
		fp6_add(c[0], v0, v1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp6_free(v0);
		fp6_free(v1);
		fp6_free(t0);
	}
#elif EP_ADD == PROJC
	/* a0 = (a00, a11). */
	/* a1 = (a10, a02). */
	/* a2 = (a01, a12). */
	fp2_t t0, t1, t2, t3, t4, t5, t6, t7, t8, t9;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);
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

		/* (t0,t1) = a0 * b0. */
		fp4_mul(t0, t1, a[0][0], a[1][1], b[0][0], b[1][1]);

		/* (t2,t3) = a1 * b1. */
		fp2_mul(t4, a[0][2], b[0][2]);
		fp2_mul(t3, a[1][0], b[0][2]);
		fp2_mul_nor(t2, t4);

		/* (t4,t5) = a0 + a1. */
		fp2_add(t4, a[0][0], a[1][0]);
		fp2_add(t5, a[1][1], a[0][2]);

		/* (0,t6) = b0 + b1. */
		fp2_add(t6, b[1][1], b[0][2]);

		/* c_1 = (a0 + a1) * (b0 + b1) - (t0,t1) - (t2,t3). */
		fp4_mul(t7, t8, t4, t5, b[0][0], t6);
		fp2_sub(t7, t7, t0);
		fp2_sub(c[1][0], t7, t2);
		fp2_sub(t8, t8, t1);
		fp2_sub(c[0][2], t8, t3);

		/* (t6, t7) = (a1 + a2) * (b1 + b2) - (t2, t3). */
		fp2_mul(t9, a[1][2], b[0][2]);
		fp2_mul(t7, a[0][1], b[0][2]);
		fp2_mul_nor(t6, t9);

		/* (t4,t5) = a0 + a2. */
		fp2_add(t4, a[0][0], a[0][1]);
		fp2_add(t5, a[1][1], a[1][2]);

		/* (t4,t5) = (a0 + a2) * (b0 + b2). */
		fp4_mul(t4, t5, t4, t5, b[0][0], b[1][1]);

		/* (t2,t3) = (t2,t3) + (t4,t5) - (t0,t1). */
		fp2_add(t2, t2, t4);
		fp2_add(t3, t3, t5);
		fp2_sub(c[0][1], t2, t0);
		fp2_sub(c[1][2], t3, t1);

		fp2_mul_nor(t8, t7);
		fp2_add(c[0][0], t0, t8);
		fp2_add(c[1][1], t1, t6);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
		fp2_free(t3);
		fp2_free(t4);
		fp2_free(t5);
		fp2_free(t6);
		fp2_free(t7);
		fp2_free(t8);
		fp2_free(t9);
	}
#endif
}

void fp12_sqr_cyc_basic(fp12_t c, fp12_t a) {
	fp2_t t0, t1, t2, t3, t4, t5, t6;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);
	fp2_null(t3);
	fp2_null(t4);
	fp2_null(t5);
	fp2_null(t6);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);
		fp2_new(t4);
		fp2_new(t5);
		fp2_new(t6);

		/* Define z = sqrt(E) */

		/* Now a is seen as (t0,t1) + (t2,t3) * w + (t4,t5) * w^2 */

		/* (t0, t1) = (a00 + a11*z)^2. */
		fp2_sqr(t2, a[0][0]);
		fp2_sqr(t3, a[1][1]);
		fp2_add(t1, a[0][0], a[1][1]);

		fp2_mul_nor(t0, t3);
		fp2_add(t0, t0, t2);

		fp2_sqr(t1, t1);
		fp2_sub(t1, t1, t2);
		fp2_sub(t1, t1, t3);

		fp2_sub(c[0][0], t0, a[0][0]);
		fp2_add(c[0][0], c[0][0], c[0][0]);
		fp2_add(c[0][0], t0, c[0][0]);

		fp2_add(c[1][1], t1, a[1][1]);
		fp2_add(c[1][1], c[1][1], c[1][1]);
		fp2_add(c[1][1], t1, c[1][1]);

		fp2_sqr(t0, a[0][1]);
		fp2_sqr(t1, a[1][2]);
		fp2_add(t5, a[0][1], a[1][2]);
		fp2_sqr(t2, t5);

		fp2_add(t3, t0, t1);
		fp2_sub(t5, t2, t3);

		fp2_add(t6, a[1][0], a[0][2]);
		fp2_sqr(t3, t6);
		fp2_sqr(t2, a[1][0]);

		fp2_mul_nor(t6, t5);
		fp2_add(t5, t6, a[1][0]);
		fp2_dbl(t5, t5);
		fp2_add(c[1][0], t5, t6);

		fp2_mul_nor(t4, t1);
		fp2_add(t5, t0, t4);
		fp2_sub(t6, t5, a[0][2]);

		fp2_sqr(t1, a[0][2]);

		fp2_dbl(t6, t6);
		fp2_add(c[0][2], t6, t5);

		fp2_mul_nor(t4, t1);
		fp2_add(t5, t2, t4);
		fp2_sub(t6, t5, a[0][1]);
		fp2_dbl(t6, t6);
		fp2_add(c[0][1], t6, t5);

		fp2_add(t0, t2, t1);
		fp2_sub(t5, t3, t0);
		fp2_add(t6, t5, a[1][2]);
		fp2_dbl(t6, t6);
		fp2_add(c[1][2], t5, t6);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
		fp2_free(t3);
		fp2_free(t4);
		fp2_free(t5);
		fp2_free(t6);
	}
}

void fp12_sqr_pck_basic(fp12_t c, fp12_t a) {
	fp2_t t0, t1, t2, t3, t4, t5, t6;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);
	fp2_null(t3);
	fp2_null(t4);
	fp2_null(t5);
	fp2_null(t6);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);
		fp2_new(t4);
		fp2_new(t5);
		fp2_new(t6);

		fp2_sqr(t0, a[0][1]);
		fp2_sqr(t1, a[1][2]);
		fp2_add(t5, a[0][1], a[1][2]);
		fp2_sqr(t2, t5);

		fp2_add(t3, t0, t1);
		fp2_sub(t5, t2, t3);

		fp2_add(t6, a[1][0], a[0][2]);
		fp2_sqr(t3, t6);
		fp2_sqr(t2, a[1][0]);

		fp2_mul_nor(t6, t5);
		fp2_add(t5, t6, a[1][0]);
		fp2_dbl(t5, t5);
		fp2_add(c[1][0], t5, t6);

		fp2_mul_nor(t4, t1);
		fp2_add(t5, t0, t4);
		fp2_sub(t6, t5, a[0][2]);

		fp2_sqr(t1, a[0][2]);

		fp2_dbl(t6, t6);
		fp2_add(c[0][2], t6, t5);

		fp2_mul_nor(t4, t1);
		fp2_add(t5, t2, t4);
		fp2_sub(t6, t5, a[0][1]);
		fp2_dbl(t6, t6);
		fp2_add(c[0][1], t6, t5);

		fp2_add(t0, t2, t1);
		fp2_sub(t5, t3, t0);
		fp2_add(t6, t5, a[1][2]);
		fp2_dbl(t6, t6);
		fp2_add(c[1][2], t5, t6);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
		fp2_free(t3);
		fp2_free(t4);
		fp2_free(t5);
		fp2_free(t6);
	}
}

#endif

#if PP_EXT == LAZYR || !defined(STRIP)

void fp12_mul_lazyr(fp12_t c, fp12_t a, fp12_t b) {
	dv6_t u0, u1, u2, u3;
	fp6_t t0, t1;

	dv6_null(u0);
	dv6_null(u1);
	dv6_null(u2);
	dv6_null(u3);
	fp6_null(t0);
	fp6_null(t0);
	fp6_null(t1);

	TRY {
		dv6_new(u0);
		dv6_new(u1);
		dv6_new(u2);
		dv6_new(u3);
		fp6_new(t0);
		fp6_new(t1);

		/* Karatsuba algorithm. */

		/* u0 = a_0 * b_0. */
		fp6_mul_unr(u0, a[0], b[0]);
		/* u1 = a_1 * b_1. */
		fp6_mul_unr(u1, a[1], b[1]);
		/* t1 = a_0 + a_1. */
		fp6_add(t0, a[0], a[1]);
		/* t0 = b_0 + b_1. */
		fp6_add(t1, b[0], b[1]);
		/* u2 = (a_0 + a_1) * (b_0 + b_1) */
		fp6_mul_unr(u2, t0, t1);
		/* c_1 = u2 - a_0b_0 - a_1b_1. */
		for (int i = 0; i < 3; i++) {
			fp2_addc_low(u3[i], u0[i], u1[i]);
			fp2_subc_low(u2[i], u2[i], u3[i]);
			fp2_rdcn_low(c[1][i], u2[i]);
		}
		/* c_0 = a_0b_0 + v * a_1b_1. */
		fp2_nord_low(u2[0], u1[2]);
		dv_copy(u2[1][0], u1[0][0], 2 * FP_DIGS);
		dv_copy(u2[1][1], u1[0][1], 2 * FP_DIGS);
		dv_copy(u2[2][0], u1[1][0], 2 * FP_DIGS);
		dv_copy(u2[2][1], u1[1][1], 2 * FP_DIGS);
		for (int i = 0; i < 3; i++) {
			fp2_addc_low(u2[i], u0[i], u2[i]);
			fp2_rdcn_low(c[0][i], u2[i]);
		}
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		dv6_free(u0);
		dv6_free(u1);
		dv6_free(u2);
		dv6_free(u3);
		fp6_free(t0);
		fp6_free(t1);
	}
}

void fp12_mul_dxs_lazyr(fp12_t c, fp12_t a, fp12_t b) {
#if EP_ADD == BASIC
	fp6_t v0, v1, t0;

	fp6_null(v0);
	fp6_null(v1);
	fp6_null(t0);

	TRY {
		fp6_new(v0);
		fp6_new(v1);
		fp6_new(t0);

		/* c_1 = (a0 + a1)(b0 + b1) */
		fp6_add(v0, a[0], a[1]);
		fp2_add(v1[0], b[0][0], b[1][0]);
		fp2_copy(v1[1], b[1][1]);
		fp6_mul_dxs(t0, v0, v1);

		/* v0 = a0b0 */
		fp6_mul_dxq(v0, a[0], b[0][0]);

		/* v1 = a1b1 */
		fp6_mul_dxs(v1, a[1], b[1]);

		/* c_1 = c_1 - v0 - v1 */
		fp6_sub(c[1], t0, v0);
		fp6_sub(c[1], c[1], v1);

		/* c_0 = v0 + v * v1 */
		fp6_mul_art(v1, v1);
		fp6_add(c[0], v0, v1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp6_free(v0);
		fp6_free(v1);
		fp6_free(t0);
	}
#elif EP_ADD == PROJC
	/* a0 = (a00, a11). */
	/* a1 = (a10, a02). */
	/* a2 = (a01, a12). */
	fp2_t t0, t1, t2;
	dv2_t u0, u1, u2, u3, u4, u5, u6, u7;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);
	dv2_null(u0);
	dv2_null(u1);
	dv2_null(u2);
	dv2_null(u3);
	dv2_null(u4);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		dv2_new(u0);
		dv2_new(u1);
		dv2_new(u2);
		dv2_new(u3);
		dv2_new(u4);

		/* (t0,t1) = a0 * b0. */
		fp4_mul_unr(u0, u1, a[0][0], a[1][1], b[0][0], b[1][1]);

		/* (t2,t3) = a1 * b1. */
		fp2_muln_low(u4, a[0][2], b[0][2]);
		fp2_nord_low(u2, u4);
		fp2_muln_low(u3, a[1][0], b[0][2]);

		/* (t4,t5) = a0 + a1. */
		fp2_add(t0, a[0][0], a[1][0]);
		fp2_add(t1, a[1][1], a[0][2]);

		/* (0,t2) = b0 + b1. */
		fp2_add(t2, b[1][1], b[0][2]);

		/* c_1 = (a0 + a1) * (b0 + b1) - (t0,t1) - (t2,t3). */
		fp4_mul_unr(u4, u5, t0, t1, b[0][0], t2);
		fp2_subc_low(u4, u4, u0);
		fp2_subc_low(u4, u4, u2);
		fp2_subc_low(u5, u5, u1);
		fp2_subc_low(u5, u5, u3);

		fp2_rdcn_low(c[1][0], u4);
		fp2_rdcn_low(c[0][2], u5);

		/* (t6, t7) = (a1 + a2) * (b1 + b2) - (t2, t3). */
		fp2_muln_low(u4, a[1][2], b[0][2]);
		fp2_nord_low(u6, u4);
		fp2_muln_low(u7, a[0][1], b[0][2]);

		/* (t4,t5) = a0 + a2. */
		fp2_add(t0, a[0][0], a[0][1]);
		fp2_add(t1, a[1][1], a[1][2]);

		/* (t4,t5) = (a0 + a2) * (b0 + b2). */
		fp4_mul_unr(u4, u5, t0, t1, b[0][0], b[1][1]);

		/* (t2,t3) = (t2,t3) + (t4,t5) - (t0,t1). */
		fp2_addc_low(u2, u2, u4);
		fp2_addc_low(u3, u3, u5);
		fp2_subc_low(u2, u2, u0);
		fp2_subc_low(u3, u3, u1);
		fp2_rdcn_low(c[0][1], u2);
		fp2_rdcn_low(c[1][2], u3);

		fp2_nord_low(u5, u7);
		fp2_addc_low(u0, u0, u5);
		fp2_addc_low(u1, u1, u6);
		fp2_rdcn_low(c[0][0], u0);
		fp2_rdcn_low(c[1][1], u1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
		dv2_free(u0);
		dv2_free(u1);
		dv2_free(u2);
		dv2_free(u3);
		dv2_free(u4);
	}
#endif
}

void fp12_sqr_cyc_lazyr(fp12_t c, fp12_t a) {
	fp2_t t0, t1;
	dv2_t u0, u1, u2, u3, u4;

	fp2_null(t0);
	fp2_null(t1);
	dv2_null(u0);
	dv2_null(u1);
	dv2_null(u2);
	dv2_null(u3);
	dv2_null(u4);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		dv2_new(u0);
		dv2_new(u1);
		dv2_new(u2);
		dv2_new(u3);
		dv2_new(u4);

		fp2_sqrn_low(u2, a[0][0]);
		fp2_sqrn_low(u3, a[1][1]);
		fp2_add(t1, a[0][0], a[1][1]);

		fp2_nord_low(u0, u3);
		fp2_addc_low(u0, u0, u2);
		fp2_rdcn_low(t0, u0);

		fp2_sqrn_low(u1, t1);
		fp2_addd_low(u2, u2, u3);
		fp2_subc_low(u1, u1, u2);
		fp2_rdcn_low(t1, u1);

		fp2_sub(c[0][0], t0, a[0][0]);
		fp2_dbl(c[0][0], c[0][0]);
		fp2_add(c[0][0], t0, c[0][0]);

		fp2_add(c[1][1], t1, a[1][1]);
		fp2_dbl(c[1][1], c[1][1]);
		fp2_add(c[1][1], t1, c[1][1]);

		fp2_sqrn_low(u0, a[0][1]);
		fp2_sqrn_low(u1, a[1][2]);
		fp2_add(t0, a[0][1], a[1][2]);
		fp2_sqrn_low(u2, t0);

		fp2_addd_low(u3, u0, u1);
		fp2_subc_low(u3, u2, u3);
		fp2_rdcn_low(t0, u3);

		fp2_add(t1, a[1][0], a[0][2]);
		fp2_sqrn_low(u3, t1);
		fp2_sqrn_low(u2, a[1][0]);

		fp2_mul_nor(t1, t0);
		fp2_add(t0, t1, a[1][0]);
		fp2_dbl(t0, t0);
		fp2_add(c[1][0], t0, t1);

		fp2_nord_low(u4, u1);
		fp2_addc_low(u4, u0, u4);
		fp2_rdcn_low(t0, u4);
		fp2_sub(t1, t0, a[0][2]);

		fp2_sqrn_low(u1, a[0][2]);

		fp2_dbl(t1, t1);
		fp2_add(c[0][2], t1, t0);

		fp2_nord_low(u4, u1);
		fp2_addc_low(u4, u2, u4);
		fp2_rdcn_low(t0, u4);
		fp2_sub(t1, t0, a[0][1]);
		fp2_dbl(t1, t1);
		fp2_add(c[0][1], t1, t0);

		fp2_addd_low(u0, u2, u1);
		fp2_subc_low(u3, u3, u0);
		fp2_rdcn_low(t0, u3);
		fp2_add(t1, t0, a[1][2]);
		fp2_dbl(t1, t1);
		fp2_add(c[1][2], t0, t1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		dv2_free(u0);
		dv2_free(u1);
		dv2_free(u2);
		dv2_free(u3);
		dv2_free(u4);
	}
}

void fp12_sqr_pck_lazyr(fp12_t c, fp12_t a) {
	fp2_t t0, t1;
	dv2_t u0, u1, u2, u3, u4;

	fp2_null(t0);
	fp2_null(t1);
	dv2_null(u0);
	dv2_null(u1);
	dv2_null(u2);
	dv2_null(u3);
	dv2_null(u4);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		dv2_new(u0);
		dv2_new(u1);
		dv2_new(u2);
		dv2_new(u3);
		dv2_new(u4);

		fp2_sqrn_low(u0, a[0][1]);
		fp2_sqrn_low(u1, a[1][2]);
		fp2_add(t0, a[0][1], a[1][2]);
		fp2_sqrn_low(u2, t0);

		fp2_addc_low(u3, u0, u1);
		fp2_subc_low(u3, u2, u3);
		fp2_rdcn_low(t0, u3);

		fp2_add(t1, a[1][0], a[0][2]);
		fp2_sqrn_low(u3, t1);
		fp2_sqrn_low(u2, a[1][0]);

		fp2_mul_nor(t1, t0);
		fp2_add(t0, t1, a[1][0]);
		fp2_dbl(t0, t0);
		fp2_add(c[1][0], t0, t1);

		fp2_nord_low(u4, u1);
		fp2_addc_low(u4, u0, u4);
		fp2_rdcn_low(t0, u4);
		fp2_sub(t1, t0, a[0][2]);

		fp2_sqrn_low(u1, a[0][2]);

		fp2_dbl(t1, t1);
		fp2_add(c[0][2], t1, t0);

		fp2_nord_low(u4, u1);
		fp2_addc_low(u4, u2, u4);
		fp2_rdcn_low(t0, u4);
		fp2_sub(t1, t0, a[0][1]);
		fp2_dbl(t1, t1);
		fp2_add(c[0][1], t1, t0);

		fp2_addc_low(u0, u2, u1);
		fp2_subc_low(u3, u3, u0);
		fp2_rdcn_low(t0, u3);
		fp2_add(t1, t0, a[1][2]);
		fp2_dbl(t1, t1);
		fp2_add(c[1][2], t0, t1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		dv2_free(u0);
		dv2_free(u1);
		dv2_free(u2);
		dv2_free(u3);
		dv2_free(u4);
	}
}

#endif

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

void fp12_inv_uni(fp12_t c, fp12_t a) {
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

		if (fp12_test_cyc(a)) {
			fp12_exp_cyc(c, a, b);
		} else {
			fp12_copy(t, a);

			for (int i = bn_bits(b) - 2; i >= 0; i--) {
				fp12_sqr(t, t);
				if (bn_test_bit(b, i)) {
					fp12_mul(t, t, a);
				}
			}
			fp12_copy(c, t);
		}
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
	int i, j, k, w = bn_ham(b);

	fp12_null(t);

	if (w > (bn_bits(b) >> 3)) {
		TRY {
			fp12_new(t);

			fp12_copy(t, a);

			for (i = bn_bits(b) - 2; i >= 0; i--) {
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
	} else {
		fp12_t u[w];

		TRY {
			for (i = 0; i < w; i++) {
				fp12_null(u[i]);
				fp12_new(u[i]);
			}
			fp12_new(t);

			j = 0;
			fp12_copy(t, a);
			for (i = 1; i < bn_bits(b); i++) {
				fp12_sqr_pck(t, t);
				if (bn_test_bit(b, i)) {
					fp12_copy(u[j++], t);
				}
			}

			if (!bn_is_even(b)) {
				j = 0;
				k = w - 1;
			} else {
				j = 1;
				k = w;
			}

			fp12_back_cyc_sim(u, u, k);

			if (!bn_is_even(b)) {
				fp12_copy(c, a);
			} else {
				fp12_copy(c, u[0]);
			}

			for (i = j; i < k; i++) {
				fp12_mul(c, c, u[i]);
			}
		}
		CATCH_ANY {
			THROW(ERR_CAUGHT);
		}
		FINALLY {
			for (i = 0; i < w - 1; i++) {
				fp12_free(u[i]);
			}
			fp12_free(t);
		}
	}
}

void fp12_exp_cyc_sps(fp12_t c, fp12_t a, int *b, int len) {
	int i, j, k, w = len;
	fp12_t t, u[w];

	fp12_null(t);

	TRY {
		for (i = 0; i < w; i++) {
			fp12_null(u[i]);
			fp12_new(u[i]);
		}
		fp12_new(t);

		fp12_copy(t, a);
		if (b[0] == 0) {
			for (j = 0, i = 1; i < len; i++) {
				k = (b[i] < 0 ? -b[i] : b[i]);
				for (; j < k; j++) {
					fp12_sqr_pck(t, t);
				}
				if (b[i] < 0) {
					fp12_inv_uni(u[i - 1], t);
				} else {
					fp12_copy(u[i - 1], t);
				}
			}

			fp12_back_cyc_sim(u, u, w - 1);

			fp12_copy(c, a);
			for (i = 0; i < w - 1; i++) {
				fp12_mul(c, c, u[i]);
			}
		} else {
			for (j = 0, i = 0; i < len; i++) {
				k = (b[i] < 0 ? -b[i] : b[i]);
				for (; j < k; j++) {
					fp12_sqr_pck(t, t);
				}
				if (b[i] < 0) {
					fp12_inv_uni(u[i], t);
				} else {
					fp12_copy(u[i], t);
				}
			}

			fp12_back_cyc_sim(u, u, w);

			fp12_copy(c, u[0]);
			for (i = 1; i < w; i++) {
				fp12_mul(c, c, u[i]);
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		for (i = 0; i < w - 1; i++) {
			fp12_free(u[i]);
		}
		fp12_free(t);
	}
}

void fp12_conv_uni(fp12_t c, fp12_t a) {
	fp12_t t;

	fp12_null(t);

	TRY {
		fp12_new(t);

		/* t = a^{-1}. */
		fp12_inv(t, a);
		/* c = a^(p^6). */
		fp12_inv_uni(c, a);
		/* c = a^(p^6 - 1). */
		fp12_mul(c, c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp12_free(t);
	}
}

void fp12_conv_cyc(fp12_t c, fp12_t a) {
	fp12_t t;

	fp12_null(t);

	TRY {
		fp12_new(t);

		/* First, compute c = a^(p^6 - 1). */
		/* t = a^{-1}. */
		fp12_inv(t, a);
		/* c = a^(p^6). */
		fp12_inv_uni(c, a);
		/* c = a^(p^6 - 1). */
		fp12_mul(c, c, t);

		/* Second, compute c^(p^2 + 1). */
		/* t = c^(p^2). */
		fp12_frb_sqr(t, c);
		/* c = c^(p^2 + 1). */
		fp12_mul(c, c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp12_free(t);
	}
}

int fp12_test_cyc(fp12_t a) {
	fp12_t t;
	int result = 0;

	fp12_null(t);

	TRY {
		fp12_new(t);

		fp12_back_cyc(t, a);
		result = (fp12_cmp(t, a) == CMP_EQ);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp12_free(t);
	}

	return result;
}

void fp12_back_cyc(fp12_t c, fp12_t a) {
	fp2_t t0, t1, t2;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);

		/* t0 = g4^2. */
		fp2_sqr(t0, a[0][1]);
		/* t1 = 3 * g4^2 - 2 * g3. */
		fp2_sub(t1, t0, a[0][2]);
		fp2_dbl(t1, t1);
		fp2_add(t1, t1, t0);
		/* t0 = E * g5^2 + t1. */
		fp2_sqr(t2, a[1][2]);
		fp2_mul_nor(t0, t2);
		fp2_add(t0, t0, t1);
		/* t1 = 1/(4 * g2). */
		fp2_dbl(t1, a[1][0]);
		fp2_dbl(t1, t1);
		fp2_inv(t1, t1);
		/* c_1 = g1. */
		fp2_mul(c[1][1], t0, t1);

		/* t1 = g3 * g4. */
		fp2_mul(t1, a[0][2], a[0][1]);
		/* t2 = 2 * g1^2 - 3 * g3 * g4. */
		fp2_sqr(t2, c[1][1]);
		fp2_sub(t2, t2, t1);
		fp2_dbl(t2, t2);
		fp2_sub(t2, t2, t1);
		/* t1 = g2 * g5. */
		fp2_mul(t1, a[1][0], a[1][2]);
		/* c_0 = E * (2 * g1^2 + g2 * g5 - 3 * g3 * g4) + 1. */
		fp2_add(t2, t2, t1);
		fp2_mul_nor(c[0][0], t2);
		fp_add_dig(c[0][0][0], c[0][0][0], 1);

		fp2_copy(c[0][1], a[0][1]);
		fp2_copy(c[0][2], a[0][2]);
		fp2_copy(c[1][0], a[1][0]);
		fp2_copy(c[1][2], a[1][2]);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
	}
}

void fp12_back_cyc_sim(fp12_t c[], fp12_t a[], int n) {
	fp2_t t0[n], t1[n], t2[n];

	for (int i = 0; i < n; i++) {
		fp2_null(t0[i]);
		fp2_null(t1[i]);
		fp2_null(t2[i]);
	}

	TRY {
		for (int i = 0; i < n; i++) {
			fp2_new(t0[i]);
			fp2_new(t1[i]);
			fp2_new(t2[i]);
		}

		for (int i = 0; i < n; i++) {
			/* t0 = g4^2. */
			fp2_sqr(t0[i], a[i][0][1]);
			/* t1 = 3 * g4^2 - 2 * g3. */
			fp2_sub(t1[i], t0[i], a[i][0][2]);
			fp2_dbl(t1[i], t1[i]);
			fp2_add(t1[i], t1[i], t0[i]);
			/* t0 = E * g5^2 + t1. */
			fp2_sqr(t2[i], a[i][1][2]);
			fp2_mul_nor(t0[i], t2[i]);
			fp2_add(t0[i], t0[i], t1[i]);
			/* t1 = (4 * g2). */
			fp2_dbl(t1[i], a[i][1][0]);
			fp2_dbl(t1[i], t1[i]);
		}

		/* t1 = 1 / t1. */
		fp2_inv_sim(t1, t1, n);

		for (int i = 0; i < n; i++) {
			/* t0 = g1. */
			fp2_mul(c[i][1][1], t0[i], t1[i]);

			/* t1 = g3 * g4. */
			fp2_mul(t1[i], a[i][0][2], a[i][0][1]);
			/* t2 = 2 * g1^2 - 3 * g3 * g4. */
			fp2_sqr(t2[i], c[i][1][1]);
			fp2_sub(t2[i], t2[i], t1[i]);
			fp2_dbl(t2[i], t2[i]);
			fp2_sub(t2[i], t2[i], t1[i]);
			/* t1 = g2 * g5. */
			fp2_mul(t1[i], a[i][1][0], a[i][1][2]);
			/* t2 = E * (2 * g1^2 + g2 * g5 - 3 * g3 * g4) + 1. */
			fp2_add(t2[i], t2[i], t1[i]);
			fp2_mul_nor(c[i][0][0], t2[i]);
			fp_add_dig(c[i][0][0][0], c[i][0][0][0], 1);

			fp2_copy(c[i][0][1], a[i][0][1]);
			fp2_copy(c[i][0][2], a[i][0][2]);
			fp2_copy(c[i][1][0], a[i][1][0]);
			fp2_copy(c[i][1][2], a[i][1][2]);
		}
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		for (int i = 0; i < n; i++) {
			fp2_free(t0[i]);
			fp2_free(t1[i]);
			fp2_free(t2[i]);
		}
	}
}
