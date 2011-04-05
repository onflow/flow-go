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
 * Implementation of the ECDSA protocol.
 *
 * @version $Id$
 * @ingroup cp
 */

#include "relic.h"
#include "relic_test.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void cp_ecdsa_gen(bn_t d, ec_t q) {
	bn_t n;

	bn_null(n);

	TRY {
		bn_new(n);

		ec_curve_get_ord(n);

		do {
			bn_rand(d, BN_POS, bn_bits(n));
			bn_mod(d, d, n);
		} while (bn_is_zero(d));

		ec_mul_gen(q, d);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
	}
}

void cp_ecdsa_sign(bn_t r, bn_t s, unsigned char *msg, int len, bn_t d) {
	bn_t n, k, x, e;
	ec_t p;
	unsigned char hash[MD_LEN];

	bn_null(n);
	bn_null(k);
	bn_null(x);
	bn_null(e);
	ec_null(p);

	TRY {
		bn_new(n);
		bn_new(k);
		bn_new(x);
		bn_new(e);
		ec_new(p);

		ec_curve_get_ord(n);
		do {
			do {
				do {
					bn_rand(k, BN_POS, bn_bits(n));
					bn_mod(k, k, n);
				} while (bn_is_zero(k));

				ec_mul_gen(p, k);
				bn_read_raw(x, p->x, EC_DIGS, BN_POS);
				bn_mod(r, x, n);
			} while (bn_is_zero(r));

			md_map(hash, msg, len);

			bn_read_bin(e, hash, MD_LEN, BN_POS);

			bn_mul(s, d, r);
			bn_mod(s, s, n);
			bn_add(s, s, e);
			bn_mod(s, s, n);
			bn_gcd_ext(x, k, NULL, k, n);
			if (bn_sign(k) == BN_NEG) {
				bn_add(k, k, n);
			}
			bn_mul(s, s, k);
			bn_mod(s, s, n);
		} while (bn_is_zero(s));
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
		bn_free(k);
		bn_free(x);
		bn_free(e);
		ec_free(p);
	}
}

int cp_ecdsa_ver(bn_t r, bn_t s, unsigned char *msg, int len, ec_t q) {
	bn_t n, k, e, v;
	dig_t d;
	ec_t p;
	unsigned char hash[MD_LEN];
	int result = 0;

	bn_null(n);
	bn_null(k);
	bn_null(e);
	bn_null(v);
	ec_null(p);

	TRY {
		bn_new(n);
		bn_new(e);
		bn_new(v);
		bn_new(k);
		ec_new(p);

		ec_curve_get_ord(n);
		md_map(hash, msg, len);

		if (bn_sign(r) == BN_POS && bn_sign(s) == BN_POS &&
				!bn_is_zero(r) && !bn_is_zero(s)) {
			if (bn_cmp(r, n) == CMP_LT && bn_cmp(s, n) == CMP_LT) {
				bn_gcd_ext(e, k, NULL, s, n);
				if (bn_sign(k) == BN_NEG) {
					bn_add(k, k, n);
				}

				bn_read_bin(e, hash, MD_LEN, BN_POS);

				bn_mul(e, e, k);
				bn_mod(e, e, n);
				bn_mul(v, r, k);
				bn_mod(v, v, n);

				ec_mul_sim_gen(p, e, q, v);

				bn_read_raw(v, p->x, EC_DIGS, BN_POS);

				bn_mod(v, v, n);

				d = 0;
				for (int i = 0; i < MIN(v->used, r->used); i++) {
					d |= v->dp[i] - r->dp[i];
				}
				result = (d ? 0 : 1);

				if (v->used != r->used) {
					result = 0;
				}

				if (ec_is_infty(p)) {
					result = 0;
				}
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(n);
		bn_free(e);
		bn_free(v);
		bn_free(k);
		ec_free(p);
	}
	return result;
}

