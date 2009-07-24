/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of the ECDSA protocol.
 *
 * @version $Id$
 * @ingroup cp
 */

#include <stdio.h>
#include<string.h>
#include<math.h>
#include<stdlib.h>
#include<stdint.h>

#include "relic.h"
#include "relic_test.h"
#include "relic_bench.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void cp_ecdsa_init() {

}

void cp_ecdsa_gen(bn_t d, eb_t q) {
	bn_t n = NULL;

	n = eb_curve_get_ord();

	do {
		bn_rand(d, BN_POS, bn_bits(n));
		bn_mod_basic(d, d, n);
	} while (bn_is_zero(d));

	eb_mul_gen(q, d);
	cp_ecdsa_init();
}

#if CP_ECDSA == BASIC || !defined(STRIP)

void cp_ecdsa_sign_basic(bn_t r, bn_t s, unsigned char *msg, int len, bn_t d) {
	bn_t n = NULL, k = NULL, x = NULL, e = NULL;
	eb_t p = NULL;
	unsigned char hash[HF_LEN];

	TRY {
		n = eb_curve_get_ord();
		bn_new(k);
		bn_new(x);
		bn_new(e);
		eb_new(p);

		do {
			do {
				bn_rand(k, BN_POS, bn_bits(n));
				bn_mod_basic(k, k, n);
			} while (bn_is_zero(k));

			eb_mul_gen(p, k);
			bn_read_raw(x, p->x, FB_DIGS, BN_POS);
			bn_mod_basic(r, x, n);
		} while (bn_is_zero(r));

		hf_map(hash, msg, len);

		bn_read_bin(e, hash, 20, BN_POS);

		bn_mul(s, d, r);
		bn_mod_basic(s, s, n);
		bn_add(s, s, e);
		bn_mod_basic(s, s, n);
		bn_gcd_ext(x, NULL, k, n, k);
		if (bn_sign(k) == BN_NEG) {
			bn_add(k, k, n);
		}
		bn_mul(s, s, k);
		bn_mod_basic(s, s, n);

	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(k);
		bn_free(x);
		bn_free(e);
		eb_free(p);
	}
}

int cp_ecdsa_ver_basic(bn_t r, bn_t s, unsigned char *msg, int len, eb_t q) {
	bn_t n = NULL, k = NULL, e = NULL, v = NULL;
	eb_t p = NULL;
	unsigned char hash[20];
	int result = 0;

	TRY {
		n = eb_curve_get_ord();
		bn_new(e);
		bn_new(v);
		bn_new(k);
		eb_new(p);

		hf_map(hash, msg, len);

		if (bn_sign(r) == BN_POS && bn_sign(s) == BN_POS &&
				!bn_is_zero(r) && !bn_is_zero(s)) {
			if (bn_cmp(r, n) == CMP_LT && bn_cmp(s, n) == CMP_LT) {
				bn_gcd_ext(e, k, NULL, s, n);
				if (bn_sign(k) == BN_NEG) {
					bn_add(k, k, n);
				}

				bn_read_bin(e, hash, 20, BN_POS);

				bn_mul(e, e, k);
				bn_mod_basic(e, e, n);
				bn_mul(v, r, k);
				bn_mod_basic(v, v, n);

				eb_mul_sim_gen(p, e, q, v);

				bn_read_raw(v, p->x, FB_DIGS, BN_POS);

				bn_mod_basic(v, v, n);

				if (bn_cmp(v, r) == CMP_EQ) {
					result = 1;
				}
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(e);
		bn_free(v);
		bn_free(k);
		eb_free(p);
	}
	return result;
}

#endif
