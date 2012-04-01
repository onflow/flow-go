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
 * Implementation of exponentiation in extensions defined over prime fields.
 *
 * @version $Id$
 * @ingroup fpx
 */

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_fp.h"
#include "relic_fp_low.h"
#include "relic_pp_low.h"
#include "relic_pp.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

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
			for (i = 0; i < w; i++) {
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
		for (i = 0; i < w; i++) {
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
		fp12_frb(t, c, 2);
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
	int result;

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
