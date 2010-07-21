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
 * Implementation of hashing to a prime elliptic curve over a quadratic
 * extension.
 *
 * @version $Id: relic_pp_ep2.c 463 2010-07-13 21:12:13Z conradoplg $
 * @ingroup pp
 */

#include "relic_core.h"
#include "relic_md.h"
#include "relic_pp.h"
#include "relic_error.h"
#include "relic_conf.h"
#include "relic_fp_low.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

void ep2_mul_cof(ep2_t r, ep2_t p) {
	bn_t a, x;
	ep2_t t1;
	ep2_t t2;

	ep2_null(t1);
	ep2_null(t2);
	bn_null(a);
	bn_null(x);

	TRY {
		ep2_new(t1);
		ep2_new(t2);
		bn_new(a);
		bn_new(x);

		switch (fp_param_get()) {
			case BN_158:
				/* x = 4000000031. */
				bn_set_2b(x, 38);
				bn_add_dig(x, x, 0x31);
				break;
			case BN_254:
				/* x = -4080000000000001. */
				bn_set_2b(x, 62);
				bn_set_2b(a, 55);
				bn_add(x, x, a);
				bn_add_dig(x, x, 1);
				bn_neg(x, x);
				break;
			case BN_256:
				/* x = 6000000000001F2D. */
				bn_set_2b(x, 62);
				bn_set_2b(a, 61);
				bn_add(x, x, a);
				bn_set_dig(a, 0x1F);
				bn_lsh(a, a, 8);
				bn_add(x, x, a);
				bn_add_dig(x, x, 0x2D);
				break;
			default:
				/* TODO: generalize for non-BN curves */
				THROW(ERR_NO_CURVE);
		}

		bn_sqr(x, x);
		bn_mul_dig(a, x, 6);
		ep2_frb(t1, p);
		ep2_frb(t2, t1);
		ep2_sub(t1, t1, t2);
		ep2_norm(t1, t1);
		ep2_mul(r, p, a);
		ep2_frb(t2, r);
		ep2_add(r, r, t1);
		ep2_add(r, r, t2);
		ep2_norm(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep2_free(t1);
		ep2_free(t2);
		bn_new(a);
		bn_new(x);
	}
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ep2_map(ep2_t p, unsigned char *msg, int len) {
	fp2_t t0, t1;
	int bits, digits;
	unsigned char digest[MD_LEN];

	fp2_null(t0);
	fp2_null(t1);

	TRY {
		fp2_new(t0);
		fp2_new(t1);

		md_map(digest, msg, len);
		fp_set_dig(p->z[0], 1);
		fp_zero(p->z[1]);
		memcpy(p->x[0], digest, MIN(FP_BYTES, MD_LEN));
		fp_zero(p->x[1]);

		SPLIT(bits, digits, FP_BITS, FP_DIG_LOG);
		if (bits > 0) {
			dig_t mask = ((dig_t)1 << (dig_t)bits) - 1;
			p->x[0][FP_DIGS - 1] &= mask;
		}

		while (fp_cmp(p->x[0], fp_prime_get()) != CMP_LT) {
			fp_subn_low(p->x[0], p->x[0], fp_prime_get());
		}

		while (1) {
			/* t0 = x1^2. */
			fp2_sqr(t0, p->x);
			/* t1 = x1^3. */
			fp2_mul(t1, t0, p->x);

			/* t1 = x1^3 + a * x1 + b. */
			ep2_curve_get_a(t0);
			fp2_mul(t0, p->x, t0);
			fp2_add(t1, t1, t0);
			ep2_curve_get_b(t0);
			fp2_add(t1, t1, t0);

			if (fp2_srt(p->y, t1)) {
				p->norm = 1;
				break;
			}
			fp_add_dig(p->x[0], p->x[0], 1);
		}

		ep2_mul_cof(p, p);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(t0);
		fp2_free(t1);
	}
}
