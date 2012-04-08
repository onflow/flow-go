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
 * Compute the Miller loop for a pairings of type G_2 x G_1 over the bits of a
 * given parameter.
 *
 * @param[out] r			- the result.
 * @param[out] t			- the resulting point.
 * @param[in] q				- the first point of the pairing, in G_2.
 * @param[in] p				- the second point of the pairing, in G_1.
 * @param[in] a				- the loop parameter.
 */
static void pp_mil_k12(fp12_t r, ep2_t t, ep2_t q, ep_t p, bn_t a) {
	fp12_t l;
	ep_t _p;

	fp12_null(l);
	ep_null(_p);

	TRY {
		fp12_new(l);
		ep_new(_p);

		fp12_zero(l);
		fp12_zero(r);
		fp_set_dig(r[0][0][0], 1);
		ep2_copy(t, q);

		ep_neg(_p, p);

		for (int i = bn_bits(a) - 2; i >= 0; i--) {
			fp12_sqr(r, r);
			pp_dbl_k12(l, t, t, _p);
			fp12_mul_dxs(r, r, l);
			if (bn_test_bit(a, i)) {
				pp_add_k12(l, t, q, p);
				fp12_mul_dxs(r, r, l);
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp12_free(l);
		ep_free(_p);
	}
}

/**
 * Compute the Miller loop for a pairings of type G_2 x G_1 over the bits of a
 * given parameter represented in sparse form.
 *
 * @param[out] r			- the result.
 * @param[out] t			- the resulting point.
 * @param[in] q				- the first point of the pairing, in G_2.
 * @param[in] p				- the second point of the pairing, in G_1.
 * @param[in] s				- the loop parameter in sparse form.
 * @paramin] len			- the length of the loop parameter.
 */
static void pp_mil_k12_sps(fp12_t r, ep2_t t, ep2_t q, ep_t p, int *s, int len) {
	fp12_t l;
	ep_t _p;
	ep2_t _q;

	fp12_null(l);
	ep_null(_p);
	ep2_null(_q);

	TRY {
		fp12_new(l);
		ep_new(_p);
		ep2_new(_q);

		fp12_zero(l);
		fp12_zero(r);
		fp_set_dig(r[0][0][0], 1);
		ep2_copy(t, q);

		ep_neg(_p, p);
		ep2_neg(_q, q);

		for (int i = len - 2; i >= 0; i--) {
			fp12_sqr(r, r);
			pp_dbl_k12(l, t, t, _p);
			fp12_mul_dxs(r, r, l);
			if (s[i] > 0) {
				pp_add_k12(l, t, q, p);
				fp12_mul_dxs(r, r, l);
			}
			if (s[i] < 0) {
				pp_add_k12(l, t, _q, p);
				fp12_mul_dxs(r, r, l);
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp12_free(l);
		ep_free(_p);
		ep2_free(_q);
	}
}

static void pp_mil_k12_lit(fp12_t r, ep_t t, ep_t p, ep2_t q, bn_t a) {
	fp12_t l;

	fp12_null(l);

	TRY {
		fp12_new(l);
		fp12_zero(l);

		ep_copy(t, p);
		fp12_zero(r);
		fp_set_dig(r[0][0][0], 1);
		fp12_zero(l);

		for (int i = bn_bits(a) - 2; i >= 0; i--) {
			fp12_sqr(r, r);
			pp_dbl_k12_lit(l, t, t, q);
			fp12_mul(r, r, l);
			if (bn_test_bit(a, i)) {
				pp_add_k12_lit(l, t, p, q);
				fp12_mul(r, r, l);
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp12_free(l);
	}
}

/**
 * Compute the final exponentiation of a pairing defined over a Barreto-Naehrig
 * curve.
 *
 * @param[out] c			- the result.
 * @param[in] a				- the extension field element to exponentiate.
 */
static void pp_exp_bn(fp12_t c, fp12_t a) {
	fp12_t t0, t1, t2, t3;
	int l, *b = fp_param_get_sps(&l);
	bn_t x;

	fp12_null(t0);
	fp12_null(t1);
	fp12_null(t2);
	fp12_null(t3);
	bn_null(x);

	TRY {
		fp12_new(t0);
		fp12_new(t1);
		fp12_new(t2);
		fp12_new(t3);
		bn_new(x);

		/*
		 * New final exponentiation following Fuentes-Castañeda, Knapp and
		 * Rodríguez-Henríquez: Fast Hashing to G_2.
		 */
		fp_param_get_var(x);

		/* First, compute m = f^(p^6 - 1)(p^2 + 1). */
		fp12_conv_cyc(c, a);

		/* Now compute m^((p^4 - p^2 + 1) / r). */
		/* t0 = m^2x. */
		fp12_exp_cyc_sps(t0, c, b, l);
		fp12_sqr_cyc(t0, t0);
		/* t1 = m^6x. */
		fp12_sqr_cyc(t1, t0);
		fp12_mul(t1, t1, t0);
		/* t2 = m^6x^2. */
		fp12_exp_cyc_sps(t2, t1, b, l);
		/* t3 = m^12x^3. */
		fp12_sqr_cyc(t3, t2);
		fp12_exp_cyc_sps(t3, t3, b, l);

		if (bn_sign(x) == BN_NEG) {
			fp12_inv_uni(t0, t0);
			fp12_inv_uni(t1, t1);
			fp12_inv_uni(t3, t3);
		}

		/* t3 = a = m^12x^3 * m^6x^2 * m^6x. */
		fp12_mul(t3, t3, t2);
		fp12_mul(t3, t3, t1);

		/* t0 = b = 1/(m^2x) * t3. */
		fp12_inv_uni(t0, t0);
		fp12_mul(t0, t0, t3);

		/* Compute t2 * t3 * m * b^p * a^p^2 * [b * 1/m]^p^3. */
		fp12_mul(t2, t2, t3);
		fp12_mul(t2, t2, c);
		fp12_inv_uni(c, c);
		fp12_mul(c, c, t0);
		fp12_frb(c, c, 3);
		fp12_mul(c, c, t2);
		fp12_frb(t0, t0, 1);
		fp12_mul(c, c, t0);
		fp12_frb(t3, t3, 2);
		fp12_mul(c, c, t3);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp12_free(t0);
		fp12_free(t1);
		fp12_free(t2);
		fp12_free(t3);
		bn_free(x);
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

#if PP_MAP == TATEP || !defined(STRIP)

void pp_map_tatep(fp12_t r, ep_t p, ep2_t q) {
	ep_t t;
	bn_t n;

	ep_null(t);
	bn_null(n);

	TRY {
		ep_new(t);
		bn_new(n);

		ep_curve_get_ord(n);
		pp_mil_k12_lit(r, t, p, q, n);
		pp_exp(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep_free(t);
		bn_free(n);
	}
}

#endif

#if PP_MAP == WEILP || !defined(STRIP)

void pp_map_weilp(fp12_t r, ep_t p, ep2_t q) {
	ep_t t0;
	ep2_t t1;
	fp12_t r0, r1;
	bn_t n;

	ep_null(t0);
	ep2_null(t1);
	fp12_null(r0);
	fp12_null(r1);
	bn_null(n);

	TRY {
		ep_new(t0);
		ep2_new(t1);
		fp12_new(r0);
		fp12_new(r1);
		bn_new(n);

		ep_curve_get_ord(n);
		pp_mil_k12_lit(r0, t0, p, q, n);
		pp_mil_k12(r1, t1, q, p, n);
		fp12_inv(r1, r1);
		fp12_mul(r0, r0, r1);

		fp12_inv(r1, r0);
		fp12_inv_uni(r0, r0);
		fp12_mul(r, r0, r1);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep_free(t0);
		ep2_free(t1);
		fp12_null(r0);
		fp12_null(r1);
		bn_free(n);
	}
}

#endif


#if PP_MAP == OATEP || !defined(STRIP)

void pp_map_oatep(fp12_t r, ep_t p, ep2_t q) {
	ep2_t t, q1, q2;
	bn_t a;
	int len, s[FP_BITS];
	fp12_t l;

	ep2_null(t);
	ep2_null(q1);
	ep2_null(q2);
	fp12_null(l);
	bn_null(a);

	TRY {
		ep2_new(t);
		ep2_new(q1);
		ep2_new(q2);
		fp12_new(l);
		bn_new(a);

		fp_param_get_var(a);
		fp_param_get_map(s, &len);

		/* r = f_{r,Q}(P). */
		pp_mil_k12_sps(r, t, q, p, s, len);

		if (bn_sign(a) == BN_NEG) {
			/* Since f_{-a,Q}(P) = 1/f_{a,Q}(P), we must invert the result. */
			fp12_inv_uni(r, r);
			ep2_neg(t, t);
		}

		fp_set_dig(q1->z[0], 1);
		fp_zero(q1->z[1]);
		fp_set_dig(q2->z[0], 1);
		fp_zero(q2->z[1]);

		ep2_frb(q1, q, 1);
		ep2_frb(q2, q, 2);
		ep2_neg(q2, q2);

		fp12_zero(l);
		pp_add_k12(l, t, q1, p);
		fp12_mul_dxs(r, r, l);
		pp_add_k12(l, t, q2, p);
		fp12_mul_dxs(r, r, l);

		pp_exp(r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep2_free(t);
		bn_free(a);
	}
}

#endif

void pp_exp(fp12_t c, fp12_t a) {
	switch (ep_param_get()) {
		case BN_P158:
		case BN_P254:
		case BN_P256:
		case BN_P638:
			pp_exp_bn(c, a);
			break;
	}
}
