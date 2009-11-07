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
 * Implementation of the point addition on prime elliptic curves.
 *
 * @version $Id$
 * @ingroup ep
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
 * Adds two points represented in affine coordinates on an ordinary prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the first point to add.
 * @param q					- the second point to add.
 */
static void ep2_add_basic_ordin(ep2_t r, fp2_t s, ep2_t p, ep2_t q) {
	fp2_t t0, t1, t2;

	fp2_new(t0);
	fp2_new(t1);
	fp2_new(t2);
#if 0
	/* t0 = x2 - x1. */
	fp2_sub(t0, q->x, p->x);
	/* t1 = y2 - y1. */
	fp2_sub(t1, q->y, p->y);

	/* If t0 is zero. */
	if (fp2_is_zero(t0)) {
		if (fp2_is_zero(t1)) {
			/* If t1 is zero, q = p, should have doubled. */
			ep2_dbl_basic(r, p);
			return;
		} else {
			/* If t1 is not zero and t0 is zero, q = -p and r = infinity. */
			ep2_set_infinity(r);
			return;
		}
	}

	/* t2 = 1/(x2 - x1). */
	fp2_inv(t2, t0);
	/* t2 = lambda = (y2 - y1)/(x2 - x1). */
	fp2_mul(t2, t1, t2);

	/* x3 = lambda^2 - x2 - x1. */
	fp2_sqr(t1, t2);
	fp2_sub(t0, t1, p->x);
	fp2_sub(t0, t0, q->x);

	/* y3 = lambda * (x1 - x3) - y1. */
	fp2_sub(t1, p->x, t0);
	fp2_mul(t1, t2, t1);
	fp2_sub(r->y, t1, p->y);

	fp2_copy(r->x, t0);
	fp2_copy(r->z, p->z);

	if (s != NULL) {
		fp2_copy(s, t2);
	}

	r->norm = 1;
#endif
	fp2_t t3, t4;

	fp2_new(t3);
	fp2_new(t4);

    /*zzn2_sqr(_MIPP_ &(P->z),&t1);
    zzn2_mul(_MIPP_ &t3,&t1,&t3);
    zzn2_mul(_MIPP_ &t1,&(P->z),&t1);
    zzn2_mul(_MIPP_ &Yzzz,&t1,&Yzzz);

    zzn2_sub(_MIPP_ &t3,&(P->x),&t3);
    zzn2_sub(_MIPP_ &Yzzz,&(P->y),lam);
    if (P->marker==MR_EPOINT_NORMALIZED) zzn2_copy(&t3,&(P->z));
    else zzn2_mul(_MIPP_ &(P->z),&t3,&(P->z));
    zzn2_sqr(_MIPP_ &t3,&t1);
    zzn2_mul(_MIPP_ &t1,&t3,&Yzzz);
    zzn2_mul(_MIPP_ &t1,&(P->x),&t1);
    zzn2_copy(&t1,&t3);
    zzn2_add(_MIPP_ &t3,&t3,&t3);
    zzn2_sqr(_MIPP_ lam,&(P->x));
    zzn2_sub(_MIPP_ &(P->x),&t3,&(P->x));
    zzn2_sub(_MIPP_ &(P->x),&Yzzz,&(P->x));
    zzn2_sub(_MIPP_ &t1,&(P->x),&t1);
    zzn2_mul(_MIPP_ &t1,lam,&t1);
    zzn2_mul(_MIPP_ &Yzzz,&(P->y),&Yzzz);
    zzn2_sub(_MIPP_ &t1,&Yzzz,&(P->y));*/

	fp2_copy(t3, q->x);
	fp2_copy(t4, q->y);
	fp2_sqr(t1, p->z);
	fp2_mul(t3, t3, t1);
	fp2_mul(t1, t1, p->z);
	fp2_mul(t4, t4, t1);

	fp2_sub(t3, t3, p->x);
	fp2_sub(t0, t4, p->y);
	fp2_mul(r->z, p->z, t3);
	fp2_sqr(t1, t3);
	fp2_mul(t4, t1, t3);
	fp2_mul(t1, t1, p->x);
	fp2_copy(t3, t1);
	fp2_add(t3, t3, t3);
	fp2_sqr(r->x, t0);
	fp2_sub(r->x, r->x, t3);
	fp2_sub(r->x, r->x, t4);
	fp2_sub(t1, t1, r->x);
	fp2_mul(t1, t1, t0);
	fp2_mul(t4, t4, p->y);
	fp2_sub(r->y, t1, t4);

	if (s != NULL) {
		fp2_copy(s, t0);
	}

	fp2_free(t0);
	fp2_free(t1);
	fp2_free(t2);
}

#endif /* EP_ORDIN */

#endif /* EP_ADD == BASIC || EP_MIXED */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EP_ADD == BASIC || defined(EP_MIXED) || !defined(STRIP)

void ep2_add_basic(ep2_t r, ep2_t p, ep2_t q) {
	if (ep2_is_infinity(p)) {
		ep2_copy(r, q);
		return;
	}

	if (ep2_is_infinity(q)) {
		ep2_copy(r, p);
		return;
	}

#if defined(EP_ORDIN)
	ep2_add_basic_ordin(r, NULL, p, q);
#endif
}

void ep2_add_basic_slope(ep2_t r, fp2_t s, ep2_t p, ep2_t q) {
	if (ep2_is_infinity(p)) {
		ep2_copy(r, q);
		return;
	}

	if (ep2_is_infinity(q)) {
		ep2_copy(r, p);
		return;
	}

#if defined(EP_ORDIN)
	ep2_add_basic_ordin(r, s, p, q);
#endif
}

#endif

void ep2_mul(ep2_t r, ep2_t p, bn_t k) {
	int i, l;
	ep2_t t;

	ep2_new(t);
	l = bn_bits(k);

	if (bn_test_bit(k, l - 1)) {
		ep2_copy(t, p);
	} else {
		ep2_set_infinity(t);
	}

	for (i = l - 2; i >= 0; i--) {
		ep2_dbl(t, t);
		if (bn_test_bit(k, i)) {
			ep2_add(t, t, p);
		}
	}

	ep2_copy(r, t);
	ep2_norm(r, r);

	ep_free(t);
}
