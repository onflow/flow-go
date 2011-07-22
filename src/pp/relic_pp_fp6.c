/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
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
 * Implementation of the sextic extension binary field arithmetic module.
 *
 * @version $Id$
 * @ingroup fp6
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_fp.h"
#include "relic_fp_low.h"
#include "relic_pp.h"
#include "relic_pp_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp6_copy(fp6_t c, fp6_t a) {
	fp2_copy(c[0], a[0]);
	fp2_copy(c[1], a[1]);
	fp2_copy(c[2], a[2]);
}

void fp6_neg(fp6_t c, fp6_t a) {
	fp2_neg(c[0], a[0]);
	fp2_neg(c[1], a[1]);
	fp2_neg(c[2], a[2]);
}

void fp6_zero(fp6_t a) {
	fp2_zero(a[0]);
	fp2_zero(a[1]);
	fp2_zero(a[2]);
}

int fp6_is_zero(fp6_t a) {
	return fp2_is_zero(a[0]) && fp2_is_zero(a[1]) && fp2_is_zero(a[2]);
}

void fp6_rand(fp6_t a) {
	fp2_rand(a[0]);
	fp2_rand(a[1]);
	fp2_rand(a[2]);
}

void fp6_print(fp6_t a) {
	fp2_print(a[0]);
	fp2_print(a[1]);
	fp2_print(a[2]);
}

int fp6_cmp(fp6_t a, fp6_t b) {
	return ((fp2_cmp(a[0], b[0]) == CMP_EQ) && (fp2_cmp(a[1], b[1]) == CMP_EQ)
			&& (fp2_cmp(a[2], b[2]) == CMP_EQ) ? CMP_EQ : CMP_NE);
}

void fp6_add(fp6_t c, fp6_t a, fp6_t b) {
	fp2_add(c[0], a[0], b[0]);
	fp2_add(c[1], a[1], b[1]);
	fp2_add(c[2], a[2], b[2]);
}

void fp6_sub(fp6_t c, fp6_t a, fp6_t b) {
	fp2_sub(c[0], a[0], b[0]);
	fp2_sub(c[1], a[1], b[1]);
	fp2_sub(c[2], a[2], b[2]);
}

void fp6_dbl(fp6_t c, fp6_t a) {
	/* 2 * (a_0 + a_1 * v + a_2 * v^2) = 2 * a_0 + 2 * a_1 * v + 2 * a_2 * v^2. */
	fp2_dbl(c[0], a[0]);
	fp2_dbl(c[1], a[1]);
	fp2_dbl(c[2], a[2]);
}

#if PP_EXT == BASIC || !defined(STRIP)

void fp6_mul_basic(fp6_t c, fp6_t a, fp6_t b) {
	fp2_t v0, v1, v2, t0, t1, t2;

	fp2_null(v0);
	fp2_null(v1);
	fp2_null(v2);
	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);

	TRY {
		fp2_new(v0);
		fp2_new(v1);
		fp2_new(v2);
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);

		/* v0 = a_0b_0 */
		fp2_mul(v0, a[0], b[0]);

		/* v1 = a_1b_1 */
		fp2_mul(v1, a[1], b[1]);

		/* v2 = a_2b_2 */
		fp2_mul(v2, a[2], b[2]);

		/* t2 (c_0) = v0 + E((a_1 + a_2)(b_1 + b_2) - v1 - v2) */
		fp2_add(t0, a[1], a[2]);
		fp2_add(t1, b[1], b[2]);
		fp2_mul(t2, t0, t1);
		fp2_sub(t2, t2, v1);
		fp2_sub(t2, t2, v2);
		fp2_mul_nor(t0, t2);
		fp2_add(t2, t0, v0);

		/* c_1 = (a_0 + a_1)(b_0 + b_1) - v0 - v1 + Ev2 */
		fp2_add(t0, a[0], a[1]);
		fp2_add(t1, b[0], b[1]);
		fp2_mul(c[1], t0, t1);
		fp2_sub(c[1], c[1], v0);
		fp2_sub(c[1], c[1], v1);
		fp2_mul_nor(t0, v2);
		fp2_add(c[1], c[1], t0);

		/* c_2 = (a_0 + a_2)(b_0 + b_2) - v0 + v1 - v2 */
		fp2_add(t0, a[0], a[2]);
		fp2_add(t1, b[0], b[2]);
		fp2_mul(c[2], t0, t1);
		fp2_sub(c[2], c[2], v0);
		fp2_add(c[2], c[2], v1);
		fp2_sub(c[2], c[2], v2);

		/* c_0 = t2 */
		fp2_copy(c[0], t2);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t2);
		fp2_free(t1);
		fp2_free(t0);
		fp2_free(v2);
		fp2_free(v1);
		fp2_free(v0);
	}
}

void fp6_sqr_basic(fp6_t c, fp6_t a) {
	fp2_t t0, t1, t2, t3, t4;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t3);
	fp2_null(t4);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);
		fp2_new(t4);

		/* t0 = a0^2 */
		fp2_sqr(t0, a[0]);

		/* t1 = 2 * a1 * a2 */
		fp2_mul(t1, a[1], a[2]);
		fp2_dbl(t1, t1);

		/* t2 = a2^2. */
		fp2_sqr(t2, a[2]);

		/* c2 = a0 + a2. */
		fp2_add(c[2], a[0], a[2]);

		/* t3 = (a0 + a2 + a1)^2. */
		fp2_add(t3, c[2], a[1]);
		fp2_sqr(t3, t3);

		/* c2 = (a0 + a2 - a1)^2. */
		fp2_sub(c[2], c[2], a[1]);
		fp2_sqr(c[2], c[2]);

		/* c2 = (c2 + t3)/2. */
		fp2_add(c[2], c[2], t3);
		fp_hlv(c[2][0], c[2][0]);
		fp_hlv(c[2][1], c[2][1]);

		/* t3 = t3 - c2 - t1. */
		fp2_sub(t3, t3, c[2]);
		fp2_sub(t3, t3, t1);

		/* c2 = c2 - t0 - t2. */
		fp2_sub(c[2], c[2], t0);
		fp2_sub(c[2], c[2], t2);

		/* c0 = t0 + t1 * E. */
		fp2_mul_nor(t4, t1);
		fp2_add(c[0], t0, t4);

		/* c1 = t3 + t2 * E. */
		fp2_mul_nor(t4, t2);
		fp2_add(c[1], t3, t4);
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

void fp6_sqr_basic2(fp6_t c, fp6_t a) {
	fp2_t t0, t1, t2, t3, t4;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t3);
	fp2_null(t4);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);
		fp2_new(t4);

		/* t0 = a_0^2 */
		fp2_sqr(t0, a[0]);

		/* t1 = 2 * a_0 * a_1 */
		fp2_mul(t1, a[0], a[1]);
		fp2_dbl(t1, t1);

		/* t2 = (a_0 - a_1 + a_2)^2 */
		fp2_sub(t2, a[0], a[1]);
		fp2_add(t2, t2, a[2]);
		fp2_sqr(t2, t2);

		/* t3 = 2 * a_1 * a_2 */
		fp2_mul(t3, a[1], a[2]);
		fp2_dbl(t3, t3);

		/* t4 = a_2^2 */
		fp2_sqr(t4, a[2]);

		/* c_0 = t0 + E * t3 */
		fp2_mul_nor(c[0], t3);
		fp2_add(c[0], c[0], t0);

		/* c_1 = t1 + E * t4 */
		fp2_mul_nor(c[1], t4);
		fp2_add(c[1], c[1], t1);

		/* c_2 = t1 + t2 + t3 - t0 - t4 */
		fp2_add(c[2], t1, t2);
		fp2_add(c[2], c[2], t3);
		fp2_sub(c[2], c[2], t0);
		fp2_sub(c[2], c[2], t4);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
		fp2_free(t3);
		fp2_free(t4);
	}
}

#endif

#if PP_EXT == LAZYR || !defined(STRIP)

void fp6_mul_unr(dv6_t c, fp6_t a, fp6_t b) {
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
		fp2_mulc_low(u2, a[2], b[2]);
        fp2_addd_low(c[0], u1, u2);
        fp2_addd_low(c[1], u0, u1);
		fp2_addd_low(c[2], u0, u2);
		fp2_addn_low(t0, a[1], a[2]);
		fp2_addn_low(t1, b[1], b[2]);
#else
		fp2_muln_low(u0, a[0], b[0]);
		fp2_muln_low(u1, a[1], b[1]);
		fp2_muln_low(u2, a[2], b[2]);
        fp2_addc_low(c[0], u1, u2);
        fp2_addc_low(c[1], u0, u1);
		fp2_addc_low(c[2], u0, u2);
		fp2_addm_low(t0, a[1], a[2]);
		fp2_addm_low(t1, b[1], b[2]);
#endif
		/* t2 (c_0) = v0 + E((a_1 + a_2)(b_1 + b_2) - v1 - v2) */
        fp2_muln_low(u3, t0, t1);
        fp2_subc_low(u3, u3, c[0]);
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
		fp2_subc_low(u3, u3, c[1]);
		fp2_nord_low(u0, u2);
		fp2_addc_low(c[1], u3, u0);

		/* c_2 = (a_0 + a_2)(b_0 + b_2) - v0 + v1 - v2 */
#ifdef FP_SPACE
		fp2_addn_low(t0, a[0], a[2]);
		fp2_addn_low(t1, b[0], b[2]);
#else
		fp2_addm_low(t0, a[0], a[2]);
		fp2_addm_low(t1, b[0], b[2]);
#endif
		fp2_muln_low(u3, t0, t1);
		fp2_subc_low(u3, u3, c[2]);
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

void fp6_mul_lazyr(fp6_t c, fp6_t a, fp6_t b) {
	dv6_t t;

	dv6_null(t);

	TRY {
		fp6_mul_unr(t, a, b);
		fp2_rdcn_low(c[0], t[0]);
		fp2_rdcn_low(c[1], t[1]);
		fp2_rdcn_low(c[2], t[2]);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		dv6_free(t);
	}
}

void fp6_sqr_lazyr(fp6_t c, fp6_t a) {
	dv2_t u0, u1, u2, u3, u4, u5;
	fp2_t t0, t1, t2, t3;

	fp2_null(t0);
	fp2_null(t0);
	fp2_null(t2);
	fp2_null(t3);

	TRY {
		fp2_new(t0);
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);

		/* u0 = a0^2 */
		fp2_sqrn_low(u0, a[0]);

		/* t1 = 2 * a1 * a2 */
		fp2_dbl(t0, a[1]);

#ifdef FP_SPACE
		fp2_mulc_low(u1, t0, a[2]);
#else
		fp2_muln_low(u1, t0, a[2]);
#endif

		/* u2 = a2^2. */
		fp2_sqrn_low(u2, a[2]);

		/* t4 = a0 + a2. */
		fp2_add(t3, a[0], a[2]);

		/* u3 = (a0 + a2 + a1)^2. */
		fp2_add(t2, t3, a[1]);
		fp2_sqrn_low(u3, t2);

		/* u4 = (a0 + a2 - a1)^2. */
		fp2_sub(t1, t3, a[1]);
		fp2_sqrn_low(u4, t1);

		/* u4 = (u4 + u3)/2. */
#ifdef FP_SPACE
		fp2_addd_low(u4, u4, u3);
#else
		fp2_addc_low(u4, u4, u3);
#endif
		fp_hlvd_low(u4[0], u4[0]);
		fp_hlvd_low(u4[1], u4[1]);

		/* u3 = u3 - u4 - u1. */
#ifdef FP_SPACE
		fp2_addd_low(u5, u1, u4);
#else
		fp2_addc_low(u5, u1, u4);
#endif
		fp2_subc_low(u3, u3, u5);

		/* c2 = u4 - u0 - u2. */
#ifdef FP_SPACE
		fp2_addd_low(u5, u0, u2);
#else
		fp2_addc_low(u5, u0, u2);
#endif
		fp2_subc_low(u4, u4, u5);
		fp2_rdcn_low(c[2], u4);

		/* c0 = u0 + u1 * E. */
		fp2_nord_low(u4, u1);
		fp2_addc_low(u0, u0, u4);
		fp2_rdcn_low(c[0], u0);

		/* c1 = u3 + u2 * E. */
		fp2_nord_low(u4, u2);
		fp2_addc_low(u3, u3, u4);
		fp2_rdcn_low(c[1], u3);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
		fp2_free(t3);
		fp2_free(t5);
	}
}

void fp6_sqr_lazyr2(fp6_t c, fp6_t a) {
	dv2_t u0, u1, u2, u3, u4;
	fp2_t t0, t1, t2;

	dv2_null(u0);
	dv2_null(u1);
	dv2_null(u2);
	dv2_null(u3);
	dv2_null(u4);
	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);

	TRY {
		dv2_new(u0);
		dv2_new(u1);
		dv2_new(u2);
		dv2_new(u3);
		dv2_new(u4);
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);

		/* u0 = a_0^2 */
		fp2_sqrn_low(u0, a[0]);

		/* u1 = 2 * a_0 * a_1 */
		fp2_dbl(t0, a[0]);

		/* u3 = 2 * a_1 * a_2 */
		fp2_dbl(t1, a[1]);

#ifdef FP_SPACE
		fp2_mulc_low(u1, t0, a[1]);
		fp2_mulc_low(u3, t1, a[2]);
#else
		fp2_muln_low(u1, t0, a[1]);
		fp2_muln_low(u3, t1, a[2]);
#endif
		/* u2 = (a_0 - a_1 + a_2)^2 */
		fp2_sub(t2, a[0], a[1]);
		fp2_add(t2, t2, a[2]);
		fp2_sqrn_low(u2, t2);

		/* c_0 = u0 + E * u3 */
		fp2_nord_low(u4, u3);
		fp2_addc_low(u4, u4, u0);
		fp2_rdcn_low(c[0], u4);

		/* t4 = a_2^2 */
		fp2_sqrn_low(u4, a[2]);

		/* c_2 = u1 + u2 + u3 - u0 - u4 */
#ifdef FP_SPACE
		fp2_addd_low(u0, u0, u4);
		fp2_addd_low(u3, u3, u1);
		fp2_addd_low(u3, u3, u2);
		fp2_subc_low(u3, u3, u0);
#else
		fp2_addc_low(u0, u0, u4);
		fp2_addc_low(u3, u3, u1);
		fp2_subc_low(u3, u3, u0);
		fp2_addc_low(u3, u3, u2);
#endif
		fp2_rdcn_low(c[2], u3);

		/* c_1 = u1 + E * u4 */
		fp2_nord_low(u2, u4);
		fp2_addc_low(u2, u2, u1);
		fp2_rdcn_low(c[1], u2);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		dv2_free(u0);
		dv2_free(u1);
		dv2_free(u2);
		dv2_free(u3);
		dv2_free(u4);
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
	}
}

#endif

void fp6_mul_dxs(fp6_t c, fp6_t a, fp6_t b) {
	fp2_t v0, v1, t0, t1, t2;

	fp2_null(v0);
	fp2_null(v1);
	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);

	TRY {
		fp2_new(v0);
		fp2_new(v1);
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);

		/* v0 = a0b0 */
		fp2_mul(v0, a[0], b[0]);

		/* v1 = a1b1 */
		fp2_mul(v1, a[1], b[1]);

		/* v2 = a2b2 = 0 */

		/* t2 (c0) = v0 + E((a1 + a2)(b1 + b2) - v1 - v2) */
		fp2_add(t0, a[1], a[2]);
		fp2_mul(t2, t0, b[1]);
		fp2_sub(t2, t2, v1);
		fp2_mul_nor(t2, t2);
		fp2_add(t2, t2, v0);

		/* c1 = (a0 + a1)(b0 + b1) - v0 - v1 + Ev2 */
		fp2_add(t0, a[0], a[1]);
		fp2_add(t1, b[0], b[1]);
		fp2_mul(c[1], t0, t1);
		fp2_sub(c[1], c[1], v0);
		fp2_sub(c[1], c[1], v1);

		/* c2 = (a0 + a2)(b0 + b2) - v0 + v1 - v2 */
		fp2_add(t0, a[0], a[2]);
		fp2_mul(c[2], t0, b[0]);
		fp2_sub(c[2], c[2], v0);
		fp2_add(c[2], c[2], v1);

		/* c0 = t2 */
		fp2_copy(c[0], t2);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(v0);
		fp2_free(v1);
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
	}
}

void fp6_mul_dxq(fp6_t c, fp6_t a, fp2_t b) {
	fp2_mul(c[0], a[0], b);
	fp2_mul(c[1], a[1], b);
	fp2_mul(c[2], a[2], b);
}

void fp6_mul_art(fp6_t c, fp6_t a) {
	fp2_t t0;

	fp2_null(t0);

	TRY {
		fp2_new(t0);

		/* (a_0 + a_1 * v + a_2 * v^2) * v = a_2 + a_0 * v + a_1 * v^2 */
		fp2_copy(t0, a[0]);
		fp2_mul_nor(c[0], a[2]);
		fp2_copy(c[2], a[1]);
		fp2_copy(c[1], t0);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
	}
}

void fp6_exp(fp6_t c, fp6_t a, bn_t b) {
	fp6_t t;

	fp6_null(t);

	TRY {
		fp6_new(t);

		fp6_copy(t, a);

		for (int i = bn_bits(b) - 2; i >= 0; i--) {
			fp6_sqr(t, t);
			if (bn_test_bit(b, i)) {
				fp6_mul(t, t, a);
			}
		}
		fp6_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp6_free(t);
	}
}

void fp6_frb(fp6_t c, fp6_t a) {
	fp2_frb(c[0], a[0]);
	fp2_frb(c[1], a[1]);
	fp2_frb(c[2], a[2]);

	fp2_mul_frb(c[1], c[1], 2);
	fp2_mul_frb(c[2], c[2], 4);
}

void fp6_frb_sqr(fp6_t c, fp6_t a) {
	fp2_copy(c[0], a[0]);
	fp2_mul_frb_sqr(c[1], a[1], 2);
	fp2_mul_frb_sqr(c[2], a[2], 1);
	fp2_neg(c[2], c[2]);
}

void fp6_inv(fp6_t c, fp6_t a) {
	fp2_t v0;
	fp2_t v1;
	fp2_t v2;
	fp2_t t0;

	fp2_null(v0);
	fp2_null(v1);
	fp2_null(v2);
	fp2_null(t0);

	TRY {
		fp2_new(v0);
		fp2_new(v1);
		fp2_new(v2);
		fp2_new(t0);

		/* v0 = a_0^2 - E * a_1 * a_2. */
		fp2_sqr(t0, a[0]);
		fp2_mul(v0, a[1], a[2]);
		fp2_mul_nor(v2, v0);
		fp2_sub(v0, t0, v2);

		/* v1 = E * a_2^2 - a_0 * a_1. */
		fp2_sqr(t0, a[2]);
		fp2_mul_nor(v2, t0);
		fp2_mul(v1, a[0], a[1]);
		fp2_sub(v1, v2, v1);

		/* v2 = a_1^2 - a_0 * a_2. */
		fp2_sqr(t0, a[1]);
		fp2_mul(v2, a[0], a[2]);
		fp2_sub(v2, t0, v2);

		fp2_mul(t0, a[1], v2);
		fp2_mul_nor(c[1], t0);

		fp2_mul(c[0], a[0], v0);

		fp2_mul(t0, a[2], v1);
		fp2_mul_nor(c[2], t0);

		fp2_add(t0, c[0], c[1]);
		fp2_add(t0, t0, c[2]);
		fp2_inv(t0, t0);

		fp2_mul(c[0], v0, t0);
		fp2_mul(c[1], v1, t0);
		fp2_mul(c[2], v2, t0);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(v0);
		fp2_free(v1);
		fp2_free(v2);
		fp2_free(t0);
	}
}
