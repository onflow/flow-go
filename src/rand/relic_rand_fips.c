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
 * Implementation of the fips 186-2 (cn1) pseudo-random number generator.
 *
 * @version $Id: relic_rand.c 1522 2013-08-27 15:55:54Z dfaranha $
 * @ingroup rand
 */

#include <string.h>

#include "relic_conf.h"
#include "relic_core.h"
#include "relic_label.h"
#include "relic_rand.h"
#include "relic_md.h"
#include "relic_err.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Accumulates the hash value plus one in the internal state.
 * 
 * @param[in,out] state		- the internal state.
 * @param[in] hash			- the hash value.
 */
static void rand_add_inc(unsigned char *state, unsigned char *hash) {
	int carry = 1;
	for (int i = MD_LEN_SHONE - 1; i >= 0; i--) {
		int16_t s;
		s = (state[i] + hash[i] + carry);
		state[i] = s & 0xFF;
		carry = s >> 8;
	}
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void rand_bytes(unsigned char *buf, int size) {
	unsigned char hash[MD_LEN_SHONE];
	ctx_t *ctx = core_get();

	int j = 0;
	while (j < size) {
		/* w_0 = G(t, XKEY) */
		md_map_shone_mid(hash, ctx->rand, sizeof(ctx->rand));
		/* XKEY = (XKEY + w_0 + 1) mod 2^b */
		rand_add_inc(ctx->rand, hash);

		memcpy(buf + j, hash, MIN(size, MD_LEN_SHONE));
		j += MIN(size, MD_LEN_SHONE);

		/* w_1 = G(t, XKEY) */
		md_map_shone_mid(hash, ctx->rand, sizeof(ctx->rand));
		/* XKEY = (XKEY + w_1 + 1) mod 2^b */
		rand_add(ctx->rand, hash);

		memcpy(buf + j, hash, MIN(size, MD_LEN_SHONE));
		j += MIN(size, MD_LEN_SHONE);
	}
}

void rand_seed(unsigned char *buf, int size) {
	int i;
	ctx_t *ctx = core_get();

	if (size < MD_LEN_SHONE) {
		THROW(ERR_NO_VALID);
	}

	/* XKEY = SEED, throws away additional bytes. */
	for (i = 0; i < MD_LEN_SHONE; i++) {
		ctx->rand[i] = buf[i];
	}
	ctx->seeded = 1;
}
