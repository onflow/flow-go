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
 * Implementation of the low-level quadratic extension field multiplication
 * functions.
 *
 * @version $Id$
 * @ingroup fp
 */

#include "relic_fp.h"
#include "relic_pp.h"
#include "relic_core.h"
#include "relic_conf.h"
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

void fp2_muln_low(fp2_t c, fp2_t a, fp2_t b) {
	dv_t t0, t1, t2, t3, t4;

	dv_null(t0);
	dv_null(t1);
	dv_null(t2);
	dv_null(t3);
	dv_null(t4);

	TRY {

		dv_new(t0);
		dv_new(t1);
		dv_new(t2);
		dv_new(t3);
		dv_new(t4);

		/* Karatsuba algorithm. */

		/* t2 = a0 + a1, t1 = b0 + b1. */
#ifdef FP_ROOM
		fp_addn_low(t2, a[0], a[1]);
		fp_addn_low(t1, b[0], b[1]);
#else
		fp_add(t2, a[0], a[1]);
		fp_add(t1, b[0], b[1]);
#endif
		/* t3 = (a0 + a1) * (b0 + b1). */
		/* t0 = a0 * b0, t4 = a1 * b1. */
		fp_muln_low(t0, a[0], b[0]);
		fp_muln_low(t4, a[1], b[1]);
		fp_muln_low(t3, t2, t1);

		/* t2 = (a0 * b0) + (a1 * b1). */
		fp_addd_low(t2, t0, t4);

		/* t1 = (a0 * b0) + u^2 * (a1 * b1). */
		fp_subc_low(t1, t0, t4);
#ifndef FP_QNRES
		/* t1 = u^2 * (a1 * b1). */
		for (int i = -1; i > fp_prime_get_qnr(); i--) {
			fp_subc_low(t1, t1, t4);
		}
#endif
		/* c0 = t1 mod p. */
#if FP_RDC == MONTY
		fp_rdcn_low(c[0], t1);
#else
		fp_rdc(c[0], t1);
#endif

		/* t4 = t3 - t2. */
#ifdef FP_ROOM
		fp_subd_low(t4, t3, t2);
#else
		fp_subc_low(t4, t3, t2);
#endif

		/* c1 = t4 mod p. */
#if FP_RDC == MONTY
		fp_rdcn_low(c[1], t4);
#else
		fp_rdc(c[1], t4);
#endif
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t0);
		dv_free(t1);
		dv_free(t2);
		dv_free(t3);
		dv_free(t4);
	}
}
