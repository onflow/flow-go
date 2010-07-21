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
 * Implementation of hashing to a prime elliptic curve.
 *
 * @version $Id: relic_ep_util.c 447 2010-07-12 02:24:21Z conradoplg $
 * @ingroup ep
 */

#include "relic_core.h"
#include "relic_md.h"
#include "relic_ep.h"
#include "relic_error.h"
#include "relic_conf.h"
#include "relic_fp_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ep_map(ep_t p, unsigned char *msg, int len) {
	fp_t t0, t1;
	int bits, digits;
	unsigned char digest[MD_LEN];

	fp_null(t0);
	fp_null(t1);

	TRY {
		fp_new(t0);
		fp_new(t1);

		md_map(digest, msg, len);
		fp_set_dig(p->z, 1);
		memcpy(p->x, digest, MIN(FP_BYTES, MD_LEN));

		SPLIT(bits, digits, FP_BITS, FP_DIG_LOG);
		if (bits > 0) {
			dig_t mask = ((dig_t)1 << (dig_t)bits) - 1;
			p->x[FP_DIGS - 1] &= mask;
		}

		while (fp_cmp(p->x, fp_prime_get()) != CMP_LT) {
			fp_subn_low(p->x, p->x, fp_prime_get());
		}

		while (1) {
			/* t0 = x1^2. */
			fp_sqr(t0, p->x);
			/* t1 = x1^3. */
			fp_mul(t1, t0, p->x);

			/* t1 = x1^3 + a * x1 + b. */
			switch (ep_curve_opt_a()) {
				case OPT_ZERO:
					break;
				case OPT_ONE:
					fp_add(t1, t1, p->x);
					break;
				case OPT_DIGIT:
					fp_mul_dig(t0, p->x, ep_curve_get_a()[0]);
					fp_add(t1, t1, t0);
					break;
				default:
					fp_mul(t0, p->x, ep_curve_get_a());
					fp_add(t1, t1, t0);
					break;
			}

			fp_add(t1, t1, ep_curve_get_b());

			if (fp_srt(p->y, t1)) {
				p->norm = 1;
				break;
			}
			fp_add_dig(p->x, p->x, 1);
		}
		/* Assuming cofactor is 1 */
		/* TODO: generalize? */
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(t0);
		fp_free(t1);
	}
}
