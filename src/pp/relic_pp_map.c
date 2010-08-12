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
 * Implementation of the R-Ate bilinear pairing.
 *
 * @version $Id$
 * @ingroup pp
 */

#include "relic_core.h"
#include "relic_pp.h"
#include "relic_util.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Adds two prime elliptic curve points and evaluates the corresponding line
 * function at another elliptic curve point.
 * 
 * @param[out] l			- the result of the evaluation.
 * @param[in,out] r			- the first point to add, in Affine coordinates.
 * 							The result of the addition, in Jacobian coordinates.
 * @param[in] q				- the second point to add, in Jacobian coordinates.
 * @param[in] p				- the point where the line function will be
 * 							evaluated, in Affine coordinates.
 */
void pp_add(fp12_t l, ep2_t r, ep2_t q, ep_t p) {
	fp2_t slope;
	ep2_t t;

	fp2_null(slope);
	ep2_null(t);

	TRY {
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
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(slope);
		ep2_free(t);
	}
}

/**
 * Doubles a prime elliptic curve point and evaluates the corresponding line
 * function at another elliptic curve point.
 * 
 * @param[out] l			- the result of the evaluation.
 * @param[out] r			- the result, in Jacobian coordinates.
 * @param[in] q				- the point to double, in Jacobian coordinates.
 * @param[in] p				- the point where the line function will be
 * 							evaluated, in Affine coordinates.
 */
void pp_dbl(fp12_t l, ep2_t r, ep2_t q, ep_t p) {
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

#define PART 32

/**
 * Compute the Miller loop for Ate variants over the bits of r.
 *
 * @param[out] r			- the result.
 * @param[out] t			- the point rQ.
 * @param[in] q				- the first point of the pairing, in G_2.
 * @param[in] a				- the length of the loop.
 * @param[in] p				- the second point of the pairing, in G_1.
 */
void pp_miller(fp12_t r, ep2_t t, ep2_t q, bn_t a, ep_t p) {
#ifndef PP_PARAL
	fp12_t tmp;

	fp12_null(tmp);

	TRY {
		fp12_new(tmp);

		fp12_zero(r);
		fp_set_dig(r[0][0][0], 1);
		ep2_copy(t, q);

		for (int i = bn_bits(a) - 2; i >= 0; i--) {
			fp12_sqr(r, r);
			pp_dbl(tmp, t, t, p);
			fp12_mul_dxs(r, r, tmp);
			if (bn_test_bit(a, i)) {
				pp_add(tmp, t, q, p);
				fp12_mul_dxs(r, r, tmp);
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp12_free(tmp);
	}
#else
	fp12_t _f[CORES], _t[CORES];
	ep2_t _q[CORES];
	bn_t a0, a1;

	bn_null(a0);
	bn_null(a1);

	TRY {
		for (int j = 0; j < CORES; j++) {
			fp12_null(_f[j]);
			fp12_null(_f[j]);
			fp12_new(_t[j]);
			fp12_new(_t[j]);
			ep2_null(_q[j]);
			ep2_new(_q[j]);
		}

		bn_new(a0);
		bn_new(a1);

		bn_rsh(a0, a, PART);
		bn_mod_2b(a1, a, PART);

		omp_set_num_threads(CORES);
#pragma omp parallel sections shared(r, _f, _t, _q, p, q, t, a, a0, a1)
		{
#pragma omp section
			{
				fp12_zero(_f[0]);
				fp_set_dig(_f[0][0][0][0], 1);
				ep2_copy(_q[0], q);
				for (int i = bn_bits(a0) - 2; i >= 0; i--) {
					fp12_sqr(_f[0], _f[0]);
					pp_dbl(_t[0], _q[0], _q[0], p);
					fp12_mul_dxs(_f[0], _f[0], _t[0]);
					if (bn_test_bit(a0, i)) {
						pp_add(_t[0], _q[0], q, p);
						fp12_mul_dxs(_f[0], _f[0], _t[0]);
					}
				}
				for (int i = PART - 1; i >= 0; i--) {
					fp12_sqr(_f[0], _f[0]);
				}
			}
#pragma omp section
			{
				fp12_zero(_f[1]);
				fp_set_dig(_f[1][0][0][0], 1);
				ep2_mul(_q[1], q, a0);
				for (int i = PART - 1; i >= 0; i--) {
					fp12_sqr(_f[1], _f[1]);
					pp_dbl(_t[1], _q[1], _q[1], p);
					fp12_mul_dxs(_f[1], _f[1], _t[1]);
					if (bn_test_bit(a, i)) {
						pp_add(_t[1], _q[1], q, p);
						fp12_mul_dxs(_f[1], _f[1], _t[1]);
					}
				}
				ep2_copy(t, _q[1]);
			}
		}
		fp12_mul(r, _f[0], _f[1]);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep2_free(t0);
		ep2_free(t1);
		bn_free(a0);
		bn_free(a1);
	}
#endif
}

/**
 * Compute the final exponentiation of the rate pairing in a BN curve.
 * 
 * @param[in,out] m			- the result.
 * @param[in] x				- the parameter used to generate the curve.
 */
void pp_exp(fp12_t m, bn_t x) {
	fp12_t v0;
	fp12_t v1;
	fp12_t v2;
	fp12_t v3;

	fp12_null(v0);
	fp12_null(v1);
	fp12_null(v2);
	fp12_null(v3);

	TRY {
		fp12_new(v0);
		fp12_new(v1);
		fp12_new(v2);
		fp12_new(v3);

		/* First, compute m^(p^6 - 1). */
		/* tmp = m^{-1}. */
		fp12_inv(v0, m);
		/* m' = m^(p^6). */
		fp12_inv_cyc(m, m);
		/* m' = m^(p^6 - 1). */
		fp12_mul(m, m, v0);

		/* Second, compute m^(p^2 + 1). */
		/* t = m^(p^2). */
		fp12_frb_sqr(v0, m);
		/* m' = m^(p^2 + 1). */
		fp12_mul(m, m, v0);

		/* Third, compute m^((p^4 - p^2 + 1) / r). */
		/* From here on we work with x' = -x, therefore if x is positive
		 * we need inversions. */
		if (bn_sign(x) == BN_POS) {
			/* We are now on the cyclotomic subgroup, so inversions are
			 * conjugations. */
			fp12_inv_cyc(v3, m);
			fp12_exp_cyc(v0, v3, x);
			fp12_inv_cyc(v3, v0);
			fp12_exp_cyc(v1, v3, x);
			fp12_inv_cyc(v3, v1);
			fp12_exp_cyc(v2, v3, x);
		} else {
			fp12_exp_cyc(v0, m, x);
			fp12_exp_cyc(v1, v0, x);
			fp12_exp_cyc(v2, v1, x);
		}

		fp12_frb(v3, v2);
		fp12_mul(v2, v2, v3);
		fp12_sqr_cyc(v2, v2);
		fp12_frb(v3, v1);
		fp12_inv_cyc(v3, v3);
		fp12_mul(v3, v3, v0);
		fp12_mul(v2, v2, v3);
		fp12_frb(v0, v0);
		fp12_inv_cyc(v3, v1);
		fp12_mul(v2, v2, v3);
		fp12_mul(v0, v0, v3);
		fp12_frb_sqr(v1, v1);
		fp12_mul(v0, v0, v2);
		fp12_mul(v2, v2, v1);
		fp12_sqr_cyc(v0, v0);
		fp12_mul(v0, v0, v2);
		fp12_sqr_cyc(v0, v0);
		fp12_inv_cyc(v1, m);
		fp12_mul(v2, v0, v1);
		fp12_frb_sqr(v1, m);
		fp12_frb(v3, v1);
		fp12_mul(v1, v1, v3);
		fp12_frb(v3, m);
		fp12_mul(v1, v1, v3);
		fp12_mul(v0, v0, v1);
		fp12_sqr_cyc(v2, v2);
		fp12_mul(m, v2, v0);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp12_free(v0);
		fp12_free(v1);
		fp12_free(v2);
		fp12_free(v3);
	}
}

/**
 * Compute the additional multiplication required by the R-ate pairing.
 *
 * @param[in,out] res		- the result.
 * @param[in] t				- the elliptic point produced by the Miller loop.
 * @param[in] q				- the first point of the pairing, in G_2.
 * @param[in] p				- the second point of the pairing, in G_1.
 */
void pp_r_ate_mul(fp12_t res, ep2_t t, ep2_t q, ep_t p) {
	ep2_t q1, r1q;
	fp12_t tmp1;
	fp12_t tmp2;

	fp12_null(tmp1);
	fp12_null(tmp2);
	ep2_null(q1);
	ep2_null(r1q);

	TRY {
		ep2_new(q1);
		ep2_new(r1q);
		fp12_new(tmp1);
		fp12_new(tmp2);

		ep2_copy(r1q, t);
		fp_set_dig(q1->z[0], 1);
		fp_zero(q1->z[1]);

		pp_add(tmp1, r1q, q, p);
		fp12_mul_dxs(tmp2, res, tmp1);
		fp12_frb(tmp2, tmp2);
		fp12_mul(res, res, tmp2);

		r1q->norm = 0;
		ep2_norm(r1q, r1q);

		ep2_frb(q1, r1q);

		ep2_copy(r1q, t);

		pp_add(tmp1, r1q, q1, p);
		fp12_mul_dxs(res, res, tmp1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp12_free(tmp1);
		fp12_free(tmp2);
		ep2_free(q1);
		ep2_free(r1q);
	}
}

/**
 * Compute the additional multiplication required by the Optimal Ate pairing.
 *
 * @param[in,out] res		- the result.
 * @param[in] t				- the elliptic point produced by the Miller loop.
 * @param[in] q				- the first point of the pairing, in G_2.
 * @param[in] p				- the second point of the pairing, in G_1.
 */
void pp_o_ate_mul(fp12_t res, ep2_t t, ep2_t q, ep_t p) {
	ep2_t q1, q2;
	fp12_t tmp;

	fp12_null(tmp);
	ep2_null(q1);
	ep2_null(q2);

	TRY {
		ep2_new(q1);
		ep2_new(q2);
		ep2_new(q3);
		fp12_new(tmp);

		fp_set_dig(q1->z[0], 1);
		fp_zero(q1->z[1]);
		fp_set_dig(q2->z[0], 1);
		fp_zero(q2->z[1]);

		ep2_frb(q1, q);
		ep2_frb_sqr(q2, q);
		ep2_neg(q2, q2);

		pp_add(tmp, t, q1, p);
		fp12_mul_dxs(res, res, tmp);
		pp_add(tmp, t, q2, p);
		fp12_mul_dxs(res, res, tmp);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp12_free(tmp);
		ep2_free(q1);
		ep2_free(q2);
	}
}

/**
 * Compute the additional multiplication required by the X-ate pairing.
 *
 * @param[in,out] res		- the result.
 * @param[in] t				- the elliptic point produced by the Miller loop.
 * @param[in] q				- the first point of the pairing, in G_2.
 * @param[in] p				- the second point of the pairing, in G_1.
 */
void pp_x_ate_mul(fp12_t res, ep2_t t, ep2_t q, ep_t p) {
	ep2_t q1, q2, q3;
	fp12_t tmp;

	fp12_null(tmp);
	ep2_null(q1);
	ep2_null(q2);
	ep2_null(q3);

	TRY {
		ep2_new(q1);
		ep2_new(q2);
		ep2_new(q3);
		fp12_new(tmp);

		fp_set_dig(q1->z[0], 1);
		fp_zero(q1->z[1]);
		fp_set_dig(q2->z[0], 1);
		fp_zero(q2->z[1]);
		fp_set_dig(q3->z[0], 1);
		fp_zero(q3->z[1]);

		/* r = r^p. */
		fp12_frb(tmp, res);
		fp12_mul(res, res, tmp);

		/* r = r^p^3. */
		fp12_frb_sqr(tmp, tmp);
		fp12_mul(res, res, tmp);

		/* r = r^p^5. */
		fp12_frb_sqr(tmp, tmp);
		/* r = r^p^7. */
		fp12_frb_sqr(tmp, tmp);
		/* r = r^p^9. */
		fp12_frb_sqr(tmp, tmp);
		/* r = r^p^10. */
		fp12_frb(tmp, tmp);
		fp12_mul(res, res, tmp);

		/* q1 = p * xQ. */
		ep2_frb(q1, t);
		/* q2 = p^3 * xQ. */
		ep2_frb_sqr(q2, q1);
		/* q3 = p^5 * xQ. */
		ep2_frb_sqr(q3, q2);
		/* q3 = p^7 * xQ. */
		ep2_frb_sqr(q3, q3);
		/* q3 = p^9 * xQ. */
		ep2_frb_sqr(q3, q3);
		/* q3 = p^10 * xQ. */
		ep2_frb(q3, q3);

		/* q1 = p*xQ + xQ. */
		pp_add(tmp, q1, t, p);
		fp12_mul_dxs(res, res, tmp);

		/* q2 = q2 + q3. */
		pp_add(tmp, q2, q3, p);
		fp12_mul_dxs(res, res, tmp);

		/* Make q2 affine again. */
		ep2_norm(q2, q2);
		pp_add(tmp, q1, q2, p);

		fp12_mul_dxs(res, res, tmp);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp12_free(tmp);
		ep2_free(q1);
		ep2_free(q2);
		ep2_free(q3);
	}
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void pp_map_init(void) {
	fp2_const_init();
	ep2_curve_init();
}

void pp_map_clean(void) {
	fp2_const_clean();
	ep2_curve_clean();
}

#if PP_MAP == R_ATE || !defined(STRIP)

void pp_map_r_ate(fp12_t r, ep_t p, ep2_t q) {
	ep2_t t;
	bn_t a, x;

	ep2_null(t);
	bn_null(a);
	bn_null(x);

	TRY {
		ep2_new(t);
		bn_new(a);
		bn_new(x);

		switch (fp_param_get()) {
			case BN_158:
				/* x = 4000000031. */
				bn_set_2b(x, 38);
				bn_add_dig(x, x, 0x31);
				break;
			case BN_254:
				/* x = -4080000000000001. */
				bn_set_2b(x, 62);
				bn_set_2b(a, 55);
				bn_add(x, x, a);
				bn_add_dig(x, x, 1);
				bn_neg(x, x);
				break;
			case BN_256:
				/* x = 6000000000001F2D. */
				bn_set_2b(x, 62);
				bn_set_2b(a, 61);
				bn_add(x, x, a);
				bn_set_dig(a, 0x1F);
				bn_lsh(a, a, 8);
				bn_add(x, x, a);
				bn_add_dig(x, x, 0x2D);
				break;
			default:
				THROW(ERR_INVALID);
				break;
		}

		bn_mul_dig(a, x, 6);
		bn_add_dig(a, a, 2);
		if (bn_sign(x) == BN_NEG) {
			bn_neg(a, a);
		}

		/* r = f_{r,Q}(P). */
		pp_miller(r, t, q, a, p);

		if (bn_sign(x) == BN_NEG) {
			/* Since f_{-r,Q}(P) = 1/f_{r,Q}(P), we must invert the result. */
			fp12_inv_cyc(r, r);
			ep2_neg(t, t);
		}

		pp_r_ate_mul(r, t, q, p);
		pp_exp(r, x);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep2_free(t);
		bn_free(a);
		bn_free(x);
	}
}

#endif

#if PP_MAP == O_ATE || !defined(STRIP)

void pp_map_o_ate(fp12_t r, ep_t p, ep2_t q) {
	ep2_t t;
	bn_t a, x;

	ep2_null(t);
	bn_null(a);
	bn_null(x);

	TRY {
		ep2_new(t);
		bn_new(a);
		bn_new(x);

		switch (fp_param_get()) {
			case BN_158:
				/* x = 4000000031. */
				bn_set_2b(x, 38);
				bn_add_dig(x, x, 0x31);
				break;
			case BN_254:
				/* x = -4080000000000001. */
				bn_set_2b(x, 62);
				bn_set_2b(a, 55);
				bn_add(x, x, a);
				bn_add_dig(x, x, 1);
				bn_neg(x, x);
				break;
			case BN_256:
				/* x = 6000000000001F2D. */
				bn_set_2b(x, 62);
				bn_set_2b(a, 61);
				bn_add(x, x, a);
				bn_set_dig(a, 0x1F);
				bn_lsh(a, a, 8);
				bn_add(x, x, a);
				bn_add_dig(x, x, 0x2D);
				break;
			default:
				THROW(ERR_INVALID);
				break;
		}

		bn_mul_dig(a, x, 6);
		bn_add_dig(a, a, 2);
		if (bn_sign(x) == BN_NEG) {
			bn_neg(a, a);
		}

		/* r = f_{r,Q}(P). */
		pp_miller(r, t, q, a, p);

		if (bn_sign(x) == BN_NEG) {
			/* Since f_{-r,Q}(P) = 1/f_{r,Q}(P), we must invert the result. */
			fp12_inv_cyc(r, r);
			ep2_neg(t, t);
		}

		pp_o_ate_mul(r, t, q, p);

		pp_exp(r, x);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep2_free(t);
		bn_free(a);
		bn_free(x);
	}
}

#endif

#if PP_MAP == X_ATE || !defined(STRIP)

void pp_map_x_ate(fp12_t r, ep_t p, ep2_t q) {
	ep2_t t;
	bn_t a, x;

	ep2_null(t);
	bn_null(a);
	bn_null(x);

	TRY {
		ep2_new(t);
		bn_new(a);
		bn_new(x);

		switch (fp_param_get()) {
			case BN_158:
				/* x = 4000000031. */
				bn_set_2b(x, 38);
				bn_add_dig(x, x, 0x31);
				break;
			case BN_254:
				/* x = -4080000000000001. */
				bn_set_2b(x, 62);
				bn_set_2b(a, 55);
				bn_add(x, x, a);
				bn_add_dig(x, x, 1);
				bn_neg(x, x);
				break;
			case BN_256:
				/* x = 6000000000001F2D. */
				bn_set_2b(x, 62);
				bn_set_2b(a, 61);
				bn_add(x, x, a);
				bn_set_dig(a, 0x1F);
				bn_lsh(a, a, 8);
				bn_add(x, x, a);
				bn_add_dig(x, x, 0x2D);
				break;
			default:
				THROW(ERR_INVALID);
				break;
		}

		bn_copy(a, x);
		if (bn_sign(x) == BN_NEG) {
			bn_neg(a, a);
		}

		/* r = f_{r,Q}(P). */
		pp_miller(r, t, q, a, p);

		/* Make xQ affine. */
		ep2_norm(t, t);

		if (bn_sign(x) == BN_NEG) {
			/* Since f_{-r,Q}(P) = 1/f_{r,Q}(P), we must invert the result. */
			fp12_inv_cyc(r, r);
			ep2_neg(t, t);
		}

		pp_x_ate_mul(r, t, q, p);
		pp_exp(r, x);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep2_free(t);
		bn_free(a);
		bn_free(x);
	}
}

#endif
