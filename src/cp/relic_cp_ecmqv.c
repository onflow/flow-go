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
 * Implementation of the ECMQV protocol.
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

void cp_ecmqv_gen(bn_t d, ec_t q) {
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

void cp_ecmqv_key(unsigned char *key, int key_len, bn_t d1, bn_t d2, ec_t q2u, ec_t q1v, ec_t q2v) {
	ec_t p;
	bn_t x, n, s;
	int l;
	unsigned char _x[EC_BYTES];

	ec_null(p);
	bn_null(x);
	bn_null(n);
	bn_null(s);

	TRY {
		ec_new(p);
		bn_new(x);
		bn_new(n);
		bn_new(s);

		ec_curve_get_ord(n);
		l = bn_bits(n);
		l = (l % 2) ? (l + 1) / 2 : l / 2;

		ec_get_x(x, q2u);
		bn_mod_2b(x, x, l);

		bn_set_bit(x, l, 1);

		bn_mul(s, x, d1);
		bn_mod(s, s, n);
		bn_add(s, s, d2);
		bn_mod(s, s, n);

		ec_get_x(x, q2v);
		bn_mod_2b(x, x, l);

		bn_set_bit(x, l, 1);

		bn_mul(x, x, s);
		bn_mod(x, x, n);

		ec_mul_sim(p, q2v, s, q1v, x);

		ec_get_x(x, p);
		bn_size_bin(&l, x);
		bn_write_bin(_x, l, x);
		md_kdf2(key, key_len, _x, l);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ec_free(p);
		bn_free(x);
		bn_free(n);
		bn_free(s);
	}
}
