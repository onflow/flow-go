/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2013 RELIC Authors
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
 * Implementation of the ECIES cryptosystem.
 *
 * @version $Id$
 * @ingroup cp
 */

#include <string.h>

#include "relic_core.h"
#include "relic_bn.h"
#include "relic_util.h"
#include "relic_cp.h"
#include "relic_md.h"
#include "relic_bc.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int cp_ecies_gen(bn_t d, ec_t q) {
	bn_t n;
	int result = STS_OK;

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
		result = STS_ERR;
	}
	FINALLY {
		bn_free(n);
	}
	
	return result;
}

int cp_ecies_enc(unsigned char *out, int *out_len, unsigned char *in,
		int in_len, unsigned char *iv, unsigned char *mac, ec_t r, ec_t q) {
	bn_t k, n, x;
	ec_t p;
	int l, size = ec_param_level()/8, result = STS_OK;
	unsigned char _x[EC_BYTES], key[2 * size];
	
	bn_null(k);
	bn_null(n);
	bn_null(x);
	ec_null(p);
	
	TRY {
		bn_new(k);
		bn_new(n);
		bn_new(x);
		ec_new(p);
		
		ec_curve_get_ord(n);

		do {
			bn_rand(k, BN_POS, bn_bits(n));
			bn_mod(k, k, n);
		} while (bn_is_zero(k));

		ec_mul_gen(r, k);
		ec_mul(p, q, k);
		ec_get_x(x, p);
		bn_size_bin(&l, x);
		bn_write_bin(_x, l, x);
		md_kdf2(key, 2 * size, _x, l);
		if (bc_aes_cbc_enc(out, out_len, in, in_len, key, 8 * size, iv) != STS_OK) {
			result = STS_ERR;
		} else {
			md_hmac(mac, out, *out_len, key + size, size);
		}
	} CATCH_ANY {
		result = STS_ERR;
	} FINALLY {
		bn_free(k);
		bn_free(n);
		bn_free(x);
		ec_free(p);
	}
	
	return result;
}

int cp_ecies_dec(unsigned char *out, int *out_len, unsigned char *in,
		int in_len, unsigned char *iv, unsigned char *mac, ec_t r, bn_t d) {
	ec_t p;
	bn_t x;
	int l, size = ec_param_level()/8, result = STS_OK;
	unsigned char _x[EC_BYTES], h[MD_LEN], key[2 * size];
	
	bn_null(x);
	ec_null(p);
	
	TRY {
		bn_new(x);
		ec_new(p);
		
		ec_mul(p, r, d);
		ec_get_x(x, p);
		bn_size_bin(&l, x);
		bn_write_bin(_x, l, x);
		md_kdf2(key, 2 * size, _x, l);
		md_hmac(h, in, in_len, key + size, size);
		if (util_cmp_const(h, mac, MD_LEN)) {
			result = STS_ERR;
		} else {
			if (bc_aes_cbc_dec(out, out_len, in, in_len, key, 8 * size, iv) != STS_OK) {
				result = STS_ERR;
			}
		}
	} CATCH_ANY {
		result = STS_ERR;
	} FINALLY {
		bn_free(x);
		ec_free(p);
	}
	
	return result;
}
