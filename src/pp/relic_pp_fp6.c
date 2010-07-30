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
 * Implementation of the sextic extension binary field arithmetic module.
 *
 * @version $Id$
 * @ingroup fp6
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_fp.h"
#include "relic_pp.h"
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
	/* 2 * (a0 + a1 * v + a2 * v^2) = 2 * a0 + 2 * a1 * v + 2 * a2 * v^2. */
	fp2_dbl(c[0], a[0]);
	fp2_dbl(c[1], a[1]);
	fp2_dbl(c[2], a[2]);
}

void fp6_mul(fp6_t c, fp6_t a, fp6_t b) {
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

		/* v0 = a0b0 */
		fp2_mul(v0, a[0], b[0]);

		/* v1 = a1b1 */
		fp2_mul(v1, a[1], b[1]);

		/* v2 = a2b2 */
		fp2_mul(v2, a[2], b[2]);

		/* t2 (c0) = v0 + E((a1 + a2)(b1 + b2) - v1 - v2) */
		fp2_add(t0, a[1], a[2]);
		fp2_add(t1, b[1], b[2]);
		fp2_mul(t2, t0, t1);
		fp2_sub(t2, t2, v1);
		fp2_sub(t2, t2, v2);
		fp2_mul_nor(t2, t2);
		fp2_add(t2, t2, v0);

		/* c1 = (a0 + a1)(b0 + b1) - v0 - v1 + Ev2 */
		fp2_add(t0, a[0], a[1]);
		fp2_add(t1, b[0], b[1]);
		fp2_mul(c[1], t0, t1);
		fp2_sub(c[1], c[1], v0);
		fp2_sub(c[1], c[1], v1);
		fp2_mul_nor(t0, v2);
		fp2_add(c[1], c[1], t0);

		/* c2 = (a0 + a2)(b0 + b2) - v0 + v1 - v2 */
		fp2_add(t0, a[0], a[2]);
		fp2_add(t1, b[0], b[2]);
		fp2_mul(c[2], t0, t1);
		fp2_sub(c[2], c[2], v0);
		fp2_add(c[2], c[2], v1);
		fp2_sub(c[2], c[2], v2);

		/* c0 = t2 */
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

		/* (a0 + a1 * v + a2 * v^2) * v = a2 + a0 * v + a1 * v^2 */
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

void fp6_sqr(fp6_t c, fp6_t a) {
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

		/* t1 = 2 * a0 * a1 */
		fp2_mul(t1, a[0], a[1]);
		fp2_dbl(t1, t1);

		/* t2 = (a0 - a1 + a2)^2 */
		fp2_sub(t2, a[0], a[1]);
		fp2_add(t2, t2, a[2]);
		fp2_sqr(t2, t2);

		/* t3 = 2 * a1 * a2 */
		fp2_mul(t3, a[1], a[2]);
		fp2_dbl(t3, t3);

		/* t4 = a2^2 */
		fp2_sqr(t4, a[2]);

		/* c0 = t0 + E * t3 */
		fp2_mul_nor(c[0], t3);
		fp2_add(c[0], c[0], t0);

		/* c1 = t1 + E * t4 */
		fp2_mul_nor(c[1], t4);
		fp2_add(c[1], c[1], t1);

		/* c2 = t1 + t2 + t3 - t0 - t4 */
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

		fp2_sqr(t0, a[0]);
		fp2_mul(v0, a[1], a[2]);
		fp2_mul_nor(v0, v0);
		fp2_sub(v0, t0, v0);

		fp2_sqr(t0, a[2]);
		fp2_mul_nor(t0, t0);
		fp2_mul(v1, a[0], a[1]);
		fp2_sub(v1, t0, v1);

		fp2_sqr(t0, a[1]);
		fp2_mul(v2, a[0], a[2]);
		fp2_sub(v2, t0, v2);

		fp2_mul(c[1], a[1], v2);
		fp2_mul_nor(c[1], c[1]);

		fp2_mul(c[0], a[0], v0);

		fp2_mul(c[2], a[2], v1);
		fp2_mul_nor(c[2], c[2]);

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
