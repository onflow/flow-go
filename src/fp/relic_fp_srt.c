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
 * Implementation of the square root function.
 *
 * @version $Id$
 * @ingroup bn
 */

#include <string.h>

#include "relic_core.h"
#include "relic_fp.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Extracts the square root of prime field element using the Tonelli-Shanks
 * algorithm. Computes c = sqrt(a). The other square root is the negation of c.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the prime field element.
 * @param[in] t1			- used as temporary (to save stack space).
 * @param[in] t2			- used as temporary (to save stack space).
 * @param[in] e				- used as temporary (to save stack space).
 * @return					- 1 if there is a square root, 0 otherwise.
 */
static int fp_srt_ts(fp_t c, fp_t a, fp_t t1, fp_t t2, bn_t e) {
	int f, r;
	bn_t s;
	fp_t t3;
	fp_t t4;

	bn_null(s);
	fp_null(t3);
	fp_null(t4);

	TRY {
		bn_new(s);
		fp_new(t3);
		fp_new(t4);

		//needs to check if square root exists
		e->used = FP_DIGS;
		dv_copy(e->dp, fp_prime_get(), FP_DIGS);
		bn_rsh(e, e, 1);
		fp_exp(t1, a, e);

		if (fp_cmp_dig(t1, 1) != CMP_EQ) {
			//does not exist
			r = 0;
		} else {
			r = 1;
			//Partition p - 1 = s * 2^f for odd s
			//t1 = s
			s->used = FP_DIGS;
			dv_copy(s->dp, fp_prime_get(), FP_DIGS);
			bn_sub_dig(s, s, 1);
			f = 0;
			while (bn_get_bit(s, 0) == 0) {
				bn_rsh(s, s, 1);
				f += 1;
			}
			//Find n, a quadratic non-residue mod p
			//t2 = n
			fp_set_dig(t2, 2);
			while (1) {
				fp_exp(t3, t2, e);
				if (fp_cmp_dig(t3, 1) != CMP_EQ) {
					break;
				}
				fp_add_dig(t2, t2, 1);
			}

			//b = a ^ s
			//t1 = b
			fp_exp(t3, a, s);
			//g = n ^ s
			//t2 = g
			fp_exp(t2, t2, s);
			//x = a ^ ((s + 1) / 2)
			//t3 = x
			bn_add_dig(s, s, 1);
			bn_rsh(s, s, 1);
			fp_exp(t1, a, s);
			//r = e

			while (1) {
				//t = b
				int m;
				fp_copy(t4, t3);
				for (m = 0; m < f; m++) {
					if (fp_cmp_dig(t4, 1) == CMP_EQ) {
						break;
					}
					fp_sqr(t4, t4);
				}
				if (m == 0) {
					break;
				}
				bn_set_2b(e, f - m - 1);
				fp_exp(t2, t2, e);
				fp_mul(t1, t1, t2);
				fp_sqr(t2, t2);
				fp_mul(t3, t3, t2);
				f = m;
			}
			fp_copy(c, t1);
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(s);
		fp_free(t3);
		fp_free(t4);
	}
	return r;
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int fp_srt(fp_t c, fp_t a) {
	bn_t e;
	fp_t t1;
	fp_t t2;
	int r = 0;

	bn_null(e);
	fp_null(t1);
	fp_null(t2);

	TRY {
		bn_new(e);
		fp_new(t1);
		fp_new(t2);

		if (fp_prime_get_mod8() != 3 && fp_prime_get_mod8() != 7) {
			r = fp_srt_ts(c, a, t1, t2, e);
		} else {
			e->used = FP_DIGS;
			dv_copy(e->dp, fp_prime_get(), FP_DIGS);
			bn_add_dig(e, e, 1);
			bn_rsh(e, e, 2);

			fp_exp(t1, a, e);
			fp_sqr(t2, t1);
			r = (fp_cmp(t2, a) == CMP_EQ);
			fp_copy(c, t1);
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(e);
		fp_free(t1);
		fp_free(t2);
	}
	return r;
}
