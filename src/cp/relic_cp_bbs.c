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

void cp_bbs_gen(bn_t d, g2_t q, gt_t z) {
	bn_t n;
	g1_t g;

	bn_null(n);
	g1_null(g);

	TRY {
		bn_new(n);
		g1_new(g);

		g1_get_gen(g);
		g2_get_gen(q);

		/* z = e(g1, g2). */
		pc_map(z, g, q);

		g2_get_ord(n);

		do {
			bn_rand(d, BN_POS, 2 * pc_param_level());
			bn_mod(d, d, n);
		} while (bn_is_zero(d));

		/* q = d * g2. */
		g2_mul_gen(q, d);
		g2_norm(q, q);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

void cp_bbs_sig(int *b, g1_t s, unsigned char *msg, int len, bn_t d) {
	bn_t m, n, r;
	unsigned char hash[MD_LEN];
	unsigned char key[len + PC_BYTES];

	bn_null(m);
	bn_null(n);
	bn_null(r);

	TRY {
		bn_new(m);
		bn_new(n);
		bn_new(r);

		g1_get_ord(n);

		/* hash = H(SJ, msg). */
		memcpy(key, msg, len);
		bn_write_bin(key + len, PC_BYTES, d);
		md_map(hash, key, len + PC_BYTES);

		*b =hash[0] & 0x01;

		/* m = H(b, msg). */
		key[len] = *b;
		md_map(hash, key, len + 1);
		bn_read_bin(m, hash, MD_LEN);

		/* m = 1/(m + d) mod n. */
		bn_add(m, m, d);
		bn_gcd_ext(r, m, NULL, m, n);
		if (bn_sign(m) == BN_NEG) {
			bn_add(m, m, n);
		}
		/* s = 1/(m+d) * g1. */
		g1_mul_gen(s, m);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(m);
		bn_free(n);
		bn_free(r);
	}
}

int cp_bbs_ver(int b, g1_t s, unsigned char *msg, int len, g2_t q, gt_t z) {
	bn_t m, n;
	g2_t g;
	gt_t e;
	unsigned char hash[MD_LEN];
	unsigned char h[len + 1];
	int result = 0;

	bn_null(m);
	bn_null(n);
	g2_null(g);
	gt_null(e);

	TRY {
		bn_new(m);
		bn_new(n);
		g2_new(g);
		gt_new(e);

		g2_get_ord(n);

		/* hash = H(b, msg). */
		memcpy(h, msg, len);
		h[len] = b;

		md_map(hash, h, len + 1);
		bn_read_bin(m, hash, MD_LEN);
		bn_mod(m, m, n);

		g2_mul_gen(g, m);
		g2_add(g, g, q);
		g2_norm(g, g);

		pc_map(e, s, g);

		if (gt_cmp(e, z) == CMP_EQ) {
			result = 1;
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(m);
		bn_free(n);
		g2_free(g);
		gt_free(e);
	}
	return result;
}
