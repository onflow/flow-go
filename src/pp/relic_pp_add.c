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
 * along with RELIC. If not, see <hup://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of the Miller addition function.
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

void pp_add_basic(fp12_t l, ep2_t r, ep2_t q, ep_t p) {
	fp2_t s;
	ep2_t t;

	fp2_null(s);
	ep2_null(t);

	TRY {
		fp2_new(s);
		ep2_new(t);

		ep2_copy(t, r);
		ep2_add_slp_basic(r, s, r, q);

		fp_mul(l[1][0][0], s[0], p->x);
		fp_mul(l[1][0][1], s[1], p->x);
		fp2_mul(l[1][1], s, t->x);
		fp2_sub(l[1][1], t->y, l[1][1]);
		fp_neg(l[0][0][0], p->y);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(s);
		ep2_free(t);
	}
}

#endif

#if EP_ADD == PROJC || !defined(STRIP)

#if PP_EXT == BASIC || !defined(STRIP)

void pp_add_projc_basic(fp12_t l, ep2_t r, ep2_t q, ep_t p) {
	fp2_t t0, t1, t2, t3, t4;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);
	fp2_null(t3);
	fp2_null(t4);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);
		fp2_new(t4);

		fp2_mul(t0, r->z, q->x);
		fp2_sub(t0, r->x, t0);
		fp2_mul(t1, r->z, q->y);
		fp2_sub(t1, r->y, t1);

		fp2_sqr(t2, t0);
		fp2_mul(r->x, t2, r->x);
		fp2_mul(t2, t0, t2);
		fp2_sqr(t3, t1);
		fp2_mul(t3, t3, r->z);
		fp2_add(t3, t2, t3);

		fp_mul(l[0][2][0], t1[0], p->x);
		fp_mul(l[0][2][1], t1[1], p->x);
		fp2_neg(l[0][2], l[0][2]);

		/* l00 = x2 * t2 - y2 * t1. */
		fp2_mul(t4, q->x, t1);

		fp2_sub(t3, t3, r->x);
		fp2_sub(t3, t3, r->x);
		fp2_sub(r->x, r->x, t3);
		fp2_mul(t1, t1, r->x);

		fp2_mul(r->y, t2, r->y);
		fp2_sub(r->y, t1, r->y);
		fp2_mul(r->x, t0, t3);
		fp2_mul(r->z, r->z, t2);

		fp2_mul(t2, q->y, t0);
		fp2_neg(t2, t2);
		fp2_add(t4, t4, t2);
		fp2_mul_nor(l[0][0], t4);

		/* l01 = t1 * yp. */
		fp_mul(l[1][1][0], t0[0], p->y);
		fp_mul(l[1][1][1], t0[1], p->y);

		r->norm = 0;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
		fp2_free(t3);
		fp2_free(t4);
	}
}

#endif

#if PP_EXT == LAZYR || !defined(STRIP)

void pp_add_projc_lazyr(fp12_t l, ep2_t r, ep2_t q, ep_t p) {
	fp2_t t1, t2, t3, t4;
	dv2_t u1, u2;

	fp2_null(t1);
	fp2_null(t2);
	fp2_null(t3);
	fp2_null(t4);
	dv2_null(u1);
	dv2_null(u2);

	TRY {
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);
		fp2_new(t4);
		dv2_new(u1);
		dv2_new(u2);

		fp2_mul(t1, r->z, q->x);
		fp2_sub(t1, r->x, t1);
		fp2_mul(t2, r->z, q->y);
		fp2_sub(t2, r->y, t2);

		fp2_sqr(t3, t1);
		fp2_mul(r->x, t3, r->x);
		fp2_mul(t3, t1, t3);
		fp2_sqr(t4, t2);
		fp2_mul(t4, t4, r->z);
		fp2_add(t4, t3, t4);

		fp2_sub(t4, t4, r->x);
		fp2_sub(t4, t4, r->x);
		fp2_sub(r->x, r->x, t4);
#ifdef FP_SPACE
		fp2_mulc_low(u1, t2, r->x);
		fp2_mulc_low(u2, t3, r->y);
#else
		fp2_muln_low(u1, t2, r->x);
		fp2_muln_low(u2, t3, r->y);
#endif
		fp2_subc_low(u2, u1, u2);
		fp2_rdcn_low(r->y, u2);
		fp2_mul(r->x, t1, t4);
		fp2_mul(r->z, r->z, t3);

		fp_mul(l[0][2][0], t2[0], p->x);
		fp_mul(l[0][2][1], t2[1], p->x);
		fp2_neg(l[0][2], l[0][2]);

#ifdef FP_SPACE
		fp2_mulc_low(u1, q->x, t2);
		fp2_mulc_low(u2, q->y, t1);
#else
		fp2_muln_low(u1, q->x, t2);
		fp2_muln_low(u2, q->y, t1);
#endif
		fp2_subc_low(u1, u1, u2);
		fp2_rdcn_low(t2, u1);
		fp2_mul_nor(l[0][0], t2);

		fp_mul(l[1][1][0], t1[0], p->y);
		fp_mul(l[1][1][1], t1[1], p->y);

		r->norm = 0;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t1);
		fp2_free(t2);
		fp2_free(t3);
		fp2_free(t4);
		dv2_free(u1);
		dv2_free(u2);
	}
}

#endif

#endif
