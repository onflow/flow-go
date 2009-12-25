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
/* Public definitions                                                         */
/*============================================================================*/

void fp12_conj(fp12_t c, fp12_t a) {
	fp6_copy(c[0], a[0]);
	fp6_neg(c[1], a[1]);
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
		fp6_mul(t0, a[0], b[0]);
		fp6_mul(t1, a[1], b[1]);
		fp6_add(t2, b[0], b[1]);
		fp6_add(c[1], a[1], a[0]);
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

void fp12_mul_dexsp(fp12_t c, fp12_t a, fp12_t b) {
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
		fp6_mul_dexsp(t0, v0, v1);

		/* v0 = a0b0 */
		fp6_mul_dexqu(v0, a[0], b[0][0]);

		/* v1 = a1b1 */
		fp6_mul(v1, a[1], b[1]);

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

void fp12_sqr_uni(fp12_t c, fp12_t a) {
	fp6_t t0, t1;
	fp_t one;

	fp6_null(t0);
	fp6_null(t1);

	TRY {
		fp6_new(t0);
		fp6_new(t1);
		fp_new(one);

		/* (a0 + a1 * w)^2 = (2b^2v + 1) + ((a + b)^2 - b^2 - b^2v - 1) * w. */
		fp6_sqr(t0, a[1]);
		fp6_add(t1, a[0], a[1]);
		fp6_sqr(t1, t1);
		fp6_sub(t1, t1, t0);
		fp6_mul_art(c[0], t0);
		fp6_sub(c[1], t1, c[0]);
		fp_set_dig(one, 1);
		fp6_dbl(c[0], c[0]);
		fp_add_dig(c[0][0][0], c[0][0][0], 1);
		fp_sub(c[1][0][0], c[1][0][0], one);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp6_free(t0);
		fp6_free(t1);
		fp_free(one);
	}
}

void fp12_frb(fp12_t c, fp12_t a, fp12_t b) {
	fp12_t t, t0, t1;

	fp12_null(t);
	fp12_null(t0);
	fp12_null(t1);

	TRY {
		fp12_new(t);
		fp12_new(t0);
		fp12_new(t1);

		fp12_sqr(t, b);
		fp6_frb(t0[0], a[0], t[0]);
		fp6_frb(t1[0], a[1], t[0]);
		fp6_zero(t0[1]);
		fp6_zero(t1[1]);
		fp12_mul(t1, t1, b);
		fp12_add(c, t0, t1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp12_free(t);
		fp12_free(t0);
		fp12_free(t1);
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

void fp12_exp_uni(fp12_t c, fp12_t a, bn_t b) {
	fp12_t t;

	fp12_null(t);

	TRY {
		fp12_new(t);

		fp12_copy(t, a);

		for (int i = bn_bits(b) - 2; i >= 0; i--) {
			fp12_sqr_uni(t, t);
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
