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
 * Implementation of the Miller doubling function.
 *
 * @version $Id$
 * @ingroup pp
 */

#include "relic_core.h"
#include "relic_pp.h"
#include "relic_pp_low.h"
#include "relic_util.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EP_ADD == BASIC || !defined(STRIP)

void pp_dbl_k12_basic(fp12_t l, ep2_t r, ep2_t q, ep_t p) {
	fp2_t s;
	fp2_t e;
	ep2_t t;

	fp2_null(s);
	fp2_null(e);
	ep2_null(t);

	TRY {
		fp2_new(s);
		fp2_new(e);
		ep2_new(t);
		ep2_copy(t, r);
		ep2_dbl_slp_basic(r, s, e, q);

		fp_mul(l[1][0][0], s[0], p->x);
		fp_mul(l[1][0][1], s[1], p->x);
		fp2_mul(l[1][1], s, t->x);
		fp2_sub(l[1][1], t->y, l[1][1]);
		fp_copy(l[0][0][0], p->y);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(s);
		fp2_free(e);
		ep2_free(t);
	}
}

#endif

#if EP_ADD == PROJC || !defined(STRIP)

#if PP_EXT == BASIC || !defined(STRIP)

void pp_dbl_k12_projc_basic(fp12_t l, ep2_t r, ep2_t q, ep_t p) {
	fp2_t t0, t1, t2, t3, t4, t5, t6;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);
	fp2_null(t3);
	fp2_null(t4);
	fp2_null(t5);
	fp2_null(t6);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);
		fp2_new(t4);
		fp2_new(t5);
		fp2_new(t6);

		if (ep_curve_opt_b() == OPT_TWO) {
			fp2_sqr(t0, q->x);
			fp2_sqr(t1, q->y);
			fp2_sqr(t2, q->z);
			fp2_mul(t4, q->x, q->y);
			fp_hlv(t4[0], t4[0]);
			fp_hlv(t4[1], t4[1]);
			fp2_dbl(t3, t2);
			fp2_add(t2, t2, t3);
			fp_add(t3[0], t2[0], t2[1]);
			fp_sub(t3[1], t2[1], t2[0]);
			fp2_dbl(t2, t3);
			fp2_add(t2, t3, t2);
			fp2_sub(r->x, t1, t2);
			fp2_mul(r->x, r->x, t4);

			fp2_add(t2, t1, t2);
			fp_hlv(t2[0], t2[0]);
			fp_hlv(t2[1], t2[1]);
			fp2_sqr(t2, t2);
			fp2_sqr(t4, t3);
			fp2_mul(t5, q->y, q->z);
			fp2_dbl(r->y, t4);
			fp2_add(r->y, r->y, t4);
			fp2_sub(r->y, t2, r->y);

			fp2_dbl(t2, t5);
			fp2_mul(r->z, t2, t1);

			fp2_sub(t3, t3, t1);
			fp2_mul_nor(l[0][0], t3);

			fp2_add(l[0][2], t0, t0);
			fp2_add(l[0][2], l[0][2], t0);
			fp_mul(l[0][2][0], l[0][2][0], p->x);
			fp_mul(l[0][2][1], l[0][2][1], p->x);

			fp_mul(l[1][1][0], t2[0], p->y);
			fp_mul(l[1][1][1], t2[1], p->y);
		} else {
			/* A = x1^2. */
			fp2_sqr(t0, q->x);
			/* B = y1^2. */
			fp2_sqr(t1, q->y);
			/* C = z1^2. */
			fp2_sqr(t2, q->z);
			/* D = 3bC. */
			// TODO: Optimize considering that B = 1 - i. //
			fp2_dbl(t3, t2);
			fp2_add(t3, t3, t2);
			ep2_curve_get_b(t4);
			fp2_mul(t3, t3, t4);
			/* E = (x1 + y1)^2 - A - B. */
			fp2_add(t4, q->x, q->y);
			fp2_sqr(t4, t4);
			fp2_sub(t4, t4, t0);
			fp2_sub(t4, t4, t1);

			/* F = (y1 + z1)^2 - B - C. */
			fp2_add(t5, q->y, q->z);
			fp2_sqr(t5, t5);
			fp2_sub(t5, t5, t1);
			fp2_sub(t5, t5, t2);

			/* G = 3D. */
			fp2_dbl(t6, t3);
			fp2_add(t6, t6, t3);

			/* x3 = E * (B - G). */
			fp2_sub(r->x, t1, t6);
			fp2_mul(r->x, r->x, t4);

			/* y3 = (B + G)^2 -12D^2. */
			fp2_add(t6, t6, t1);
			fp2_sqr(t6, t6);
			fp2_sqr(t2, t3);
			fp2_dbl(r->y, t2);
			fp2_dbl(t2, r->y);
			fp2_dbl(r->y, t2);
			fp2_add(r->y, r->y, t2);
			fp2_sub(r->y, t6, r->y);

			/* z3 = 4B * F. */
			fp2_dbl(r->z, t1);
			fp2_dbl(r->z, r->z);
			fp2_mul(r->z, r->z, t5);

			/* l00 = D - B. */
			fp2_sub(t3, t3, t1);
			fp2_mul_nor(l[0][0], t3);

			/* l10 = 3A * xp */
			fp2_add(l[0][2], t0, t0);
			fp2_add(l[0][2], l[0][2], t0);
			fp_mul(l[0][2][0], l[0][2][0], p->x);
			fp_mul(l[0][2][1], l[0][2][1], p->x);

			/* l01 = -F * yp. */
			fp_mul(l[1][1][0], t5[0], p->y);
			fp_mul(l[1][1][1], t5[1], p->y);
		}
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
		fp2_free(t4);
		fp2_free(t5);
		fp2_free(t6);
	}
}

#endif

#if PP_EXT == LAZYR || !defined(STRIP)

void pp_dbl_k12_projc_lazyr(fp12_t l, ep2_t r, ep2_t q, ep_t p) {
	fp2_t t0, t1, t2, t3, t4, t5, t6;
	dv2_t u0, u1;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);
	fp2_null(t3);
	fp2_null(t4);
	fp2_null(t5);
	fp2_null(t6);
	dv2_null(u0);
	dv2_null(u1);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);
		fp2_new(t4);
		fp2_new(t5);
		fp2_new(t6);
		dv2_new(u0);
		dv2_new(u1);

		if (ep_curve_opt_b() == OPT_TWO) {
			fp2_sqr(t0, q->z);
			fp2_sqr(t1, q->y);
			fp2_add(t5, t0, t1);
			fp2_dbl(t3, t0);
			fp2_add(t0, t0, t3);
			fp_add(t2[0], t0[0], t0[1]);
			fp_sub(t2[1], t0[1], t0[0]);

			fp2_sqr(t0, q->x);
			fp2_mul(t4, q->x, q->y);
			fp_hlv(t4[0], t4[0]);
			fp_hlv(t4[1], t4[1]);
			fp2_dbl(t3, t2);
			fp2_add(t3, t3, t2);
			fp2_sub(r->x, t1, t3);
			fp2_mul(r->x, r->x, t4);

			fp2_add(t3, t1, t3);
			fp_hlv(t3[0], t3[0]);
			fp_hlv(t3[1], t3[1]);
			fp2_sqrn_low(u0, t2);
			fp2_addd_low(u1, u0, u0);
			fp2_addd_low(u1, u1, u0);
			fp2_sqrn_low(u0, t3);
			fp2_subc_low(u0, u0, u1);

			fp2_add(t3, q->y, q->z);
			fp2_sqr(t3, t3);
			fp2_sub(t3, t3, t5);

			fp2_rdcn_low(r->y, u0);
			fp2_mul(r->z, t3, t1);

			fp2_sub(t2, t2, t1);
			fp2_mul_nor(l[0][0], t2);

			fp2_addn_low(l[0][2], t0, t0);
			fp2_addn_low(l[0][2], l[0][2], t0);
			fp_mul(l[0][2][0], l[0][2][0], p->x);
			fp_mul(l[0][2][1], l[0][2][1], p->x);

			fp_mul(l[1][1][0], t3[0], p->y);
			fp_mul(l[1][1][1], t3[1], p->y);
		} else {
			/* A = x1^2. */
			fp2_sqr(t0, q->x);
			/* B = y1^2. */
			fp2_sqr(t1, q->y);
			/* C = z1^2. */
			fp2_sqr(t2, q->z);
			/* D = 3bC. */
			// TODO: Optimize considering that B = 1 - i. //
			fp2_dbl(t3, t2);
			fp2_add(t3, t3, t2);
			ep2_curve_get_b(t4);
			fp2_mul(t3, t3, t4);
			/* E = (x1 + y1)^2 - A - B. */
			fp2_add(t4, q->x, q->y);
			fp2_sqr(t4, t4);
			fp2_sub(t4, t4, t0);
			fp2_sub(t4, t4, t1);

			/* F = (y1 + z1)^2 - B - C. */
			fp2_add(t5, q->y, q->z);
			fp2_sqr(t5, t5);
			fp2_sub(t5, t5, t1);
			fp2_sub(t5, t5, t2);

			/* G = 3D. */
			fp2_dbl(t6, t3);
			fp2_add(t6, t6, t3);

			/* x3 = E * (B - G). */
			fp2_sub(r->x, t1, t6);
			fp2_mul(r->x, r->x, t4);

			/* y3 = (B + G)^2 -12D^2. */
			fp2_add(t6, t6, t1);
			fp2_sqr(t6, t6);
			fp2_sqr(t2, t3);
			fp2_dbl(r->y, t2);
			fp2_dbl(t2, r->y);
			fp2_dbl(r->y, t2);
			fp2_add(r->y, r->y, t2);
			fp2_sub(r->y, t6, r->y);

			/* z3 = 4B * F. */
			fp2_dbl(r->z, t1);
			fp2_dbl(r->z, r->z);
			fp2_mul(r->z, r->z, t5);

			/* l00 = D - B. */
			fp2_sub(t3, t3, t1);
			fp2_mul_nor(l[0][0], t3);

			/* l10 = 3A * xp */
			fp2_add(l[0][2], t0, t0);
			fp2_add(l[0][2], l[0][2], t0);
			fp_mul(l[0][2][0], l[0][2][0], p->x);
			fp_mul(l[0][2][1], l[0][2][1], p->x);

			/* l01 = -F * yp. */
			fp_mul(l[1][1][0], t5[0], p->y);
			fp_mul(l[1][1][1], t5[1], p->y);
		}
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
		fp2_free(t4);
		fp2_free(t5);
		fp2_free(t6);
		dv2_free(u0);
		dv2_free(u1);
	}
}

#endif

#endif

void pp_dbl_k12_lit(fp12_t l, ep_t r, ep_t p, ep2_t q) {
	fp_t t0, t1, t2, t3, t4, t5, t6;

	fp_null(t0);
	fp_null(t1);
	fp_null(t2);
	fp_null(t3);
	fp_null(t4);
	fp_null(t5);
	fp_null(t6);

	TRY {
		fp_new(t0);
		fp_new(t1);
		fp_new(t2);
		fp_new(t3);
		fp_new(t4);
		fp_new(t5);
		fp_new(t6);

		fp_sqr(t0, p->x);
		fp_sqr(t1, p->y);
		fp_sqr(t2, p->z);

		fp_mul(t4, ep_curve_get_b(), t2);

		fp_dbl(t3, t4);
		fp_add(t3, t3, t4);

		fp_add(t4, p->x, p->y);
		fp_sqr(t4, t4);
		fp_sub(t4, t4, t0);
		fp_sub(t4, t4, t1);
		fp_add(t5, p->y, p->z);
		fp_sqr(t5, t5);
		fp_sub(t5, t5, t1);
		fp_sub(t5, t5, t2);
		fp_dbl(t6, t3);
		fp_add(t6, t6, t3);
		fp_sub(r->x, t1, t6);
		fp_mul(r->x, r->x, t4);
		fp_add(r->y, t1, t6);
		fp_sqr(r->y, r->y);
		fp_sqr(t4, t3);
		fp_dbl(t6, t4);
		fp_add(t6, t6, t4);
		fp_dbl(t6, t6);
		fp_dbl(t6, t6);
		fp_sub(r->y, r->y, t6);
		fp_mul(r->z, t1, t5);
		fp_dbl(r->z, r->z);
		fp_dbl(r->z, r->z);
		r->norm = 0;

		fp12_zero(l);
		if (ep2_curve_is_twist() == EP_DTYPE) {
			fp2_dbl(l[0][1], q->x);
			fp2_add(l[0][1], l[0][1], q->x);
			fp_mul(l[0][1][0], l[0][1][0], t0);
			fp_mul(l[0][1][1], l[0][1][1], t0);

			fp_sub(l[0][0][0], t3, t1);

			fp_neg(t5, t5);
			fp_mul(l[1][1][0], q->y[0], t5);
			fp_mul(l[1][1][1], q->y[1], t5);
		} else {
			fp2_dbl(l[0][2], q->x);
			fp2_add(l[0][2], l[0][2], q->x);
			fp_mul(l[0][2][0], l[0][2][0], t0);
			fp_mul(l[0][2][1], l[0][2][1], t0);

			fp_sub(l[0][0][0], t3, t1);

			fp_neg(t5, t5);
			fp_mul(l[1][1][0], q->y[0], t5);
			fp_mul(l[1][1][1], q->y[1], t5);

			fp2_t t;
			fp2_copy(t, l[0][0]);
			fp2_mul_nor(l[0][0], t);
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp_free(t0);
		fp_free(t1);
		fp_free(t2);
		fp_free(t3);
		fp_free(t4);
		fp_free(t5);
		fp_free(t6);
	}
}
