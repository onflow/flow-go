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
 * Implementation of the point addition on prime elliptic curves.
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
 * Adds two points represented in affine coordinates on an ordinary prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the first point to add.
 * @param q					- the second point to add.
 */
static void ep_add_basic_ordin(ep_t r, ep_t p, ep_t q) {
	fp_t t0 = NULL, t1 = NULL, t2 = NULL;

	TRY {
		fp_new(t0);
		fp_new(t1);
		fp_new(t2);

		/* t0 = x2 - x1. */
		fp_sub(t0, q->x, p->x);
		/* t1 = y2 - y1. */
		fp_sub(t1, q->y, p->y);

		/* If t0 is zero. */
		if (fp_is_zero(t0)) {
			if (fp_is_zero(t1)) {
				/* If t1 is zero, q = p, should have doubled. */
				ep_dbl_basic(r, p);
				return;
			} else {
				/* If t1 is not zero and t0 is zero, q = -p and r = infinity. */
				ep_set_infty(r);
				return;
			}
		}

		/* t2 = 1/(x2 - x1). */
		fp_inv(t2, t0);
		/* t2 = lambda = (y2 - y1)/(x2 - x1). */
		fp_mul(t2, t1, t2);

		/* x3 = lambda^2 - x2 - x1. */
		fp_sqr(t1, t2);
		fp_sub(t0, t1, p->x);
		fp_sub(t0, t0, q->x);

		/* y3 = lambda * (x1 - x3) - y1. */
		fp_sub(t1, p->x, t0);
		fp_mul(t1, t2, t1);
		fp_sub(r->y, t1, p->y);

		fp_copy(r->x, t0);
		fp_copy(r->z, p->z);

		r->norm = 1;

		fp_free(t0);
		fp_free(t1);
		fp_free(t2);
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
 * Adds two points represented in affine coordinates on a supersingular prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the first point to add.
 * @param q					- the second point to add.
 */
static void ep_add_basic_super(ep_t r, ep_t p, ep_t q) {
	fp_t t0 = NULL, t1 = NULL, t2 = NULL;

	TRY {
		fp_new(t0);
		fp_new(t1);
		fp_new(t2);

		/* t0 = (y1 + y2). */
		fp_add(t0, p->y, q->y);
		/* t1 = (x1 + x2). */
		fp_add(t1, p->x, q->x);

		if (fp_is_zero(t1)) {
			if (fp_is_zero(t0)) {
				/* If t1 is zero and t0 is zero, p = q, should have doubled. */
				ep_dbl_basic(r, p);
				return;
			} else {
				/* If t0 is not zero and t1 is zero, q = -p and r = infinity. */
				ep_set_infty(r);
				return;
			}
		}

		/* t2 = 1/(x1 + x2). */
		fp_inv(t2, t1);
		/* t0 = lambda = (y1 + y2)/(x1 + x2). */
		fp_mul(t0, t0, t2);
		/* t2 = lambda^2. */
		fp_sqr(t2, t0);

		/* t2 = lambda^2 + x1 + x2. */
		fp_add(t2, t2, t1);

		/* y3 = lambda*(x3 + x1) + y1 + c. */
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

		/* x3 = lambda^2 + x1 + x2. */
		fp_copy(r->x, t2);
		fp_copy(r->z, p->z);

		fp_free(t0);
		fp_free(t1);
		fp_free(t2);

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
#endif /* EP_ADD == BASIC || EP_MIXED */

#if EP_ADD == PROJC || defined(EP_MIXED) || !defined(STRIP)

#if defined(EP_ORDIN) || defined(EP_KBLTZ)
/**
 * Adds two points represented in projective coordinates on an ordinary prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the first point to add.
 * @param q					- the second point to add.
 */
void ep_add_projc_ordin(ep_t r, ep_t p, ep_t q) {
	fp_t t0, t1, t2, t3, t4, t5, t6;

	fp_new(t0);
	fp_new(t1);
	fp_new(t2);
	fp_new(t3);
	fp_new(t4);
	fp_new(t5);
	fp_new(t6);

	if (!q->norm) {
		/* t0 = B = x2 * z1. */
		fp_mul(t0, q->x, p->z);

		/* A = x1 * z2 */
		fp_mul(r->x, p->x, q->z);

		/* t1 = E = A + B. */
		fp_add(t1, r->x, t0);

		/* t2 = D = B^2. */
		fp_sqr(t2, t0);
		/* t3 = C = A^2. */
		fp_sqr(t3, r->x);
		/* t4 = F = C + D. */
		fp_add(t4, t2, t3);

		/* t5 = H = y2 * z1^2. */
		fp_sqr(t5, p->z);
		fp_mul(t5, t5, q->y);

		/* t6 = G = y1 * z2^2. */
		fp_sqr(t6, q->z);
		fp_mul(t6, t6, p->y);

		/* t2 = D + H. */
		fp_add(t2, t2, t5);
		/* t3 = C + G. */
		fp_add(t3, t3, t6);
		/* t5 = I = G + H. */
		fp_add(t5, t6, t5);

		/* If E is zero. */
		if (fp_is_zero(t1)) {
			if (fp_is_zero(t5)) {
				/* If I is zero, p = q, should have doubled. */
				ep_dbl_projc(r, p);
				return;
			} else {
				/* If I is not zero, q = -p, r = infinity. */
				ep_set_infty(r);
				return;
			}
		}
		/* t5 = J = I * E. */
		fp_mul(t5, t5, t1);

		/* z3 = F * z1 * z2. */
		fp_mul(r->z, p->z, q->z);
		fp_mul(r->z, t4, r->z);

		/* t3 = B * (C + G). */
		fp_mul(t3, t0, t3);
		/* t1 = A * J. */
		fp_mul(t1, r->x, t5);
		/* x3 = A * (D + H) + B * (C + G). */
		fp_mul(r->x, r->x, t2);
		fp_add(r->x, r->x, t3);

		/* t6 = F * G. */
		fp_mul(t6, t6, t4);
		/* Y3 = (A * J + F * G) * F + (J + z3) * x3. */
		fp_add(r->y, t1, t6);
		fp_mul(r->y, r->y, t4);
		/* t6 = (J + z3) * x3. */
		fp_add(t6, t5, r->z);
		fp_mul(t6, t6, r->x);
		fp_add(r->y, r->y, t6);
	} else {
		/* Mixed addition. */
		if (!p->norm) {
			/* A = y1 + y2 * z1^2. */
			fp_sqr(t0, p->z);
			fp_mul(t0, t0, q->y);
			fp_add(t0, t0, p->y);
			/* B = x1 + x2 * z1. */
			fp_mul(t1, p->z, q->x);
			fp_add(t1, t1, p->x);
		} else {
			/* t0 = A = y1 + y2. */
			fp_add(t0, p->y, q->y);
			/* t1 = B = x1 + x2. */
			fp_add(t1, p->x, q->x);
		}

		if (fp_is_zero(t1)) {
			if (fp_is_zero(t0)) {
				/* If t0 = 0 and t1 = 0, p = q, should have doubled! */
				ep_dbl_projc(r, p);
				return;
			} else {
				/* If t0 = 0, r is infinity. */
				ep_set_infty(r);
				return;
			}
		}

		if (!p->norm) {
			/* t2 = C = B * z1. */
			fp_mul(t2, p->z, t1);
			/* z3 = C^2. */
			fp_sqr(r->z, t2);
			/* t1 = B^2. */
			fp_sqr(t1, t1);
			/* t1 = A + B^2. */
			fp_add(t1, t0, t1);
		} else {
			/* If z1 = 0, t2 = C = B. */
			fp_copy(t2, t1);
			/* z3 = B^2. */
			fp_sqr(r->z, t1);
			/* t1 = A + z3. */
			fp_add(t1, t0, r->z);
		}

		/* t3 = D = x2 * z3. */
		fp_mul(t3, r->z, q->x);

		/* t4 = (y2 + x2). */
		fp_add(t4, q->x, q->y);

		/* z3 = A^2. */
		fp_sqr(r->x, t0);

		/* t1 = A + B^2 + a2 * C. */
		switch (ep_curve_opt_a()) {
			case EP_OPT_ZERO:
				break;
			case EP_OPT_ONE:
				fp_add(t1, t1, t2);
				break;
			case EP_OPT_DIGIT:
				/* t5 = a2 * C. */
				fp_mul(t5, ep_curve_get_a(), t2);
				fp_add(t1, t1, t5);
				break;
			default:
				/* t5 = a2 * C. */
				fp_mul(t5, ep_curve_get_a(), t2);
				fp_add(t1, t1, t5);
				break;
		}
		/* t1 = C * (A + B^2 + a2 * C). */
		fp_mul(t1, t1, t2);
		/* x3 = A^2 + C * (A + B^2 + a2 * C). */
		fp_add(r->x, r->x, t1);

		/* t3 = D + x3. */
		fp_add(t3, t3, r->x);
		/* t2 = A * B. */
		fp_mul(t2, t0, t2);
		/* y3 = (D + x3) * (A * B + z3). */
		fp_add(r->y, t2, r->z);
		fp_mul(r->y, r->y, t3);
		/* t0 = z3^2. */
		fp_sqr(t0, r->z);
		/* t0 = (y2 + x2) * z3^2. */
		fp_mul(t0, t0, t4);
		/* y3 = (D + x3) * (A * B + z3) + (y2 + x2) * z3^2. */
		fp_add(r->y, r->y, t0);
	}

	r->norm = 0;

	fp_free(t0);
	fp_free(t1);
	fp_free(t2);
	fp_free(t3);
	fp_free(t4);
	fp_free(t5);
	fp_free(t6);
}

#endif /* EP_ORDIN */
/**
 * Adds two points represented in projective coordinates on a supersingular
 * prime elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the first point to add.
 * @param q					- the second point to add.
 */
#if defined(EP_SUPER)
void ep_add_projc_super(ep_t r, ep_t p, ep_t q) {
	fp_t t0, t1, t2, t3, t4, t5, t6;

	fp_new(t0);
	fp_new(t1);
	fp_new(t2);
	fp_new(t3);
	fp_new(t4);
	fp_new(t5);

	if (!q->norm) {
		/* t0 = A = y2 * z1. */
		fp_mul(t0, p->z, q->y);
		/* t1 = B = y1 * z2. */
		fp_mul(t1, p->y, q->z);
		/* t0 = E = A + B */
		fp_add(t0, t0, t1);

		/* t2 = C = x1 * z2. */
		fp_mul(t2, p->x, q->z);
		/* t3 = D = x2 * z1. */
		fp_mul(t3, p->z, q->x);
		/* t1 = F = C + D */
		fp_add(t2, t2, t3);

		/* If F is zero. */
		if (fp_is_zero(t2)) {
			if (fp_is_zero(t0)) {
				/* If E is zero, p = q, should have doubled. */
				ep_dbl_projc(r, p);
				return;
			} else {
				/* If E is not zero, q = -p, r = infinity. */
				ep_set_infty(r);
				return;
			}
		}
		/* t4 = z1 * z2. */
		fp_mul(t4, p->z, q->z);

		/* x3 = F^2. */
		fp_sqr(r->x, t2);
		/* z3 = F^3. */
		fp_mul(r->z, r->x, t2);

		/* y3 = x2 * z1 * F^2. */
		fp_mul(r->y, t3, r->x);
		/* t5 = z1 * z2 * E^2. */
		fp_sqr(t5, t0);
		fp_mul(t5, t5, t4);
		/* y3 = E * (x2 * z1 * F^2 + z1 * z2 * E^2). */
		fp_add(r->y, r->y, t5);
		fp_mul(r->y, r->y, t0);

		/* x3 = F^4. */
		fp_sqr(r->x, r->x);

		/* t5 = F * z1 * z2 * E^2. */
		fp_mul(t5, t2, t5);

		/* x3 = F^4 * z1 * z2 * E^2. */
		fp_add(r->x, r->x, t5);

		/* t5 = y1 * z2 * F^3. */
		fp_mul(t5, r->z, t1);

		/* z3 = z1 * z2 * F^3. */
		fp_mul(r->z, r->z, t4);

		/* y3 = y1 * z2 * F^3 + z3. */
		fp_add(r->y, r->y, t5);
		fp_add(r->y, r->y, r->z);
	} else {
		/* Mixed addition. */
		if (p->norm) {
			/* A = y2, B = y1, E = y1 + y2. */
			fp_add(t0, p->y, q->y);
			/* C = x1, D = x2, E = x1 + x2. */
			fp_add(t2, p->x, q->x);
		} else {
			/* t0 = A = y2 * z1. */
			fp_mul(t0, q->y, p->z);
			/* t0 = E = A + y1 */
			fp_add(t0, t0, p->y);
			/* t3 = D = x2 * z1. */
			fp_mul(t3, p->z, q->x);
			/* t1 = F = x1 + D */
			fp_add(t2, p->x, t3);
		}

		/* If F is zero. */
		if (fp_is_zero(t2)) {
			if (fp_is_zero(t0)) {
				/* If E is zero, p = q, should have doubled. */
				ep_dbl_projc(r, p);
				return;
			} else {
				/* If E is not zero, q = -p, r = infinity. */
				ep_set_infty(r);
				return;
			}
		}

		if (p->norm) {
			/* t3 = F^2. */
			fp_sqr(t3, t2);
			/* t4 = F^3. */
			fp_mul(t4, t3, t2);
			/* y3 = x2 * z1 * F^2. */
			fp_mul(t1, q->x, t3);
			/* x3 = F^4. */
			fp_sqr(r->x, t3);
		} else {
			/* x3 = F^2. */
			fp_sqr(r->x, t2);
			/* t4 = F^3. */
			fp_mul(t4, r->x, t2);
			/* y3 = x2 * z1 * F^2. */
			fp_mul(t1, t3, r->x);
			/* x3 = F^4. */
			fp_sqr(r->x, r->x);
		}

		/* t5 = z1 * z2 * E^2. */
		fp_sqr(t5, t0);
		if (!p->norm) {
			fp_mul(t5, t5, p->z);
			/* z3 = z1 * z2 * F^3. */
			fp_mul(r->z, t4, p->z);
		} else {
			fp_copy(r->z, t4);
		}

		/* y3 = E * (x2 * z1 * F^2 + z1 * z2 * E^2). */
		fp_add(t1, t1, t5);
		fp_mul(t1, t1, t0);

		/* t5 = F * z1 * z2 * E^2. */
		fp_mul(t5, t2, t5);

		/* x3 = F^4 * z1 * z2 * E^2. */
		fp_add(r->x, r->x, t5);

		/* t5 = y1 * z2 * F^3. */
		fp_mul(t5, t4, p->y);

		/* y3 = y1 * z2 * F^3 + z3. */
		fp_add(r->y, t1, t5);
		fp_add(r->y, r->y, r->z);
	}
	r->norm = 0;

	fp_free(t0);
	fp_free(t1);
	fp_free(t2);
	fp_free(t3);
	fp_free(t4);
	fp_free(t5);
}
#endif /* EP_SUPER */
#endif /* EP_ADD == PROJC || EP_MIXED */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EP_ADD == BASIC || defined(EP_MIXED) || !defined(STRIP)

void ep_add_basic(ep_t r, ep_t p, ep_t q) {
	if (ep_is_infty(p)) {
		ep_copy(r, q);
		return;
	}

	if (ep_is_infty(q)) {
		ep_copy(r, p);
		return;
	}
#if defined(EP_SUPER)
	if (ep_curve_is_super()) {
		ep_add_basic_super(r, p, q);
		return;
	}
#endif

#if defined(EP_ORDIN) || defined(EP_KBLTZ)
	ep_add_basic_ordin(r, p, q);
#endif
}

#endif

#if EP_ADD == PROJC || defined(EP_MIXED) || !defined(STRIP)

void ep_add_projc(ep_t r, ep_t p, ep_t q) {
	if (ep_is_infty(p)) {
		ep_copy(r, q);
		return;
	}

	if (ep_is_infty(q)) {
		ep_copy(r, p);
		return;
	}
#if defined(EP_SUPER)
	if (ep_curve_is_super()) {
		ep_add_projc_super(r, p, q);
		return;
	}
#endif

#if defined(EP_ORDIN) || defined(EP_KBLTZ)
	ep_add_projc_ordin(r, p, q);
#endif
}

#endif

#if EP_ADD == BASIC || defined(EP_MIXED) || !defined(STRIP)

void ep_sub_basic(ep_t r, ep_t p, ep_t q) {
	ep_t t;

	ep_new(t);

	if (p == q) {
		ep_set_infty(r);
		return;
	}

	ep_neg_basic(t, q);
	ep_add_basic(r, p, t);

	ep_free(t);

	r->norm = 1;
}

#endif

#if EP_ADD == PROJC || defined(EP_MIXED) || !defined(STRIP)

/*void ep_sub_projc(ep_t r, ep_t p, ep_t q) {
 * RLC_PROLOGUE;
 * ep_t t;
 *
 * if (p == q) {
 * ep_set_infinity(r);
 * RLC_EPILOGUE;
 * return;
 * }
 *
 * ep_new(t);
 *
 * ep_neg_projc(t, q);
 * ep_add_projc(r, p, t);
 *
 * ep_free(t);
 *
 * r->norm = 0;
 *
 * RLC_EPILOGUE;
 * } */

#endif
