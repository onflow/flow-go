/*
 * Copyright 2007-2009 RELIC Project
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
 * Implementation of point doubling on binary elliptic curves.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_eb.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if EB_ADD == BASIC || defined(EB_MIXED) || !defined(STRIP)

#if defined(EB_ORDIN) || defined(EB_KBLTZ)
/**
 * Doubles a point represented in affine coordinates on an ordinary binary
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the point to double.
 */
static void eb_dbl_basic_ordin(eb_t r, eb_t p) {
	fb_t t0 = NULL, t1 = NULL, t2 = NULL;

	TRY {
		fb_new(t0);
		fb_new(t1);
		fb_new(t2);

		/* t0 = 1/x1. */
		fb_inv(t0, p->x);
		/* t0 = y1/x1. */
		fb_mul(t0, t0, p->y);
		/* t0 = lambda = x1 + y1/x1. */
		fb_add(t0, t0, p->x);
		/* t1 = lambda^2. */
		fb_sqr(t1, t0);
		/* t2 = lambda^2 + lambda. */
		fb_add(t2, t1, t0);

		/* t2 = lambda^2 + lambda + a2. */
		switch (eb_curve_opt_a()) {
			case EB_OPT_ZERO:
				break;
			case EB_OPT_ONE:
				fb_add_dig(t2, t2, (dig_t)1);
				break;
			case EB_OPT_DIGIT:
				fb_add_dig(t2, t2, eb_curve_get_a()[0]);
				break;
			default:
				fb_add(t2, t2, eb_curve_get_a());
				break;
		}

		/* t1 = x1 + x3. */
		fb_add(t1, t2, p->x);

		/* t1 = lambda * (x1 + x3). */
		fb_mul(t1, t0, t1);

		fb_copy(r->x, t2);
		/* y3 = lambda * (x1 + x3) + x3 + y1. */
		fb_add(t1, t1, r->x);
		fb_add(r->y, t1, p->y);

		fb_copy(r->z, p->z);

		r->norm = 1;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t0);
		fb_free(t1);
		fb_free(t2);
	}
}
#endif /* EB_ORDIN || EB_KBLTZ */

#if defined(EB_SUPER)
/**
 * Doubles a point represented in affine coordinates on a supersingular binary
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the point to double.
 */
static void eb_dbl_basic_super(eb_t r, eb_t p) {
	fb_t t0 = NULL, t1 = NULL, t2 = NULL;

	TRY {
		fb_new(t0);
		fb_new(t1);
		fb_new(t2);

		/* t0 = x1^2. */
		fb_sqr(t0, p->x);

		/* t0 = (x1^2 + a)/c. */
		switch (eb_curve_opt_a()) {
			case EB_OPT_ZERO:
				break;
			case EB_OPT_ONE:
				fb_add_dig(t0, t0, (dig_t)1);
				break;
			case EB_OPT_DIGIT:
				fb_add_dig(t0, t0, eb_curve_get_a()[0]);
				break;
			default:
				fb_add(t0, t0, eb_curve_get_a());
				break;
		}

		switch (eb_curve_opt_c()) {
			case EB_OPT_ZERO:
			case EB_OPT_ONE:
				break;
			case EB_OPT_DIGIT:
			default:
				fb_inv(t2, eb_curve_get_c());
				fb_mul(t0, t0, t2);
				fb_add(t0, t0, eb_curve_get_a());
				break;
		}


		/* t2 = ((x1^2 + a)/c)^2. */
		fb_sqr(t2, t0);

		/* y3 = ((x1^2 + a)/c) * (x1 + x3) + y1 + c. */
		fb_add(t1, t2, p->x);
		fb_mul(t1, t1, t0);
		fb_add(r->y, t1, p->y);

		switch (eb_curve_opt_c()) {
			case EB_OPT_ZERO:
				break;
			case EB_OPT_ONE:
				fb_add_dig(r->y, r->y, (dig_t)1);
				break;
			case EB_OPT_DIGIT:
				fb_add_dig(r->y, r->y, eb_curve_get_c()[0]);
				break;
			default:
				fb_add(r->y, r->y, eb_curve_get_c());
				break;
		}

		/* x3 = ((x1^2 + a)/c)^2. */
		fb_copy(r->x, t2);
		fb_copy(r->z, p->z);

		r->norm = 1;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t0);
		fb_free(t1);
		fb_free(t2);
	}
}
#endif /* EB_SUPER */

#endif /* EB_ADD == BASIC */

#if EB_ADD == PROJC || !defined(STRIP)

#if defined(EB_ORDIN) || defined(EB_KBLTZ)
/**
 * Doubles a point represented in projective coordinates on an ordinary binary
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the point to double.
 */
void eb_dbl_projc_ordin(eb_t r, eb_t p) {
	fb_t t0 = NULL, t1 = NULL;

	TRY {
		fb_new(t0);
		fb_new(t1);

		/* t0 = B = x1^2. */
		fb_sqr(t0, p->x);
		/* C = B + y1. */
		fb_add(r->y, t0, p->y);

		if (!p->norm) {
			/* A = x1 * z1. */
			fb_mul(t1, p->x, p->z);
			/* z3 = A^2. */
			fb_sqr(r->z, t1);
		} else {
			/* if z1 = 1, A = x1. */
			fb_copy(t1, p->x);
			/* if z1 = 1, z3 = x1^2. */
			fb_copy(r->z, t0);
		}

		/* t1 = D = A * C. */
		fb_mul(t1, t1, r->y);

		/* C^2 + D. */
		fb_sqr(r->y, r->y);
		fb_add(r->x, t1, r->y);

		/* C^2 + D + a2 * z3. */
		switch (eb_curve_opt_a()) {
			case EB_OPT_ZERO:
				break;
			case EB_OPT_ONE:
				fb_add(r->x, r->z, r->x);
				break;
			case EB_OPT_DIGIT:
				fb_mul(r->y, r->z, eb_curve_get_a());
				fb_add(r->x, r->y, r->x);
				break;
			default:
				fb_mul(r->y, r->z, eb_curve_get_a());
				fb_add(r->x, r->y, r->x);
				break;
		}

		/* t1 = (D + z3). */
		fb_add(t1, t1, r->z);
		/* t0 = B^2. */
		fb_sqr(t0, t0);
		/* t0 = B^2 * z3. */
		fb_mul(t0, t0, r->z);
		/* y3 = (D + z3) * r3 + B^2 * z3. */
		fb_mul(r->y, t1, r->x);
		fb_add(r->y, r->y, t0);

		r->norm = 0;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t0);
		fb_free(t1);
	}
}
#endif /* EB_ORDIN || EB_KBLTZ */

#if defined(EB_SUPER)
/**
 * Doubles a point represented in projective coordinates on an supersingular
 * binary elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the point to double.
 */
void eb_dbl_projc_super(eb_t r, eb_t p) {
	/* x3 = x1^4 */
	fb_sqr(r->x, p->x);
	fb_sqr(r->x, r->x);
	/* y3 = y1^4. */
	fb_sqr(r->y, p->y);
	fb_sqr(r->y, r->y);

	if (!p->norm) {
		/* z3 = z1^4. */
		fb_sqr(r->z, p->z);
		fb_sqr(r->z, p->z);
		/* y3 = x1^4 + y1^4. */
		fb_add(r->y, r->x, r->y);
		/* x3 = x1^4 + z1^4. */
		fb_add(r->x, r->x, r->z);
	} else {
		/* y3 = x1^4 + y1^4. */
		fb_add(r->y, r->x, r->y);
		/* x3 = x1^4 + 1. */
		fb_add_dig(r->x, r->x, 1);
		/* r is still in affine coordinates. */
		fb_copy(r->z, p->z);
	}

	r->norm = p->norm;
}
#endif /* EB_SUPER */

#endif /* EB_ADD == PROJC */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EB_ADD == BASIC || defined(EB_MIXED) || !defined(STRIP)

void eb_dbl_basic(eb_t r, eb_t p) {
	if (eb_is_infty(p)) {
		eb_set_infty(r);
		return;
	}
#if defined(EB_SUPER)
	if (eb_curve_is_super()) {
		eb_dbl_basic_super(r, p);
		return;
	}
#endif

#if defined(EB_ORDIN)
	eb_dbl_basic_ordin(r, p);
#endif
}

#endif

#if EB_ADD == PROJC || !defined(STRIP)

void eb_dbl_projc(eb_t r, eb_t p) {

	if (eb_is_infty(p)) {
		eb_set_infty(r);
		return;
	}

#if defined(EB_SUPER)
	if (eb_curve_is_super()) {
		eb_dbl_projc_super(r, p);
		return;
	}
#endif

#if defined(EB_ORDIN)
	eb_dbl_projc_ordin(r, p);
#endif
}

#endif
