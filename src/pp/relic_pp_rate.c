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
 * Implementation of the eta_t bilinear pairing.
 *
 * @version $Id$
 * @ingroup etat
 */

#include "relic_core.h"
#include "relic_fp.h"
#include "relic_fp12.h"
#include "relic_ep.h"
#include "relic_ep2.h"
#include "relic_pp.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

void rate_line(fp12_t l, ep2_t c, ep2_t a, ep2_t b, fp2_t slope, ep_t q) {
	fp6_t n, d;

	fp6_new(d);
	fp6_new(n);

#if EP_ADD == BASIC

	/*
	 * The element d is composed of three quadratic extension field elements.
	 * We must initialize d with (slope * q->x, slope * a->x, 0), but first
	 * we must convert q->x to a quadratic extension field element.
	 */
	fp2_zero(d[2]);
	fp_zero(d[0][1]);
	fp_copy(d[0][0], q->x);
	fp2_mul(d[0], slope, d[0]);
	fp2_mul(d[1], slope, a->x);
	fp2_sub(d[1], a->y, d[1]);

	/* n = ((-q->y, 0), 0, 0). */
	fp_neg(n[0][0], q->y);
	fp_zero(n[0][1]);
	fp2_zero(n[1]);
	fp2_zero(n[2]);

	fp6_copy(l[0], n);
	fp6_copy(l[1], d);

#endif
	fp6_free(d);
	fp6_free(n);
}

void rate_add(fp12_t l, ep2_t r, ep2_t p, ep_t q) {
	fp2_t slope;
	ep2_t t;

	fp2_new(slope);
	ep2_new(t);

	ep2_copy(t, r);
	ep2_add_slope(r, slope, r, p);

	if (ep2_is_infinity(r)) {
		fp12_set_dig(l, 1);
	} else {
		fp6_t n, d;

		fp6_new(d);
		fp6_new(n);

		fp_zero(d[0][1]);
		fp_copy(d[0][0], q->x);
		fp2_mul(d[0], d[0], slope);
		fp2_neg(d[0], d[0]);
		fp2_mul(d[2], r->z, p->y);
		fp2_mul(d[1], slope, p->x);
		fp2_sub(d[1], d[1], d[2]);
		fp2_zero(d[2]);

		fp_zero(n[0][1]);
		fp_copy(n[0][0], q->y);
		fp2_mul(n[0], n[0], r->z);
		fp2_zero(n[1]);
		fp2_zero(n[2]);

		fp6_copy(l[0], n);
		fp6_copy(l[1], d);

		//rate_line(l, r, t, p, slope, q);
	}

	fp2_free(slope);
	ep2_free(t);
}

void rate_dbl(fp12_t l, ep2_t r, ep2_t p, ep_t q) {
	fp2_t s;
	fp2_t e;
	ep2_t t;

	fp2_new(s);
	fp2_new(e);
	ep2_new(t);
	ep2_copy(t, r);
	ep2_dbl_slope(r, s, e, r);

	if (ep2_is_infinity(r)) {
		fp12_set_dig(l, 1);
	} else {
		fp6_t n, d;

		fp6_new(d);
		fp6_new(n);

		fp2_sqr(d[2], t->z);
		fp2_mul(d[0], d[2], s);
		fp_mul(d[0][0], d[0][0], q->x);
		fp_mul(d[0][1], d[0][1], q->x);
		fp2_neg(d[0], d[0]);
		fp2_mul(d[1], s, t->x);
		fp2_sub(d[1], d[1], e);

		fp2_mul(n[0], r->z, d[2]);
		fp_mul(n[0][0], n[0][0], q->y);
		fp_mul(n[0][1], n[0][1], q->y);
		fp2_zero(d[2]);
		fp2_zero(n[1]);
		fp2_zero(n[2]);

		//rate_line(l, r, t, p, s, q);

		fp6_copy(l[0], n);
		fp6_copy(l[1], d);
	}

	fp2_free(s);
	ep2_free(t);
}

static void ep2_frob(ep2_t c, ep2_t a, fp12_t f) {
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

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void pp_pair_rate2(fp12_t r, ep2_t p, ep_t q, bn_t x, fp12_t f) {
	bn_t a;
	bn_t b;
	ep2_t t, _t;
	fp12_t l, _r, w, c, rp;

	bn_new(a);
	bn_new(b);
	fp12_new(l);
	fp12_new(_r);
	fp12_new(w);
	fp12_new(c);
	fp12_new(rp);
	ep2_new(t);
	ep2_new(_t);

	bn_mul_dig(a, x, 6);
	if (bn_sign(x) == BN_POS) {
		bn_add_dig(a, a, 2);
	} else {
		bn_add_dig(a, a, 3);
		bn_neg(a, a);
	}

	ep2_copy(t, p);
	fp12_set_dig(r, 1);

	for (int i = bn_bits(a) - 2; i >= 0; i--) {
		fp12_sqr(r, r);
		rate_dbl(l, t, t, q);
		fp12_mul_sparse(r, r, l);
		if (bn_test_bit(a, i)) {
			rate_add(l, t, p, q);
			fp12_mul_sparse(r, r, l);
		}
	}

	fp12_copy(_r, r);
	ep2_copy(_t, t);
	if (bn_sign(x) == BN_POS) {
		rate_add(l, t, p, q);
		fp12_mul_sparse(r, r, l);
		fp12_frob(r, r, f);
		fp12_mul(r, r, _r);
	} else {
		fp12_frob(r, r, f);
		fp12_mul(r, r, _r);
		rate_add(l, _t, p, q);
		fp12_mul_sparse(r, r, l);
	}
	ep2_frob(t, t, f);

	ep2_norm(_t, _t);
	rate_add(l, t, _t, q);
	fp12_mul_sparse(r, r, l);

	fp12_copy(_r, r);
	fp12_conj(r, r);
	fp12_inv(_r, _r);
	fp12_mul(r, r, _r);

	fp12_copy(_r, r);
	fp12_frob(r, r, f);
	fp12_frob(r, r, f);
	fp12_mul(r, r, _r);
//	bench_timing_reset("rate_exp");
//	bench_timing_before();

	if (bn_sign(x) == BN_NEG) {
		bn_mul_dig(a, x, 6);
		bn_add_dig(a, a, 5);
		bn_neg(a, a);
		fp12_exp_basic_uni(_r, r, a);
	} else {
		bn_mul_dig(a, x, 6);
		bn_add_dig(a, a, 5);
		fp12_exp_basic_uni(_r, r, a);
		fp12_inv(_r, _r);
	}

	fp12_frob(l, _r, f);
	fp12_mul(l, l, _r);

	fp12_mul(_r, _r, l);
	fp12_sqr(w, r);
	fp12_sqr(w, w);
	fp12_mul(_r, _r, w);
	fp12_frob(rp, r, f);
	fp12_mul(c, rp, r);
	fp12_sqr(w, c);
	fp12_sqr(w, w);
	fp12_sqr(w, w);
	fp12_mul(w, w, c);

	fp12_mul(_r, _r, w);
	fp12_sqr(w, rp);
	fp12_frob(rp, rp, f);
	fp12_mul(w, w, l);
	fp12_mul(w, w, rp);

	fp12_exp_basic_uni(c, w, x);
	bn_mul_dig(a, x, 6);
	fp12_exp_basic_uni(c, c, a);
	fp12_mul(r, w, c);

	fp12_frob(rp, rp, f);
	fp12_mul(_r, _r, rp);
	fp12_mul(r, r, _r);
//	bench_timing_after();
//	bench_timing_compute(1);

	bn_free(a);
	fp12_free(l);
	fp12_free(_r);
	fp12_free(w);
	fp12_free(c);
	fp12_free(rp);
	ep2_free(t);
	ep2_free(_t);
}

void pp_pair_rate(fp12_t r, ep2_t p, ep_t q, bn_t x, fp12_t f) {
	bn_t a;
	bn_t b;
	ep2_t t, _t, t0, t1;
	fp12_t l, _l, _r, w, c, rp;

	bn_new(a);
	bn_new(b);
	fp12_new(l);
	fp12_new(_l);
	fp12_new(_r);
	fp12_new(w);
	fp12_new(c);
	fp12_new(rp);
	ep2_new(t);
	ep2_new(t0);
	ep2_new(t1);
	ep2_new(_t);

	bn_mul_dig(a, x, 6);
	if (bn_sign(x) == BN_POS) {
		bn_add_dig(a, a, 2);
	} else {
		bn_add_dig(a, a, 3);
		bn_neg(a, a);
	}
#define PART 27
	bn_rsh(a, a, PART);
	bn_set_dig(b, 3);
#pragma omp parallel sections shared(r, c, t, t1, l, _l, p, q, a, b) default(none)
{
#pragma omp section
	{
	ep2_copy(t, p);
	fp12_set_dig(r, 1);
	for (int i = 64 - PART-1; i >= 0; i--) {
		fp12_sqr(r, r);
		rate_dbl(l, t, t, q);
		fp12_mul_sparse(r, r, l);
		if (bn_test_bit(a, i)) {
			rate_add(l, t, p, q);
			fp12_mul_sparse(r, r, l);
		}
	}
	for (int i = PART; i > 0; i--) {
		fp12_sqr(r, r);
	}
	}
#pragma omp section
	{
	ep2_mul(t1, p, a);
	fp12_set_dig(c, 1);
	for (int i = PART-1; i >= 0; i--) {
		fp12_sqr(c, c);
		rate_dbl(_l, t1, t1, q);
		fp12_mul_sparse(c, c, _l);
		if (bn_test_bit(b, i)) {
			rate_add(_l, t1, p, q);
			fp12_mul_sparse(c, c, _l);
		}
	}
	}
}
fp12_mul(r, r, c);
	fp12_copy(_r, r);
	ep2_copy(_t, t1);
	if (bn_sign(x) == BN_POS) {
		rate_add(l, t1, p, q);
		fp12_mul(r, r, l);
		fp12_frob(r, r, f);
		fp12_mul_sparse(r, r, _r);
	} else {
		fp12_frob(r, r, f);
		fp12_mul(r, r, _r);
		rate_add(l, _t, p, q);
		fp12_mul_sparse(r, r, l);
	}

	ep2_frob(t, t1, f);
	ep2_norm(_t, _t);
	rate_add(l, t, _t, q);
	fp12_mul_sparse(r, r, l);

	fp12_copy(_r, r);
	fp12_conj(r, r);
	fp12_inv(_r, _r);
	fp12_mul(r, r, _r);

	fp12_copy(_r, r);
	fp12_frob(r, r, f);
	fp12_frob(r, r, f);
	fp12_mul(r, r, _r);
//	bench_timing_reset("rate_exp");
//	bench_timing_before();

	if (bn_sign(x) == BN_NEG) {
		bn_mul_dig(a, x, 6);
		bn_add_dig(a, a, 5);
		bn_neg(a, a);
		fp12_exp_basic_uni(_r, r, a);
	} else {
		bn_mul_dig(a, x, 6);
		bn_add_dig(a, a, 5);
		fp12_exp_basic_uni(_r, r, a);
		fp12_inv(_r, _r);
	}

	fp12_frob(l, _r, f);
	fp12_mul(l, l, _r);

	fp12_mul(_r, _r, l);
	fp12_sqr(w, r);
	fp12_sqr(w, w);
	fp12_mul(_r, _r, w);
	fp12_frob(rp, r, f);
	fp12_mul(c, rp, r);
	fp12_sqr(w, c);
	fp12_sqr(w, w);
	fp12_sqr(w, w);
	fp12_mul(w, w, c);

	fp12_mul(_r, _r, w);
	fp12_sqr(w, rp);
	fp12_frob(rp, rp, f);
	fp12_mul(w, w, l);
	fp12_mul(w, w, rp);

	fp12_exp_basic_uni(c, w, x);
	bn_mul_dig(a, x, 6);
	fp12_exp_basic_uni(c, c, a);
	fp12_mul(r, w, c);

	fp12_frob(rp, rp, f);
	fp12_mul(_r, _r, rp);
	fp12_mul(r, r, _r);
//	bench_timing_after();
//	bench_timing_compute(1);

	bn_free(a);
	fp12_free(l);
	fp12_free(_r);
	fp12_free(w);
	fp12_free(c);
	fp12_free(rp);
	ep2_free(t);
	ep2_free(_t);
}

void pp_pair_rate3(fp12_t r, ep2_t p, ep_t q, bn_t x, fp12_t f) {
	bn_t a;
	bn_t b;
	ep2_t t, _t;
	fp12_t l, _r, w, c, rp;

	bn_new(a);
	bn_new(b);
	fp12_new(l);
	fp12_new(_r);
	fp12_new(w);
	fp12_new(c);
	fp12_new(rp);
	ep2_new(t);
	ep2_new(_t);

	bn_mul_dig(a, x, 6);
	if (bn_sign(x) == BN_POS) {
		bn_add_dig(a, a, 2);
	} else {
		bn_add_dig(a, a, 3);
		bn_neg(a, a);
	}
	ep2_mul(_t, p, a);

	ep2_copy(t, p);
	fp12_set_dig(r, 1);
	bn_rsh(a, a, 32);
	bn_set_dig(b, 3);
	for (int i = 31; i >= 0; i--) {
		fp12_sqr(r, r);
		rate_dbl(l, t, t, q);
		fp12_mul(r, r, l);
		if (bn_test_bit(a, i)) {
			rate_add(l, t, p, q);
			fp12_mul(r, r, l);
		}
	}
	for (int i = 32; i > 0; i--) {
		fp12_sqr(r, r);
	}

	ep2_mul(t, p, a);
	fp12_set_dig(c, 1);
	rate_dbl(l, t, t, q);
	fp12_mul(c, c, l);
	for (int i = 30; i >= 0; i--) {
		fp12_sqr(c, c);
		rate_dbl(l, t, t, q);
		fp12_mul(c, c, l);
		if (bn_test_bit(b, i)) {
			rate_add(l, t, p, q);
			fp12_mul(c, c, l);
		}
	}
	fp12_mul(r, r, c);

	fp12_copy(_r, r);
	if (bn_sign(x) == BN_POS) {
		rate_add(l, _t, p, q);
		fp12_mul(r, r, l);
		fp12_frob(r, r, f);
		fp12_mul(r, r, _r);
	} else {
		fp12_frob(r, r, f);
		fp12_mul(r, r, _r);
		rate_add(l, _t, p, q);
		fp12_mul(r, r, l);
	}

	ep2_frob(t, t, f);
	ep2_norm(_t, _t);
	rate_add(l, t, _t, q);
	fp12_mul(r, r, l);

	fp12_copy(_r, r);
	fp12_conj(r, r);
	fp12_inv(_r, _r);
	fp12_mul(r, r, _r);

	fp12_copy(_r, r);
	fp12_frob(r, r, f);
	fp12_frob(r, r, f);
	fp12_mul(r, r, _r);
//	bench_timing_reset("rate_exp");
//	bench_timing_before();

	if (bn_sign(x) == BN_NEG) {
		bn_mul_dig(a, x, 6);
		bn_add_dig(a, a, 5);
		bn_neg(a, a);
		fp12_exp_basic(_r, r, a);
	} else {
		bn_mul_dig(a, x, 6);
		bn_add_dig(a, a, 5);
		fp12_exp_basic(_r, r, a);
		fp12_inv(_r, _r);
	}

	fp12_frob(l, _r, f);
	fp12_mul(l, l, _r);

	fp12_mul(_r, _r, l);
	fp12_sqr(w, r);
	fp12_sqr(w, w);
	fp12_mul(_r, _r, w);
	fp12_frob(rp, r, f);
	fp12_mul(c, rp, r);
	fp12_sqr(w, c);
	fp12_sqr(w, w);
	fp12_sqr(w, w);
	fp12_mul(w, w, c);

	fp12_mul(_r, _r, w);
	fp12_sqr(w, rp);
	fp12_frob(rp, rp, f);
	fp12_mul(w, w, l);
	fp12_mul(w, w, rp);

	fp12_exp_basic(c, w, x);
	bn_mul_dig(a, x, 6);
	fp12_exp_basic(c, c, a);
	fp12_mul(r, w, c);

	fp12_frob(rp, rp, f);
	fp12_mul(_r, _r, rp);
	fp12_mul(r, r, _r);
//	bench_timing_after();
//	bench_timing_compute(1);

	bn_free(a);
	fp12_free(l);
	fp12_free(_r);
	fp12_free(w);
	fp12_free(c);
	fp12_free(rp);
	ep2_free(t);
	ep2_free(_t);
}

void rate_miller(fp12_t res, ep2_t t, ep2_t q, bn_t r, ep_t p) {
	fp12_t tmp;
	unsigned int i;

	fp12_new(tmp);

	fp12_set_dig(res, 1);
	ep2_copy(t, q);

	for (i = bn_bits(r) - 1; i > 0; i--) {
		rate_dbl(tmp, t, t, p);
		fp12_sqr(res, res);
		fp12_mul_sparse(res, res, tmp);

		if (bn_test_bit(r, i - 1)) {
			rate_add(tmp, t, q, p);
			fp12_mul_sparse(res, res, tmp);
		}
	}
}

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
	fp12_mul_sparse(tmp2, res, tmp1);
	fp12_frob(tmp2, tmp2, f);
	fp12_mul(res, res, tmp2);

	//INVERSION
	r1q->norm = 0;
	ep2_norm(r1q, r1q);

	ep2_frob(q1, r1q, f);

	ep2_copy(r1q, t);

	rate_add(tmp1, r1q, q1, p);
	fp12_mul_sparse(res, res, tmp1);
}

void rate_final_expo(fp12_t m, bn_t x, fp12_t f) {
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
	fp12_frob(v0, m, f);
	fp12_frob(v0, v0, f);
	//m' = m^(p^2 + 1)
	fp12_mul(m, m, v0);

	//Third, m^((p^4 - p^2 + 1) / r)
	//From here on we work with x' = -x, therefore if x is positive
	//we need inversions
	if (bn_sign(x) == BN_POS) {
		//TODO: maybe these inversions are just conjugations, checl
		//INVERSION
		fp12_inv(v3, m);
		fp12_exp_basic_uni(v0, v3, x);
		//INVERSION
		fp12_inv(v3, v0);
		fp12_exp_basic_uni(v1, v3, x);
		//INVERSION
		fp12_inv(v3, v1);
		fp12_exp_basic_uni(v2, v3, x);
	} else {
		fp12_exp_basic_uni(v0, m, x);
		fp12_exp_basic_uni(v1, v0, x);
		fp12_exp_basic_uni(v2, v1, x);
	}

	//TODO: make specific version for x positive to avoid above inversions
	fp12_frob(v3, v2, f);
	fp12_mul(v2, v2, v3);
	fp12_sqr_uni(v2, v2);
	fp12_frob(v3, v1, f);
	fp12_conj(v3, v3);
	fp12_mul(v3, v3, v0);
	fp12_mul(v2, v2, v3);
	fp12_frob(v0, v0, f);
	fp12_conj(v3, v1);
	fp12_mul(v2, v2, v3);
	fp12_mul(v0, v0, v3);
	fp12_frob(v1, v1, f);
	fp12_frob(v1, v1, f);
	fp12_mul(v0, v0, v2);
	fp12_mul(v2, v2, v1);
	fp12_sqr_uni(v0, v0);
	fp12_mul(v0, v0, v2);
	fp12_sqr_uni(v0, v0);
	fp12_conj(v1, m);
	fp12_mul(v2, v0, v1);
	fp12_frob(v1, m, f);
	fp12_frob(v1, v1, f);
	fp12_frob(v3, v1, f);
	fp12_mul(v1, v1, v3);
	fp12_frob(v3, m, f);
	fp12_mul(v1, v1, v3);
	fp12_mul(v0, v0, v1);
	fp12_sqr_uni(v2, v2);
	fp12_mul(m, v2, v0);
}

void pp_pair_rate4(fp12_t res, ep2_t q, ep_t p, bn_t x, fp12_t f) {
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

	rate_final_expo(res, x, f);
}
