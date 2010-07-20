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
 * Implementation of the low-level quadratic extension field multiplication
 * functions.
 *
 * @version $Id$
 * @ingroup fp
 */

#include "relic_fp.h"
#include "relic_pp.h"
#include "relic_core.h"
#include "relic_error.h"
#include "relic_fp_low.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Constant used to indicate that there's some room left in the storage of
 * prime field elements. This can be used to avoid carries.
 */
#if (FP_PRIME % WORD) == (WORD - 2)
#define FP_ROOM
#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp2_sqrn_low(fp2_t c, fp2_t a) {
	fp_t t0, t1, t2;

	fp_null(t0);
	fp_null(t1);
	fp_null(t2);

	TRY {
		fp_new(t0);
		fp_new(t1);
		fp_new(t2);

		/* t0 = (a0 + a1). */
#ifdef FP_ROOM
		/* if we have room for carries, we can avoid reductions here. */
		fp_addn_low(t0, a[0], a[1]);
#else
		fp_add(t0, a[0], a[1]);
#endif
		/* t1 = (a0 - a1). */
		fp_subm_low(t1, a[0], a[1]);

#ifdef FP_QNRES

#ifdef FP_ROOM
		fp_dbln_low(t2, a[0]);
#else
		fp_dbl(t2, a[0]);
#endif
		/* c1 = 2 * a0 * a1. */
		fp_mulm_low(c[1], t2, a[1]);
		/* c0 = a0^2 + b_0^2 * u^2. */
		fp_mulm_low(c[0], t0, t1);

#else /* !FP_QNRES */

		/* t1 = u^2 * (a1 * b1). */
		for (int i = -1; i > fp_prime_get_qnr(); i--) {
			fp_sub(t1, t1, a[1]);
		}

		if (fp_prime_get_qnr() == -1) {
			/* t2 = 2 * a0. */
			fp_dbl(t2, a[0]);
			/* c1 = 2 * a0 * a1. */
			fp_mul(c[1], t2, a[1]);
			/* c0 = a0^2 + b_0^2 * u^2. */
			fp_mul(c[0], t0, t1);
		} else {
			/* c1 = a0 * a1. */
			fp_mul(c[1], a[0], a[1]);
			/* c0 = a0^2 + b_0^2 * u^2. */
			fp_mul(c[0], t0, t1);
			for (int i = -1; i > fp_prime_get_qnr(); i--) {
				fp_add(c[0], c[0], c[1]);
			}
			/* c1 = 2 * a0 * a1. */
			fp_dbl(c[1], c[1]);
		}
#endif
		/* c = c0 + c1 * u. */
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(t0);
		fp_free(t1);
		fp_free(t2);
	}
}
