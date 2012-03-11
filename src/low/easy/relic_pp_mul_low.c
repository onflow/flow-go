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
#include "relic_bn_low.h"
#include "relic_fp_low.h"
#include "relic_pp_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fp2_muln_low(dv2_t c, fp2_t a, fp2_t b) {
	align dig_t t0[2 * FP_DIGS], t1[2 * FP_DIGS], t2[2 * FP_DIGS];

	/* Karatsuba algorithm. */

	/* t0 = a_0 + a_1, t1 = b_0 + b_1. */
#ifdef FP_SPACE
	fp_addn_low(t0, a[0], a[1]);
	fp_addn_low(t1, b[0], b[1]);
#else
	fp_addm_low(t0, a[0], a[1]);
	fp_addm_low(t1, b[0], b[1]);
#endif
	/* c_0 = a_0 * b_0, c_1 = a_1 * b_1. */
	fp_muln_low(c[0], a[0], b[0]);
	fp_muln_low(c[1], a[1], b[1]);
	/* t2 = (a_0 + a_1) * (b_0 + b_1). */
	fp_muln_low(t2, t0, t1);

	/* t0 = (a_0 * b_0) + (a_1 * b_1). */
#ifdef FP_SPACE
	fp_addd_low(t0, c[0], c[1]);
#else
	fp_addc_low(t0, c[0], c[1]);
#endif

	/* c_0 = (a_0 * b_0) + u^2 * (a_1 * b_1). */
	fp_subc_low(c[0], c[0], c[1]);

#ifndef FP_QNRES
	/* t1 = u^2 * (a_1 * b_1). */
	for (int i = -1; i > fp_prime_get_qnr(); i--) {
		fp_subc_low(c[0], c[0], c[1]);
	}
#endif

	/* c_1 = t2 - t0. */
#ifdef FP_SPACE
	fp_subd_low(c[1], t2, t0);
#else
	fp_subc_low(c[1], t2, t0);
#endif
}

void fp2_mulc_low(dv2_t c, fp2_t a, fp2_t b) {
	align dig_t t0[2 * FP_DIGS], t1[2 * FP_DIGS], t2[2 * FP_DIGS];

	/* Karatsuba algorithm. */

	/* t0 = a_0 + a_1, t1 = b_0 + b_1. */
	fp_addn_low(t0, a[0], a[1]);
	fp_addn_low(t1, b[0], b[1]);

	/* c_0 = a_0 * b_0, c_1 = a_1 * b_1, t2 = (a_0 + a_1) * (b_0 + b_1). */
	fp_muln_low(c[0], a[0], b[0]);
	fp_muln_low(c[1], a[1], b[1]);
	fp_muln_low(t2, t0, t1);

	/* t0 = (a_0 * b_0) + (a_1 * b_1). */
	fp_addd_low(t0, c[0], c[1]);

	/* c_0 = (a_0 * b_0) + u^2 * (a_1 * b_1). */
	fp_subd_low(c[0], c[0], c[1]);

	/* c_1 = t2 - t0. */
	fp_subd_low(c[1], t2, t0);

	/* c_0 = c_0 + 2^N * p/4. */
	bn_lshb_low(c[0] + FP_DIGS - 1, c[0] + FP_DIGS - 1, FP_DIGS + 1, 2);
	fp_addn_low(c[0] + FP_DIGS, c[0] + FP_DIGS, fp_prime_get());
	bn_rshb_low(c[0] + FP_DIGS - 1, c[0] + FP_DIGS - 1, FP_DIGS + 1, 2);
}

void fp2_mulm_low(fp2_t c, fp2_t a, fp2_t b) {
	align dv2_t t;

	dv2_null(t);

	TRY {
		dv2_new(t);
		fp2_muln_low(t, a, b);
		fp2_rdcn_low(c, t);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		dv2_free(t);
	}
}
