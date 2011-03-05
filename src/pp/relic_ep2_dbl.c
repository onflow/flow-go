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
 * Implementation of doubling on elliptic prime curves over quadratic
 * extensions.
 *
 * @version $Id: relic_pp_ep2.c 463 2010-07-13 21:12:13Z conradoplg $
 * @ingroup pp
 */

#include "relic_core.h"
#include "relic_md.h"
#include "relic_pp.h"
#include "relic_error.h"
#include "relic_conf.h"
#include "relic_fp_low.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if EP_ADD == BASIC || defined(EP_MIXED) || !defined(STRIP)

/**
 * Doubles a point represented in affine coordinates on an ordinary prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param s					- the resulting slope.
 * @param p					- the point to double.
 */
static void ep2_dbl_basic_impl(ep2_t r, fp2_t s, fp2_t e, ep2_t p) {
	fp2_t t0, t1, t2;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);

		/* t0 = 1/(2 * y1). */
		fp2_dbl(t0, p->y);
		fp2_inv(t0, t0);

		/* t1 = 3 * x1^2 + a. */
		fp2_sqr(t1, p->x);
		fp2_copy(t2, t1);
		fp2_dbl(t1, t1);
		fp2_add(t1, t1, t2);

		if (ep2_curve_is_twist()) {
			switch (ep_curve_opt_a()) {
				case OPT_ZERO:
					break;
				case OPT_ONE:
					fp_set_dig(t2[0], 1);
					fp2_mul_art(t2, t2);
					fp2_mul_art(t2, t2);
					fp2_add(t1, t1, t2);
					break;
				case OPT_DIGIT:
					fp_set_dig(t2[0], ep_curve_get_a()[0]);
					fp2_mul_art(t2, t2);
					fp2_mul_art(t2, t2);
					fp2_add(t1, t1, t2);
					break;
				default:
					fp_copy(t2[0], ep_curve_get_a());
					fp_zero(t2[1]);
					fp2_mul_art(t2, t2);
					fp2_mul_art(t2, t2);
					fp2_add(t1, t1, t2);
					break;
			}
		}

		/* t1 = (3 * x1^2 + a)/(2 * y1). */
		fp2_mul(t1, t1, t0);

		if (s != NULL) {
			fp2_copy(s, t1);
		}

		/* t2 = t1^2. */
		fp2_sqr(t2, t1);

		/* x3 = t1^2 - 2 * x1. */
		fp2_dbl(t0, p->x);
		fp2_sub(t0, t2, t0);

		/* y3 = t1 * (x1 - x3) - y1. */
		fp2_sub(t2, p->x, t0);
		fp2_mul(t1, t1, t2);

		fp2_sub(r->y, t1, p->y);

		fp2_copy(r->x, t0);
		fp2_copy(r->z, p->z);

		r->norm = 1;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
	}
}

#endif /* EP_ADD == BASIC */

#if EP_ADD == PROJC || defined(EP_MIXED) || !defined(STRIP)

/**
 * Doubles a point represented in affine coordinates on an ordinary prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the point to double.
 */
static void ep2_dbl_projc_impl(ep2_t r, fp2_t s, fp2_t e, ep2_t p) {
	fp2_t t0, t1, t2, t3;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);
	fp2_null(t3);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);

		fp2_sqr(t0, p->x);
		fp2_add(t2, t0, t0);
		fp2_add(t0, t2, t0);

		fp2_sqr(t3, p->y);
		fp2_mul(t1, t3, p->x);
		fp2_add(t1, t1, t1);
		fp2_add(t1, t1, t1);
		fp2_sqr(r->x, t0);
		fp2_add(t2, t1, t1);
		fp2_sub(r->x, r->x, t2);
		fp2_mul(r->z, p->z, p->y);
		fp2_add(r->z, r->z, r->z);
		fp2_add(t3, t3, t3);
		if (s != NULL) {
			fp2_copy(s, t0);
		}
		if (e != NULL) {
			fp2_copy(e, t3);
		}
		fp2_sqr(t3, t3);
		fp2_add(t3, t3, t3);
		fp2_sub(t1, t1, r->x);
		fp2_mul(r->y, t0, t1);
		fp2_sub(r->y, r->y, t3);

		r->norm = 0;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
		fp2_free(t3);
	}
}

#endif /* EP_ADD == PROJC || EP_MIXED */

/*============================================================================*/
	/* Public definitions                                                         */
/*============================================================================*/

#if EP_ADD == BASIC || defined(EP_MIXED) || !defined(STRIP)

void ep2_dbl_basic(ep2_t r, ep2_t p) {
	if (ep2_is_infty(p)) {
		ep2_set_infty(r);
		return;
	}

	ep2_dbl_basic_impl(r, NULL, NULL, p);
}

void ep2_dbl_slp_basic(ep2_t r, fp2_t s, fp2_t e, ep2_t p) {
	if (ep2_is_infty(p)) {
		ep2_set_infty(r);
		return;
	}

	ep2_dbl_basic_impl(r, s, e, p);
}

#endif

#if EP_ADD == PROJC || defined(EP_MIXED) || !defined(STRIP)

void ep2_dbl_projc(ep2_t r, ep2_t p) {
	if (ep2_is_infty(p)) {
		ep2_set_infty(r);
		return;
	}

	ep2_dbl_projc_impl(r, NULL, NULL, p);
}

void ep2_dbl_slp_projc(ep2_t r, fp2_t s, fp2_t e, ep2_t p) {
	if (ep2_is_infty(p)) {
		ep2_set_infty(r);
		return;
	}

	ep2_dbl_projc_impl(r, s, e, p);
}

#endif
