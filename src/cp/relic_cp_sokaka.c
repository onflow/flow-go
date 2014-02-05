/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2014 RELIC Authors
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
 * Implementation of the Sakai-Ohgishi-Kasahara Identity-Based Non-Interactive
 * Authenticated Key Agreement scheme.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>
#include <string.h>
#include <math.h>
#include <stdlib.h>
#include <stdint.h>

#include "relic.h"
#include "relic_test.h"
#include "relic_bench.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int cp_sokaka_gen(bn_t s) {
	bn_t n;
	int result = STS_OK;

	bn_null(n);

	TRY {
		bn_new(n);

		g1_get_ord(n);

		do {
			bn_rand(s, BN_POS, bn_bits(n));
			bn_mod(s, s, n);
		} while (bn_is_zero(s));
	}
	CATCH_ANY {
		result = STS_ERR;
	}
	FINALLY {
		bn_free(n);
	}
	return result;
}

int cp_sokaka_gen_prv(sokaka_t k, char *id, int len, bn_t s) {
	if (pc_map_is_type1()) {
		g1_map(k->s1, (uint8_t *)id, len);
		g1_mul(k->s1, k->s1, s);
	} else {
		g1_map(k->s1, (uint8_t *)id, len);
		g1_mul(k->s1, k->s1, s);
		g2_map(k->s2, (uint8_t *)id, len);
		g2_mul(k->s2, k->s2, s);
	}
	return STS_OK;
}

int cp_sokaka_key(uint8_t *key, unsigned int key_len, char *id1,
		int len1, sokaka_t k, char *id2, int len2) {
	int first = 0, result = STS_OK;
	g1_t p;
	g2_t q;
	gt_t e;

	g1_null(p);
	g2_null(q);
	gt_null(e);

	TRY {
		g1_new(p);
		g2_new(q);
		gt_new(e);

		if (len1 == len2) {
			if (strncmp(id1, id2, len1) == 0) {
				THROW(ERR_NO_VALID);
			}
			first = (strncmp(id1, id2, len1) == -1 ? 1 : 2);
		} else {
			if (len1 < len2) {
				if (strncmp(id1, id2, len1) == 0) {
					first = 1;
				} else {
					first = (strncmp(id1, id2, len2) == -1 ? 1 : 2);
				}
			} else {
				if (strncmp(id1, id2, len2) == 0) {
					first = 2;
				} else {
					first = (strncmp(id1, id2, len2) == -1 ? 1 : 2);
				}
			}
		}
		if (pc_map_is_type1()) {
			g2_map(q, (uint8_t *)id2, len2);
			pc_map(e, k->s1, q);
		} else {
			if (first == 1) {
				g2_map(q, (uint8_t *)id2, len2);
				pc_map(e, k->s1, q);
			} else {
				g1_map(p, (uint8_t *)id2, len2);
				pc_map(e, p, k->s2);
			}
		}
#if FP_PRIME < 1536
		uint8_t buf[12 * FP_BYTES], *ptr;
		ptr = buf;
		for (int i = 0; i < 2; i++) {
			for (int j = 0; j < 3; j++) {
				for (int m = 0; m < 2; m++) {
					fp_write_bin(ptr, FP_BYTES, e[i][j][m]);
					ptr += FP_BYTES;
				}
			}
		}
#else
		uint8_t buf[2 * FP_BYTES], *ptr;
		ptr = buf;
		for (int i = 0; i < 2; i++) {
			fp_write_bin(ptr, FP_BYTES, e[i]);
			ptr += FP_BYTES;
		}
#endif
		md_kdf1(key, key_len, buf, sizeof(buf));
	}
	CATCH_ANY {
		result = STS_ERR;
	}
	FINALLY {
		g1_free(p);
		g2_free(q);
		gt_free(e);
	}
	return result;
}
