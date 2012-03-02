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
 * Implementation of hashing to a prime elliptic curve over a quadratic
 * extension.
 *
 * @version $Id$
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

/**
 * Multiplies a point by the curve cofactor.
 *
 * @param[out] r			- the result.
 * @param[in] p				- the point to multiply.
 */
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

		fp_param_get_var(x);

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
	bn_t k;
	fp2_t t;
	unsigned char digest[MD_LEN];

	bn_null(k);
	fp2_null(t);

	TRY {
		bn_new(k);
		fp2_new(t);

		md_map(digest, msg, len);
		bn_read_bin(k, digest, MIN(FP_BYTES, MD_LEN));

		fp_prime_conv(p->x[0], k);
		fp_zero(p->x[1]);
		fp_set_dig(p->z[0], 1);
		fp_zero(p->z[1]);

		while (1) {
			ep2_rhs(t, p);

			if (fp2_srt(p->y, t)) {
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
		bn_free(k);
		fp2_free(t);
	}
}
