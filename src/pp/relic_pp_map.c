/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2014 RELIC Authors
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

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Compute the Miller loop for pairings of type G_2 x G_1 over the bits of a
 * given parameter.
 *
 * @param[out] r			- the result.
 * @param[out] t			- the resulting point.
 * @param[in] p				- the first point of the pairing, in G_1.
 * @param[in] q				- the second point of the pairing, in G_1.
 * @param[in] a				- the loop parameter.
 */
static void pp_mil_k2(fp2_t r, ep_t t, ep_t p, ep_t q, bn_t a) {
	fp2_t l;
	ep_t _q;

	fp2_null(l);
	ep_null(_q);

	TRY {
		fp2_new(l);
		ep_new(_q);

		fp2_zero(l);
		fp2_zero(r);
		fp_set_dig(r[0], 1);
		ep_copy(t, p);

		ep_neg(_q, q);

		for (int i = bn_bits(a) - 2; i >= 0; i--) {
			fp2_sqr(r, r);
			pp_dbl_k2(l, t, t, _q);
			fp2_mul(r, r, l);
			if (bn_get_bit(a, i)) {
				pp_add_k2(l, t, p, q);
				fp2_mul(r, r, l);
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(l);
		ep_free(_q);
	}
}

/**
 * Compute the Miller loop for pairings of type G_2 x G_1 over the bits of a
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

		/* Precomputing. */
#if EP_ADD == BASIC
		fp_copy(_p->x, p->x);
		fp_neg(_p->y, p->y);
#else
		fp_neg(_p->y, p->y);
		fp_add(_p->x, p->x, p->x);
		fp_add(_p->x, _p->x, p->x);
#endif

		pp_dbl_k12(r, t, t, _p);
		if (bn_get_bit(a, bn_bits(a) - 2)) {
			pp_add_k12(l, t, q, p);
			fp12_mul_dxs(r, r, l);
		}
		for (int i = bn_bits(a) - 3; i >= 0; i--) {
			fp12_sqr(r, r);
			pp_dbl_k12(l, t, t, _p);
			fp12_mul_dxs(r, r, l);
			if (bn_get_bit(a, i)) {
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
 * Compute the Miller loop for pairings of type G_2 x G_1 over the bits of a
 * given parameter represented in sparse form.
 *
 * @param[out] r			- the result.
 * @param[out] t			- the resulting point.
 * @param[in] q				- the first point of the pairing, in G_2.
 * @param[in] p				- the second point of the pairing, in G_1.
 * @param[in] s				- the loop parameter in sparse form.
 * @paramin] len			- the length of the loop parameter.
 */
static void pp_mil_sps_k12(fp12_t r, ep2_t t, ep2_t q, ep_t p, int *s, int len) {
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
		ep2_neg(_q, q);

#if EP_ADD == BASIC
		fp_copy(_p->x, p->x);
		fp_neg(_p->y, p->y);
#else
		fp_neg(_p->y, p->y);
		fp_add(_p->x, p->x, p->x);
		fp_add(_p->x, _p->x, p->x);
#endif

		pp_dbl_k12(r, t, t, _p);
		if (s[len - 2] > 0) {
			pp_add_k12(l, t, q, p);
			fp12_mul_dxs(r, r, l);
		}
		if (s[len - 2] < 0) {
			pp_add_k12(l, t, _q, p);
			fp12_mul_dxs(r, r, l);
		}
		for (int i = len - 3; i >= 0; i--) {
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

/**
 * Compute the Miller loop for pairings of type G_1 x G_2 over the bits of a
 * given parameter.
 *
 * @param[out] r			- the result.
 * @param[out] t			- the resulting point.
 * @param[in] p				- the first point of the pairing, in G_1.
 * @param[in] q				- the second point of the pairing, in G_2.
 * @param[in] a				- the loop parameter.
 */
static void pp_mil_lit_k12(fp12_t r, ep_t t, ep_t p, ep2_t q, bn_t a) {
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
			pp_dbl_lit_k12(l, t, t, q);
			fp12_mul(r, r, l);
			if (bn_get_bit(a, i)) {
				pp_add_lit_k12(l, t, p, q);
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
 * Compute the final lines for optimal ate pairings.
 *
 * @param[out] r			- the result.
 * @param[out] t			- the resulting point.
 * @param[in] q				- the first point of the pairing, in G_2.
 * @param[in] p				- the second point of the pairing, in G_1.
 * @param[in] a				- the loop parameter.
 */
static void pp_fin_k12_oatep(fp12_t r, ep2_t t, ep2_t q, ep_t p) {
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

		ep2_frb(q1, q, 1);
		ep2_frb(q2, q, 2);
		ep2_neg(q2, q2);

		pp_add_k12(tmp, t, q1, p);
		fp12_mul_dxs(r, r, tmp);
		pp_add_k12(tmp, t, q2, p);
		fp12_mul_dxs(r, r, tmp);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp12_free(tmp);
		ep2_free(q1);
		ep2_free(q2);
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

#if PP_MAP == TATEP || PP_MAP == OATEP || !defined(STRIP)

void pp_map_tatep_k2(fp2_t r, ep_t p, ep_t q) {
	ep_t t;
	bn_t n;

	ep_null(t);
	bn_null(n);

	TRY {
		ep_new(t);
		bn_new(n);

		ep_curve_get_ord(n);
		/* Since p has order n, we do not have to perform last iteration. */
		bn_sub_dig(n, n, 1);
		pp_mil_k2(r, t, p, q, n);
		pp_exp_k2(r, r);
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

#if PP_MAP == TATEP || !defined(STRIP)

void pp_map_tatep_k12(fp12_t r, ep_t p, ep2_t q) {
	ep_t t;
	bn_t n;

	ep_null(t);
	bn_null(n);

	TRY {
		ep_new(t);
		bn_new(n);

		ep_curve_get_ord(n);
		pp_mil_lit_k12(r, t, p, q, n);
		pp_exp_k12(r, r);
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

void pp_map_weilp_k2(fp2_t r, ep_t p, ep_t q) {
	ep_t t0;
	ep_t t1;
	fp2_t r0, r1;
	bn_t n;

	ep_null(t0);
	ep_null(t1);
	fp2_null(r0);
	fp2_null(r1);
	bn_null(n);

	TRY {
		ep_new(t0);
		ep_new(t1);
		fp2_new(r0);
		fp2_new(r1);
		bn_new(n);

		ep_curve_get_ord(n);
		/* Since p has order n, we do not have to perform last iteration. */
		bn_sub_dig(n, n, 1);
		pp_mil_k2(r0, t0, p, q, n);
		pp_mil_k2(r1, t1, q, p, n);
		fp2_inv(r1, r1);
		fp2_mul(r0, r0, r1);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep_free(t0);
		ep_free(t1);
		fp2_free(r0);
		fp2_free(r1);
		bn_free(n);
	}
}

void pp_map_weilp_k12(fp12_t r, ep_t p, ep2_t q) {
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
		pp_mil_lit_k12(r0, t0, p, q, n);
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
		fp12_free(r0);
		fp12_free(r1);
		bn_free(n);
	}
}

#endif


#if PP_MAP == OATEP || !defined(STRIP)

void pp_map_oatep_k12(fp12_t r, ep_t p, ep2_t q) {
	ep2_t t;
	bn_t a;
	int len = FP_BITS, s[FP_BITS];

	ep2_null(t);
	bn_null(a);

	TRY {
		ep2_new(t);
		bn_new(a);

		fp_param_get_var(a);
		bn_mul_dig(a, a, 6);
		bn_add_dig(a, a, 2);
		fp_param_get_map(s, &len);

		switch (ep_param_get()) {
			case BN_P158:
			case BN_P254:
			case BN_P256:
			case BN_P638:
				/* r = f_{|a|,Q}(P). */
				pp_mil_sps_k12(r, t, q, p, s, len);
				if (bn_sign(a) == BN_NEG) {
					/* f_{-a,Q}(P) = 1/f_{a,Q}(P). */
					fp12_inv_uni(r, r);
					ep2_neg(t, t);
				}
				pp_fin_k12_oatep(r, t, q, p);
				pp_exp_k12(r, r);
				break;
			case B12_P638:
				/* r = f_{|a|,Q}(P). */
				pp_mil_sps_k12(r, t, q, p, s, len);
				if (bn_sign(a) == BN_NEG) {
					fp12_inv_uni(r, r);
					ep2_neg(t, t);
				}
				pp_exp_k12(r, r);
				break;
		}
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
