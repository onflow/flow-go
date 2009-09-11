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
 * @ingroup ep2
 */

#include "string.h"

#include "relic_core.h"
#include "relic_ep.h"
#include "relic_ep2.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if EP_ADD == BASIC || defined(EP_MIXED) || !defined(STRIP)

#if defined(EP_ORDIN)
/**
 * Doubles a point rep2resented in affine coordinates on an ordinary prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the point to double.
 */
static void ep2_dbl_basic_ordin(ep2_t r, fp2_t s, fp2_t e, ep2_t p) {
	fp2_t t0, t1, t2;

	fp2_new(t0);
	fp2_new(t1);
	fp2_new(t2);

#if 0
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
			case EP_OPT_ZERO:
				break;
			case EP_OPT_ONE:
				fp2_set_dig(t2, 1);
				fp2_mul_poly(t2, t2);
				fp2_mul_poly(t2, t2);
				fp2_add(t1, t1, t2);
				break;
			case EP_OPT_DIGIT:
				fp2_set_dig(t2, ep_curve_get_a()[0]);
				fp2_mul_poly(t2, t2);
				fp2_mul_poly(t2, t2);
				fp2_add(t1, t1, t2);
				break;
			default:
				fp_copy(t2, ep_curve_get_a());
				fp_zero(t2);
				fp2_mul_poly(t2, t2);
				fp2_mul_poly(t2, t2);
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
#endif
	fp2_t t3;
	fp2_new(t3);

    /*zzn2_sqr(_MIPP_ &(P->y),&t3);
    zzn2_sqr(_MIPP_ &(P->x),lam);
    zzn2_add(_MIPP_ lam,lam,&t2);
    zzn2_add(_MIPP_ lam,&t2,lam);

    zzn2_mul(_MIPP_ &(P->x),&t3,&t1);
    zzn2_add(_MIPP_ &t1,&t1,&t1);
    zzn2_add(_MIPP_ &t1,&t1,&t1);
    zzn2_sqr(_MIPP_ lam,&(P->x));
    zzn2_add(_MIPP_ &t1,&t1,&t2);
    zzn2_sub(_MIPP_ &(P->x),&t2,&(P->x));
    if (P->marker==MR_EPOINT_NORMALIZED) zzn2_copy(&(P->y),&(P->z));
    else zzn2_mul(_MIPP_ &(P->z),&(P->y),&(P->z));
    zzn2_add(_MIPP_ &(P->z),&(P->z),&(P->z));
    zzn2_add(_MIPP_ &t3,&t3,&t3);
    if (ex1!=NULL) zzn2_copy(&t3,ex1);
    zzn2_sqr(_MIPP_ &t3,&t3);
    zzn2_add(_MIPP_ &t3,&t3,&t3);
    zzn2_sub(_MIPP_ &t1,&(P->x),&t1);
    zzn2_mul(_MIPP_ lam,&t1,&(P->y));
    zzn2_sub(_MIPP_ &(P->y),&t3,&(P->y));*/

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
		fp2_copy(e, t3)
	}
	fp2_sqr(t3, t3);
	fp2_add(t3, t3, t3);
	fp2_sub(t1, t1, r->x);
	fp2_mul(r->y, t0, t1);
	fp2_sub(r->y, r->y, t3);

	//ep2_norm(r, r);

	r->norm = 0;

	fp2_free(t0);
	fp2_free(t1);
	fp2_free(t2);
}
#endif /* EP_ORDIN */

#endif /* EP_ADD == BASIC */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EP_ADD == BASIC || defined(EP_MIXED) || !defined(STRIP)

void ep2_dbl_basic(ep2_t r, ep2_t p) {
	if (ep2_is_infinity(p)) {
		ep2_set_infinity(r);
		return;
	}

#if defined(EP_SUPER)
	if (ep2_curve_is_super()) {
		ep2_dbl_basic_super(r, p);
		return;
	}
#endif

#if defined(EP_ORDIN)
	ep2_dbl_basic_ordin(r, NULL, NULL, p);
#endif
}

void ep2_dbl_basic_slope(ep2_t r, fp2_t s, fp2_t e, ep2_t p) {
	if (ep2_is_infinity(p)) {
		ep2_set_infinity(r);
		return;
	}

#if defined(EP_ORDIN)
	ep2_dbl_basic_ordin(r, s, e, p);
#endif
}

#endif
