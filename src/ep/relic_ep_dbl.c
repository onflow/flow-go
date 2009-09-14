/*
 * Copyright 2007 Project RELIC
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of the point doubling on prime elliptic curves.
 *
 * @version $Id$
 * @ingroup ep
 */

#include "string.h"

#include "relic_core.h"
#include "relic_ep.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if EP_ADD == BASIC || defined(EP_MIXED) || !defined(STRIP)

#if defined(EP_ORDIN)
/**
 * Doubles a point represented in affine coordinates on an ordinary prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the point to double.
 */
static void ep_dbl_basic_ordin(ep_t r, ep_t p) {
	fp_t t0, t1, t2;

	TRY {
		fp_new(t0);
		fp_new(t1);
		fp_new(t2);

		/* t0 = 1/2 * y1. */
		fp_dbl(t0, p->y);
		fp_inv(t0, t0);

		/* t1 = 3 * x1^2 + a. */
		fp_sqr(t1, p->x);
		fp_copy(t2, t1);
		fp_dbl(t1, t1);
		fp_add(t1, t1, t2);

		switch (ep_curve_opt_a()) {
			case EP_OPT_ZERO:
				break;
			case EP_OPT_ONE:
				fp_add_dig(t1, t1, (dig_t)1);
				break;
			case EP_OPT_DIGIT:
				fp_add_dig(t1, t1, ep_curve_get_a()[0]);
				break;
			default:
				fp_add(t1, t1, ep_curve_get_a());
				break;
		}

		/* t1 = (3 * x1^2 + a)/(2 * y1). */
		fp_mul(t1, t1, t0);

		/* t2 = t1^2. */
		fp_sqr(t2, t1);

		/* x3 = t1^2 - 2 * x1. */
		fp_dbl(t0, p->x);
		fp_sub(t0, t2, t0);

		/* y3 = t1 * (x1 - x3) - y1. */
		fp_sub(t2, p->x, t0);
		fp_mul(t1, t1, t2);
		fp_sub(r->y, t1, p->y);

		fp_copy(r->x, t0);
		fp_copy(r->z, p->z);

		r->norm = 1;

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
#endif /* EP_ORDIN */

#if defined(EP_SUPER)
/**
 * Doubles a point represented in affine coordinates on a supersingular prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the point to double.
 */
static void ep_dbl_basic_super(ep_t r, ep_t p) {
	fp_t t0 = NULL, t1 = NULL, t2 = NULL;

	TRY {
		fp_new(t0);
		fp_new(t1);
		fp_new(t2);

		/* t0 = x1^2. */
		fp_sqr(t0, p->x);

		/* t0 = (x1^2 + a)/c. */
		switch (ep_curve_opt_a()) {
			case EP_OPT_ZERO:
				break;
			case EP_OPT_ONE:
				fp_add_dig(t0, t0, (dig_t)1);
				break;
			case EP_OPT_DIGIT:
				fp_add_dig(t0, t0, ep_curve_get_a()[0]);
				break;
			default:
				fp_add(t0, t0, ep_curve_get_a());
				break;
		}

		switch (ep_curve_opt_c()) {
			case EP_OPT_ZERO:
			case EP_OPT_ONE:
				break;
			case EP_OPT_DIGIT:
			default:
				fp_inv(t2, ep_curve_get_c());
				fp_mul(t0, t0, t2);
				fp_add(t0, t0, ep_curve_get_a());
				break;
		}

		/* t2 = ((x1^2 + a)/c)^2. */
		fp_sqr(t2, t0);

		/* y3 = ((x1^2 + a)/c) * (x1 + x3) + y1 + c. */
		fp_add(t1, t2, p->x);
		fp_mul(t1, t1, t0);
		fp_add(r->y, t1, p->y);

		switch (ep_curve_opt_c()) {
			case EP_OPT_ZERO:
				break;
			case EP_OPT_ONE:
				fp_add_dig(r->y, r->y, (dig_t)1);
				break;
			case EP_OPT_DIGIT:
				fp_add_dig(r->y, r->y, ep_curve_get_c()[0]);
				break;
			default:
				fp_add(r->y, r->y, ep_curve_get_c());
				break;
		}

		/* x3 = ((x1^2 + a)/c)^2. */
		fp_copy(r->x, t2);
		fp_copy(r->z, p->z);

		r->norm = 1;

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
#endif /* EP_SUPER */

#endif /* EP_ADD == BASIC */

#if EP_ADD == PROJC || defined(EP_MIXED) || !defined(STRIP)

#if defined(EP_ORDIN) || defined(EP_KBLTZ)
/**
 * Doubles a point represented in projective coordinates on an ordinary prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the point to double.
 */
void ep_dbl_projc_ordin(ep_t r, ep_t p) {
	fp_t t0 = NULL, t1 = NULL;

	fp_new(t0);
	fp_new(t1);

	/* t0 = B = x1^2. */
	fp_sqr(t0, p->x);
	/* C = B + y1. */
	fp_add(r->y, t0, p->y);

	if (!p->norm) {
		/* A = x1 * z1. */
		fp_mul(t1, p->x, p->z);
		/* z3 = A^2. */
		fp_sqr(r->z, t1);
	} else {
		/* if z1 = 1, A = x1. */
		fp_copy(t1, p->x);
		/* if z1 = 1, z3 = x1^2. */
		fp_copy(r->z, t0);
	}

	/* t1 = D = A * C. */
	fp_mul(t1, t1, r->y);

	/* C^2 + D. */
	fp_sqr(r->y, r->y);
	fp_add(r->x, t1, r->y);

	/* C^2 + D + a2 * z3. */
	switch (ep_curve_opt_a()) {
		case EP_OPT_ZERO:
			break;
		case EP_OPT_ONE:
			fp_add(r->x, r->z, r->x);
			break;
		case EP_OPT_DIGIT:
			fp_mul(r->y, r->z, ep_curve_get_a());
			fp_add(r->x, r->y, r->x);
			break;
		default:
			fp_mul(r->y, r->z, ep_curve_get_a());
			fp_add(r->x, r->y, r->x);
			break;
	}

	/* t1 = (D + z3). */
	fp_add(t1, t1, r->z);
	/* t0 = B^2. */
	fp_sqr(t0, t0);
	/* t0 = B^2 * z3. */
	fp_mul(t0, t0, r->z);
	/* y3 = (D + z3) * r3 + B^2 * z3. */
	fp_mul(r->y, t1, r->x);
	fp_add(r->y, r->y, t0);

	r->norm = 0;

	fp_free(t0);
	fp_free(t1);
}
#endif /* EP_ORDIN || EP_KBLTZ */

#if defined(EP_SUPER)
/**
 * Doubles a point represented in projective coordinates on an supersingular
 * prime elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the point to double.
 */
void ep_dbl_projc_super(ep_t r, ep_t p) {
	RLC_PROLOGUE;

	/* x3 = x1^4 */
	fp_sqr(r->x, p->x);
	fp_sqr(r->x, r->x);
	/* y3 = y1^4. */
	fp_sqr(r->y, p->y);
	fp_sqr(r->y, r->y);

	if (!p->norm) {
		/* z3 = z1^4. */
		fp_sqr(r->z, p->z);
		fp_sqr(r->z, p->z);
		/* y3 = x1^4 + y1^4. */
		fp_add(r->y, r->x, r->y);
		/* x3 = x1^4 + z1^4. */
		fp_add(r->x, r->x, r->z);
	} else {
		/* y3 = x1^4 + y1^4. */
		fp_add(r->y, r->x, r->y);
		/* x3 = x1^4 + 1. */
		fp_add_dig(r->x, r->x, 1);
		/* r is still in affine coordinates. */
		fp_copy(r->z, p->z);
	}

	r->norm = p->norm;

	RLC_EPILOGUE;
}
#endif /* EP_SUPER */

#endif /* EP_ADD == PROJC */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EP_ADD == BASIC || defined(EP_MIXED) || !defined(STRIP)

void ep_dbl_basic(ep_t r, ep_t p) {
	if (ep_is_infty(p)) {
		ep_set_infty(r);
		return;
	}
#if defined(EP_SUPER)
	if (ep_curve_is_super()) {
		ep_dbl_basic_super(r, p);
		return;
	}
#endif

#if defined(EP_ORDIN)
	ep_dbl_basic_ordin(r, p);
#endif
}

#endif

#if EP_ADD == PROJC || defined(EP_MIXED) || !defined(STRIP)

void ep_dbl_projc(ep_t r, ep_t p) {
	if (ep_is_infty(p)) {
		ep_set_infty(r);
		return;
	}

	if (fp_is_zero(p->x)) {
		ep_set_infty(r);
		return;
	}
#if defined(EP_SUPER)
	if (ep_curve_is_super()) {
		ep_dbl_projc_super(r, p);
		return;
	}
#endif

#if defined(EP_ORDIN)
	ep_dbl_projc_ordin(r, p);
#endif
}

#endif
