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
 * Implementation of multiplication in a dodecic extension of a prime field.
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
		fp2_mul_nor(t2, t1);
		fp2_add(e, t0, t2);
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

void fp6_mul_dxs_unr_lazyr(dv6_t c, fp6_t a, fp6_t b) {
	dv2_t u0, u1, u2, u3;
	fp2_t t0, t1;

	dv2_null(u0);
	dv2_null(u1);
	dv2_null(u2);
	dv2_null(u3);
	fp2_null(t0);
	fp2_null(t1);

	TRY {
		dv2_new(u0);
		dv2_new(u1);
		dv2_new(u2);
		dv2_new(u3);
		fp2_new(t0);
		fp2_new(t1);

		/* v0 = a_0b_0, v1 = a_1b_1, v2 = a_2b_2,
		 * t0 = a_1 + a_2, t1 = b_1 + b_2,
		 * u4 = u1 + u2, u5 = u0 + u1, u6 = u0 + u2 */
#ifdef FP_SPACE
		fp2_mulc_low(u0, a[0], b[0]);
		fp2_mulc_low(u1, a[1], b[1]);
		fp2_addd_low(c[1], u0, u1);
		fp2_addn_low(t0, a[1], a[2]);
#else
		fp2_muln_low(u0, a[0], b[0]);
		fp2_muln_low(u1, a[1], b[1]);
		fp2_addc_low(c[1], u0, u1);
		fp2_addm_low(t0, a[1], a[2]);
#endif
		/* t2 (c_0) = v0 + E((a_1 + a_2)(b_1 + b_2) - v1 - v2) */
		fp2_muln_low(u3, t0, b[1]);
		fp2_subc_low(u3, u3, u1);
		fp2_nord_low(c[0], u3);
		fp2_addc_low(c[0], c[0], u0);

		/* c_1 = (a_0 + a_1)(b_0 + b_1) - v0 - v1 + Ev2 */
#ifdef FP_SPACE
		fp2_addn_low(t0, a[0], a[1]);
		fp2_addn_low(t1, b[0], b[1]);
#else
		fp2_addm_low(t0, a[0], a[1]);
		fp2_addm_low(t1, b[0], b[1]);
#endif
		fp2_muln_low(u3, t0, t1);
		fp2_subc_low(c[1], u3, c[1]);

		/* c_2 = (a_0 + a_2)(b_0 + b_2) - v0 + v1 - v2 */
#ifdef FP_SPACE
		fp2_addn_low(t0, a[0], a[2]);
#else
		fp2_addm_low(t0, a[0], a[2]);
#endif
		fp2_muln_low(u3, t0, b[0]);
		fp2_subc_low(u3, u3, u0);
		fp2_addc_low(c[2], u3, u1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		dv2_free(u0);
		dv2_free(u1);
		dv2_free(u2);
		dv2_free(u3);
		fp2_free(t0);
		fp2_free(t1);
	}
}

#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

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

void fp12_mul_basic2(fp12_t c, fp12_t a, fp12_t b) {
	/* a0 = (a00, a11). */
	/* a1 = (a10, a02). */
	/* a2 = (a01, a12). */
	fp2_t t0, t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11;

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
		fp4_mul(t2, t3, a[1][0], a[0][2], b[1][0], b[0][2]);

		/* (t4,t5) = a0 + a1. */
		fp2_add(t4, a[0][0], a[1][0]);
		fp2_add(t5, a[1][1], a[0][2]);

		/* (t6,t7) = b0 + b1. */
		fp2_add(t6, b[0][0], b[1][0]);
		fp2_add(t7, b[1][1], b[0][2]);

		/* (t8,t9) = (a0 + a1) * (b0 + b1) - (t0,t1) - (t2,t3). */
		fp4_mul(t8, t9, t4, t5, t6, t7);
		fp2_sub(t8, t8, t0);
		fp2_sub(t8, t8, t2);
		fp2_sub(t9, t9, t1);
		fp2_sub(t9, t9, t3);

		/* (t10, t11) = (a1 + a2) * (b1 + b2) - (t2, t3). */
		fp2_add(t4, a[1][0], a[0][1]);
		fp2_add(t5, a[0][2], a[1][2]);
		fp2_add(t6, b[1][0], b[0][1]);
		fp2_add(t7, b[0][2], b[1][2]);
		fp4_mul(t10, t11, t4, t5, t6, t7);
		fp2_sub(t10, t10, t2);
		fp2_sub(t11, t11, t3);

		/* (t4,t5) = a0 + a2. */
		fp2_add(t4, a[0][0], a[0][1]);
		fp2_add(t5, a[1][1], a[1][2]);
		fp2_add(t6, b[0][0], b[0][1]);
		fp2_add(t7, b[1][1], b[1][2]);

		/* (t4,t5) = (a0 + a2) * (b0 + b2). */
		fp4_mul(t4, t5, t4, t5, t6, t7);

		/* (t2,t3) = (t2,t3) + (t4,t5) - (t0,t1). */
		fp2_add(t2, t2, t4);
		fp2_add(t3, t3, t5);
		fp2_sub(t2, t2, t0);	//OK
		fp2_sub(t3, t3, t1);	//OK

		fp4_mul(t6, t7, a[0][1], a[1][2], b[0][1], b[1][2]);
		fp2_sub(t2, t2, t6);
		fp2_sub(t3, t3, t7);
		fp2_sub(t10, t10, t6);
		fp2_sub(t11, t11, t7);

		fp2_mul_nor(t11, t11);
		fp2_add(c[0][0], t0, t11);
		fp2_add(c[1][1], t1, t10);

		fp2_copy(c[0][1], t2);
		fp2_copy(c[1][2], t3);

		fp2_mul_nor(t7, t7);
		fp2_add(c[1][0], t8, t7);	//OK
		fp2_add(c[0][2], t9, t6);	//OK
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
}

void fp12_mul_dxs_basic(fp12_t c, fp12_t a, fp12_t b) {
	fp6_t t0, t1, t2;

	fp6_null(t0);
	fp6_null(t1);
	fp6_null(t2);

	TRY {
		fp6_new(t0);
		fp6_new(t1);
		fp6_new(t2);

		if (ep2_curve_is_twist() == EP_DTYPE) {
#if EP_ADD == BASIC
			/* t0 = a_0 * b_0 */
			fp_mul(t0[0][0], a[0][0][0], b[0][0][0]);
			fp_mul(t0[0][1], a[0][0][1], b[0][0][0]);
			fp_mul(t0[1][0], a[0][1][0], b[0][0][0]);
			fp_mul(t0[1][1], a[0][1][1], b[0][0][0]);
			fp_mul(t0[2][0], a[0][2][0], b[0][0][0]);
			fp_mul(t0[2][1], a[0][2][1], b[0][0][0]);
			/* t2 = b_0 + b_1. */
			fp_add(t2[0][0], b[0][0][0], b[1][0][0]);
			fp_copy(t2[0][1], b[1][0][1]);
			fp2_copy(t2[1], b[1][1]);
#elif EP_ADD == PROJC
			/* t0 = a_0 * b_0 */
			fp2_mul(t0[0], a[0][0], b[0][0]);
			fp2_mul(t0[1], a[0][1], b[0][0]);
			fp2_mul(t0[2], a[0][2], b[0][0]);
			/* t2 = b_0 + b_1. */
			fp2_add(t2[0], b[0][0], b[1][0]);
			fp2_copy(t2[1], b[1][1]);
#endif
			/* t1 = a_1 * b_1. */
			fp6_mul_dxs(t1, a[1], b[1]);
		} else {
			/* t0 = a_0 * b_0. */
			fp6_mul_dxs(t0, a[0], b[0]);
#if EP_ADD == BASIC
			/* t1 = a_1 * b_1. */
			fp_mul(t1[0][0], a[1][2][0], b[1][1][0]);
			fp_mul(t1[0][1], a[1][2][1], b[1][1][0]);
			fp2_mul_nor(t1[0], t1[0]);
			fp_mul(t1[1][0], a[1][0][0], b[1][1][0]);
			fp_mul(t1[1][1], a[1][0][1], b[1][1][0]);
			fp_mul(t1[2][0], a[1][1][0], b[1][1][0]);
			fp_mul(t1[2][1], a[1][1][1], b[1][1][0]);
			/* t2 = b_0 + b_1. */
			fp2_copy(t2[0], b[0][0]);
			fp_add(t2[1][0], b[0][1][0], b[1][1][0]);
			fp_copy(t2[1][1], b[0][1][1]);
#elif EP_ADD == PROJC
			/* t1 = a_1 * b_1. */
			fp2_mul(t1[0], a[1][2], b[1][1]);
			fp2_mul_nor(t1[0], t1[0]);
			fp2_mul(t1[1], a[1][0], b[1][1]);
			fp2_mul(t1[2], a[1][1], b[1][1]);
			/* t2 = b_0 + b_1. */
			fp2_copy(t2[0], b[0][0]);
			fp2_add(t2[1], b[0][1], b[1][1]);
#endif
		}
		/* c_1 = a_0 + a_1. */
		fp6_add(c[1], a[0], a[1]);
		/* c_1 = (a_0 + a_1) * (b_0 + b_1) - a_0 * b_0 - a_1 * b_1. */
		fp6_mul_dxs(c[1], c[1], t2);
		fp6_sub(c[1], c[1], t0);
		fp6_sub(c[1], c[1], t1);
		/* c_0 = a_0 * b_0 + v * a_1 * b_1. */
		fp6_mul_art(t1, t1);
		fp6_add(c[0], t0, t1);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp6_free(t0);
		fp6_free(t1);
		fp6_free(t2);
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
	fp6_t t0;
	dv6_t u0, u1, u2;

	fp6_null(t0);
	dv6_null(u0);
	dv6_null(u1);
	dv6_null(u2);

	TRY {
		fp6_new(t0);
		dv6_new(u0);
		dv6_new(u1);
		dv6_new(u2);

		if (ep2_curve_is_twist() == EP_DTYPE) {
#if EP_ADD == BASIC
			/* t0 = a_0 * b_0. */
			fp_muln_low(u0[0][0], a[0][0][0], b[0][0][0]);
			fp_muln_low(u0[0][1], a[0][0][1], b[0][0][0]);
			fp_muln_low(u0[1][0], a[0][1][0], b[0][0][0]);
			fp_muln_low(u0[1][1], a[0][1][1], b[0][0][0]);
			fp_muln_low(u0[2][0], a[0][2][0], b[0][0][0]);
			fp_muln_low(u0[2][1], a[0][2][1], b[0][0][0]);
			/* t2 = b_0 + b_1. */
			fp_add(t0[0][0], b[0][0][0], b[1][0][0]);
			fp_copy(t0[0][1], b[1][0][1]);
			fp2_copy(t0[1], b[1][1]);
#elif EP_ADD == PROJC
			/* t0 = a_0 * b_0. */
			fp2_muln_low(u0[0], a[0][0], b[0][0]);
			fp2_muln_low(u0[1], a[0][1], b[0][0]);
			fp2_muln_low(u0[2], a[0][2], b[0][0]);
			/* t2 = b_0 + b_1. */
			fp2_add(t0[0], b[0][0], b[1][0]);
			fp2_copy(t0[1], b[1][1]);
#endif
			/* t1 = a_1 * b_1. */
			fp6_mul_dxs_unr_lazyr(u1, a[1], b[1]);
		} else {
			/* t0 = a_0 * b_0. */
			fp6_mul_dxs_unr_lazyr(u0, a[0], b[0]);
#if EP_ADD == BASIC
			/* t0 = a_0 * b_0. */
			fp_muln_low(u1[1][0], a[1][2][0], b[1][1][0]);
			fp_muln_low(u1[1][1], a[1][2][1], b[1][1][0]);
			fp2_nord_low(u1[0], u1[1]);
			fp_muln_low(u1[1][0], a[1][0][0], b[1][1][0]);
			fp_muln_low(u1[1][1], a[1][0][1], b[1][1][0]);
			fp_muln_low(u1[2][0], a[1][1][0], b[1][1][0]);
			fp_muln_low(u1[2][1], a[1][1][1], b[1][1][0]);
			/* t2 = b_0 + b_1. */
			fp2_copy(t0[0], b[0][0]);
			fp_add(t0[1][0], b[0][1][0], b[1][1][0]);
			fp_copy(t0[1][1], b[0][1][1]);
#elif EP_ADD == PROJC
			/* t1 = a_1 * b_1. */
			fp2_muln_low(u1[1], a[1][2], b[1][1]);
			fp2_nord_low(u1[0], u1[1]);
			fp2_muln_low(u1[1], a[1][0], b[1][1]);
			fp2_muln_low(u1[2], a[1][1], b[1][1]);
			/* t2 = b_0 + b_1. */
			fp2_copy(t0[0], b[0][0]);
			fp2_add(t0[1], b[0][1], b[1][1]);
#endif
		}
		/* c_1 = a_0 + a_1. */
		fp6_add(c[1], a[0], a[1]);
		/* c_1 = (a_0 + a_1) * (b_0 + b_1) */
		fp6_mul_dxs_unr_lazyr(u2, c[1], t0);
		for (int i = 0; i < 3; i++) {
			fp2_subc_low(u2[i], u2[i], u0[i]);
			fp2_subc_low(u2[i], u2[i], u1[i]);
		}
		fp2_rdcn_low(c[1][0], u2[0]);
		fp2_rdcn_low(c[1][1], u2[1]);
		fp2_rdcn_low(c[1][2], u2[2]);

		fp2_nord_low(u2[0], u1[2]);
		fp2_addc_low(u0[0], u0[0], u2[0]);
		fp2_addc_low(u0[1], u0[1], u1[0]);
		fp2_addc_low(u0[2], u0[2], u1[1]);
		/* c_0 = a_0b_0 + v * a_1b_1. */
		fp2_rdcn_low(c[0][0], u0[0]);
		fp2_rdcn_low(c[0][1], u0[1]);
		fp2_rdcn_low(c[0][2], u0[2]);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp6_free(t0);
		dv6_free(u0);
		dv6_free(u1);
		dv6_free(u2);
	}
}

#endif
