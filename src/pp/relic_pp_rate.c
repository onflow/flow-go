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
 * Implementation of the eta_t bilinear pairing.
 *
 * @version $Id$
 * @ingroup etat
 */

#include "relic_core.h"
#include "relic_pp.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Add two prime elliptic curve points and evaluates the corresponding line
 * function at another elliptic curve point.
 * 
 * @param[out] l			- the result of the evaluation.
 * @param[in,out] r			- the first point to add, in Affine coordinates.
 * 							The result of the addition, in Jacobian coordinates.
 * @param[in] q				- the second point to add, in Jacobian coordinates.
 * @param[in] p				- the point where the line function will be
 * evaluated, in Affine coordinates. 
 */
void rate_add(fp12_t l, ep2_t r, ep2_t q, ep_t p) {
	fp2_t slope;
	ep2_t t;

	fp2_new(slope);
	ep2_new(t);

	ep2_copy(t, r);
	ep2_add_slp(r, slope, r, q);

	if (ep2_is_infty(r)) {
		fp12_zero(l);
		fp_set_dig(l[0][0][0], 1);
	} else {
		fp6_t n, d;

		fp6_new(d);
		fp6_new(n);

		fp_zero(d[0][1]);
		fp_copy(d[0][0], p->x);
		fp2_mul(d[0], d[0], slope);
		fp2_neg(d[0], d[0]);
		fp2_mul(d[2], r->z, q->y);
		fp2_mul(d[1], slope, q->x);
		fp2_sub(d[1], d[1], d[2]);
		fp2_zero(d[2]);

		fp_zero(n[0][1]);
		fp_copy(n[0][0], p->y);
		fp2_mul(n[0], n[0], r->z);
		fp2_zero(n[1]);
		fp2_zero(n[2]);

		fp6_copy(l[0], n);
		fp6_copy(l[1], d);
	}

	fp2_free(slope);
	ep2_free(t);
}

/**
 * Double a prime elliptic curve point and evaluates the corresponding line
 * function at another elliptic curve point.
 * 
 * @param[out] l			- the result of the evaluation.
 * @param[out] r			- the result, in Jacobian coordinates.
 * @param[in] q				- the point to double, in Jacobian coordinates.
 * @param[in] p				- the point where the line function will be
 * evaluated, in Affine coordinates. 
 */
void rate_dbl(fp12_t l, ep2_t r, ep2_t q, ep_t p) {
	fp2_t s;
	fp2_t e;
	ep2_t t;

	fp2_new(s);
	fp2_new(e);
	ep2_new(t);
	ep2_copy(t, r);
	ep2_dbl_slp(r, s, e, r);

	if (ep2_is_infty(r)) {
		fp12_zero(l);
		fp_set_dig(l[0][0][0], 1);
	} else {
		fp6_t n, d;

		fp6_new(d);
		fp6_new(n);

		fp2_sqr(d[2], t->z);
		fp2_mul(d[0], d[2], s);
		fp_mul(d[0][0], d[0][0], p->x);
		fp_mul(d[0][1], d[0][1], p->x);
		fp2_neg(d[0], d[0]);
		fp2_mul(d[1], s, t->x);
		fp2_sub(d[1], d[1], e);

		fp2_mul(n[0], r->z, d[2]);
		fp_mul(n[0][0], n[0][0], p->y);
		fp_mul(n[0][1], n[0][1], p->y);
		fp2_zero(d[2]);
		fp2_zero(n[1]);
		fp2_zero(n[2]);

		fp6_copy(l[0], n);
		fp6_copy(l[1], d);
	}

	fp2_free(s);
	ep2_free(t);
}

/**
 * Compute Frob(a) = Frob((x,y)) = (x^p, y^p).
 * 
 * On G_2, this is the same as multiplying by p.   
 * 
 * @param[out] c			- the result in Jacobian coordinates.
 * @param[in] a				- a point Jacobian coordinates.
 * @param[in] f				- constant used in Frobenius, Z^p.
 */
static void ep2_frb(ep2_t c, ep2_t a, fp12_t f) {
	fp12_t t;

	fp12_new(t);

	fp12_sqr(t, f);

	fp2_conj(c->x, a->x);
	fp2_conj(c->y, a->y);
	fp2_conj(c->z, a->z);
	fp2_mul(c->x, c->x, t[0][1]);
	fp2_mul(c->y, c->y, t[0][1]);
	fp2_mul(c->y, c->y, f[1][0]);

	fp12_free(t);
}

/**
 * Compute the Miller loop for Ate variants over the bits of r.
 * 
 * @param[out] res			- the result.
 * @param[out] t			- the point r*q.
 * @param[in] q				- the first point of the pairing, in G_2.
 * @param[in] r				- the length of the loop.
 * @param[in] p				- the second point of the pairing, in G_1.
 */
void rate_miller(fp12_t res, ep2_t t, ep2_t q, bn_t r, ep_t p) {
	fp12_t tmp;
	unsigned int i;

	fp12_new(tmp);

	fp12_zero(res);
	fp_set_dig(res[0][0][0], 1);
	ep2_copy(t, q);

	for (i = bn_bits(r) - 1; i > 0; i--) {
		rate_dbl(tmp, t, t, p);
		fp12_sqr(res, res);
		fp12_mul_dexsp(res, res, tmp);

		if (bn_test_bit(r, i - 1)) {
			rate_add(tmp, t, q, p);
			fp12_mul_dexsp(res, res, tmp);
		}
	}
}

/**
 * Compute the additional multiplication required by the R-Ate pairing.
 * 
 * @param[in,out] res		- the field element output of the Miller loop.
 * 							The result of the additional multiplication.
 * @param[in] t				- the elliptic point output of the Miller loop.
 * @param[in] q				- the first point of the pairing, in G_2. 
 * @param[in] p				- the second point of the pairing, in G_1.
 * @param[in] f				- constant used in Frobenius, Z^p.
 */
void rate_mult(fp12_t res, ep2_t t, ep2_t q, ep_t p, fp12_t f) {
	ep2_t q1, r1q;
	fp12_t tmp1;
	fp12_t tmp2;
	ep2_new(q1);
	ep2_new(r1q);
	fp12_new(tmp1);
	fp12_new(tmp2);

	ep2_copy(r1q, t);

	rate_add(tmp1, r1q, q, p);
	fp12_mul_dexsp(tmp2, res, tmp1);
	fp12_frb(tmp2, tmp2, f);
	fp12_mul(res, res, tmp2);

	r1q->norm = 0;
	ep2_norm(r1q, r1q);

	ep2_frb(q1, r1q, f);

	ep2_copy(r1q, t);

	rate_add(tmp1, r1q, q1, p);
	fp12_mul_dexsp(res, res, tmp1);
}

/**
 * Compute the final exponentiation for BN curves.
 * 
 * @param[in,out] m			- the output of the additional pairing multiplication.
 * The result of the final exponentiation.
 * @param[in] x				- the BN parameter used to generate the curve.
 * @param[in] f				- constant used in Frobenius, Z^p.
 */
void rate_exp(fp12_t m, bn_t x, fp12_t f) {
	fp12_t v0;
	fp12_t v1;
	fp12_t v2;
	fp12_t v3;
	fp12_new(v0);
	fp12_new(v1);
	fp12_new(v2);
	fp12_new(v3);

	//First, m^(p^6 - 1)
	//tmp = m^(-1)
	//INVERSION
	fp12_inv(v0, m);
	//m' = m^(p^6)
	fp12_conj(m, m);
	//m' = m^(p^6 - 1)
	fp12_mul(m, m, v0);

	//Second, m^(p^2 + 1)
	//t = m^(p^2)
	fp12_frb(v0, m, f);
	fp12_frb(v0, v0, f);
	//m' = m^(p^2 + 1)
	fp12_mul(m, m, v0);

	//Third, m^((p^4 - p^2 + 1) / r)
	//From here on we work with x' = -x, therefore if x is positive
	//we need inversions
	if (bn_sign(x) == BN_POS) {
		//TODO: these inversions are probably just conjugations, test
		//INVERSION
		fp12_inv(v3, m);
		fp12_exp_uni(v0, v3, x);
		//INVERSION
		fp12_inv(v3, v0);
		fp12_exp_uni(v1, v3, x);
		//INVERSION
		fp12_inv(v3, v1);
		fp12_exp_uni(v2, v3, x);
	} else {
		fp12_exp_uni(v0, m, x);
		fp12_exp_uni(v1, v0, x);
		fp12_exp_uni(v2, v1, x);
	}

	fp12_frb(v3, v2, f);
	fp12_mul(v2, v2, v3);
	fp12_sqr_uni(v2, v2);
	fp12_frb(v3, v1, f);
	fp12_conj(v3, v3);
	fp12_mul(v3, v3, v0);
	fp12_mul(v2, v2, v3);
	fp12_frb(v0, v0, f);
	fp12_conj(v3, v1);
	fp12_mul(v2, v2, v3);
	fp12_mul(v0, v0, v3);
	fp12_frb(v1, v1, f);
	fp12_frb(v1, v1, f);
	fp12_mul(v0, v0, v2);
	fp12_mul(v2, v2, v1);
	fp12_sqr_uni(v0, v0);
	fp12_mul(v0, v0, v2);
	fp12_sqr_uni(v0, v0);
	fp12_conj(v1, m);
	fp12_mul(v2, v0, v1);
	fp12_frb(v1, m, f);
	fp12_frb(v1, v1, f);
	fp12_frb(v3, v1, f);
	fp12_mul(v1, v1, v3);
	fp12_frb(v3, m, f);
	fp12_mul(v1, v1, v3);
	fp12_mul(v0, v0, v1);
	fp12_sqr_uni(v2, v2);
	fp12_mul(m, v2, v0);
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void pp_pair_rate(fp12_t res, ep2_t q, ep_t p, bn_t x, fp12_t f) {
	ep2_t t;
	bn_t a;

	ep2_new(t);
	bn_new(a);

	bn_mul_dig(a, x, 6);
	bn_add_dig(a, a, 2);
	if (bn_sign(x) == BN_NEG) {
		bn_neg(a, a);
	}

	//res = f_{r,Q}(P)
	rate_miller(res, t, q, a, p);

	if (bn_sign(x) == BN_NEG) {
		//because f_{-r,Q}(P) = 1/f_{r,Q}(P)
		//INVERSION
		fp12_inv(res, res);
		//We have rQ but we need -rQ
		fp2_neg(t->y, t->y);
	}

	rate_mult(res, t, q, p, f);

	rate_exp(res, x, f);
}
