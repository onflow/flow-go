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
 * Implementation of the Boneh-Lynn-Schacham short signature protocol.
 *
 * @version $Id$
 * @ingroup cp
 */

#include "relic.h"
#include "relic_test.h"
#include "relic_bench.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void cp_bls_gen(bn_t d, g2_t q) {
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		g2_get_ord(n);

		do {
			bn_rand(d, BN_POS, 2 * pc_param_level());
			bn_mod(d, d, n);
		} while (bn_is_zero(d));

		g2_mul_gen(q, d);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

void cp_bls_sig(g1_t s, unsigned char *msg, int len, bn_t d) {
	g1_t p;

	g1_null(p);

	TRY {
		g1_new(p);
		g1_map(p, msg, len);
		g1_mul(s, p, d);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		g1_free(p);
	}
}

int cp_bls_ver(g1_t s, unsigned char *msg, int len, g2_t q) {
	g1_t p;
	g2_t g;
	gt_st e1, e2;
	int result = 0;

	g1_null(p);
	g2_null(g);

	TRY {
		g1_new(p);
		g2_new(g);

		g2_get_gen(g);

		g1_map(p, msg, len);
		pc_map(e1, p, q);
		pc_map(e2, s, g);

		/*
		 * This is currently the only possible way to do securely verify
		 * signatures in the PC module: use automatic allocation combined
		 * with constant-time comparison.
		 */
		if (dv_cmp_const((dig_t *)e1, (dig_t *)e2,
						sizeof(gt_st) / sizeof(dig_t)) == CMP_EQ) {
			result = 1;
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		g1_null(p);
		g2_null(g);
	}
	return result;
}
