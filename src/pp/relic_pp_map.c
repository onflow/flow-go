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
 * Implementation of the pairings over prime curves.
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
 * Compute the Miller loop for Ate variants over the bits of r.
 *
 * @param[out] r			- the result.
 * @param[out] t			- the point rQ.
 * @param[in] q				- the first point of the pairing, in G_2.
 * @param[in] a				- the length of the loop.
 * @param[in] p				- the second point of the pairing, in G_1.
 */
void pp_miller(fp12_t r, ep2_t t, ep2_t q, bn_t a, ep_t p) {
	fp12_t tmp;
	ep_t _p;

	fp12_null(tmp);
	ep_null(_p);

	TRY {
		fp12_new(tmp);
		ep_new(_p);

		fp12_zero(tmp);
		fp12_zero(r);
		fp_set_dig(r[0][0][0], 1);
		ep2_copy(t, q);

		ep_neg(_p, p);

		for (int i = bn_bits(a) - 2; i >= 0; i--) {
			fp12_sqr(r, r);
			pp_dbl(tmp, t, t, _p);
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
		ep_free(_p);
	}
}

/**
 * Compute the final exponentiation of the rate pairing in a BN curve.
 *
 * @param[out] m			- the result.
 * @param[in] x				- the parameter used to generate the curve.
 */
void pp_exp(fp12_t c, fp12_t a) {
	fp12_t v0;
	fp12_t v1;
	fp12_t v2;
	fp12_t v3;
	bn_t x;

	fp12_null(v0);
	fp12_null(v1);
	fp12_null(v2);
	fp12_null(v3);
	bn_null(x);

	TRY {
		fp12_new(v0);
		fp12_new(v1);
		fp12_new(v2);
		fp12_new(v3);
		bn_new(x);

		fp_param_get_var(x);

		/* First, compute m^(p^6 - 1). */
		fp12_conv_cyc(c, a);

		/* Now compute m^((p^4 - p^2 + 1) / r). */
		/* From here on we work with x' = -x, therefore if x is positive
		 * we need inversions. */
		int l, *b = fp_param_get_sps(&l);
		if (bn_sign(x) == BN_POS) {
			/* We are now on the cyclotomic subgroup, so inversions are
			 * conjugations. */
			fp12_inv_uni(v3, c);
			fp12_exp_cyc_sps(v0, v3, b, l);
			fp12_inv_uni(v3, v0);
			fp12_exp_cyc_sps(v1, v3, b, l);
			fp12_inv_uni(v3, v1);
			fp12_exp_cyc_sps(v2, v3, b, l);
		} else {
			/* v0 = m^x. */
			fp12_exp_cyc_sps(v0, c, b, l);
			/* v1 = m^x^2. */
			fp12_exp_cyc_sps(v1, v0, b, l);
			/* v2 = m^x^3. */
			fp12_exp_cyc_sps(v2, v1, b, l);
		}

		fp12_frb(v3, v2);
		fp12_mul(v2, v2, v3);
		fp12_sqr_cyc(v2, v2);
		fp12_frb(v3, v1);
		fp12_inv_uni(v3, v3);
		fp12_mul(v3, v3, v0);
		fp12_mul(v2, v2, v3);
		fp12_frb(v0, v0);
		fp12_inv_uni(v3, v1);
		fp12_mul(v2, v2, v3);
		fp12_mul(v0, v0, v3);
		fp12_frb_sqr(v1, v1);
		fp12_mul(v0, v0, v2);
		fp12_mul(v2, v2, v1);
		fp12_sqr_cyc(v0, v0);
		fp12_mul(v0, v0, v2);
		fp12_sqr_cyc(v0, v0);
		fp12_inv_uni(v1, c);
		fp12_mul(v2, v0, v1);
		fp12_frb_sqr(v1, c);
		fp12_frb(v3, v1);
		fp12_mul(v1, v1, v3);
		fp12_frb(v3, c);
		fp12_mul(v1, v1, v3);
		fp12_mul(v0, v0, v1);
		fp12_sqr_cyc(v2, v2);
		fp12_mul(c, v2, v0);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp12_free(v0);
		fp12_free(v1);
		fp12_free(v2);
		fp12_free(v3);
		bn_free(x);
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
		fp12_zero(tmp1);
		fp12_zero(tmp2);

		ep2_copy(r1q, t);
		fp_set_dig(q1->z[0], 1);
		fp_zero(q1->z[1]);

		pp_add(tmp1, r1q, q, p);
		fp12_mul(tmp2, res, tmp1);
		fp12_frb(tmp2, tmp2);
		fp12_mul(res, res, tmp2);

		r1q->norm = 0;
		pp_norm(r1q, r1q);

		ep2_frb(q1, r1q);

		ep2_copy(r1q, t);

		pp_add(tmp1, r1q, q1, p);
		fp12_mul(res, res, tmp1);
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
		fp12_new(tmp);
		fp12_zero(tmp);

		fp_set_dig(q1->z[0], 1);
		fp_zero(q1->z[1]);
		fp_set_dig(q2->z[0], 1);
		fp_zero(q2->z[1]);

		ep2_frb(q1, q);
		ep2_frb_sqr(q2, q);
		ep2_neg(q2, q2);

		pp_add(tmp, t, q1, p);
		fp12_mul(res, res, tmp);
		pp_add(tmp, t, q2, p);
		fp12_mul(res, res, tmp);
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

		fp12_zero(tmp);
		/* q1 = p*xQ + xQ. */
		pp_add(tmp, q1, t, p);
		fp12_mul(res, res, tmp);

		/* q2 = q2 + q3. */
		pp_add(tmp, q2, q3, p);
		fp12_mul(res, res, tmp);

		/* Make q2 affine again. */
		pp_norm(q2, q2);
		pp_add(tmp, q1, q2, p);

		fp12_mul(res, res, tmp);
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
	ep2_curve_init();
}

void pp_map_clean(void) {
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

		fp_param_get_var(x);

		bn_mul_dig(a, x, 6);
		bn_add_dig(a, a, 2);
		if (bn_sign(x) == BN_NEG) {
			bn_neg(a, a);
		}

		/* r = f_{r,Q}(P). */
		pp_miller(r, t, q, a, p);

		if (bn_sign(x) == BN_NEG) {
			/* Since f_{-r,Q}(P) = 1/f_{r,Q}(P), we must invert the result. */
			fp12_inv_uni(r, r);
			ep2_neg(t, t);
		}

		pp_r_ate_mul(r, t, q, p);
		pp_exp(r, r);
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

		fp_param_get_var(x);

		bn_mul_dig(a, x, 6);
		bn_add_dig(a, a, 2);
		if (bn_sign(x) == BN_NEG) {
			bn_neg(a, a);
		}

		/* r = f_{r,Q}(P). */
		pp_miller(r, t, q, a, p);

		if (bn_sign(x) == BN_NEG) {
			/* Since f_{-r,Q}(P) = 1/f_{r,Q}(P), we must invert the result. */
			fp12_inv_uni(r, r);
			ep2_neg(t, t);
		}

		pp_o_ate_mul(r, t, q, p);
		pp_exp(r, r);
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

		fp_param_get_var(x);

		bn_copy(a, x);
		if (bn_sign(x) == BN_NEG) {
			bn_neg(a, a);
		}

		/* r = f_{r,Q}(P). */
		pp_miller(r, t, q, a, p);

		/* Make xQ affine. */
		pp_norm(t, t);

		if (bn_sign(x) == BN_NEG) {
			/* Since f_{-r,Q}(P) = 1/f_{r,Q}(P), we must invert the result. */
			fp12_inv_uni(r, r);
			ep2_neg(t, t);
		}

		pp_x_ate_mul(r, t, q, p);
		pp_exp(r, r);
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
