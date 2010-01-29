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
 * Implementation of elliptic prime curves over quadratic extensions.
 *
 * @version $Id$
 * @ingroup pp
 */

#include "relic_core.h"
#include "relic_pp.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if defined(EP_ORDIN) && FP_PRIME == 256
/**
 * Parameters for a pairing-friendly prime curve over a quadratic extension.
 */
/** @{ */
#define BNN_P256_X0		"1822AA754FAFAFF95FE37842D7D5DECE88305EC19B363F6681DF06BF405F02B4"
#define BNN_P256_X1		"1AB4CC8A133A7AA970AADAE37C20D1C7191279CBA02830AFC64C19B50E8B1997"
#define BNN_P256_Y0		"16737CF6F9DEC5895A7E5A6D60316763FB6638A0A82F26888E909DA86F7F84BA"
#define BNN_P256_Y1		"05B6DB6FF5132FB917E505627E7CCC12E0CE9FCC4A59805B3B730EE0EC44E43C"
#define BNN_P256_R		"5633FF249938445904D63EF07C4DCC56A3D53BC318AC022A68794AE6A80008F79BB4B3140000188AFC77D00000002B2EBA10000000002C68200000000000145L"
/** @} */
#endif

/**
 * The generator of the elliptic curve.
 */
static ep2_st curve_g;

/**
 * The first coordinate of the generator.
 */
static fp2_st curve_gx;

/**
 * The second coordinate of the generator.
 */
static fp2_st curve_gy;

/**
 * The third coordinate of the generator.
 */
static fp2_st curve_gz;

/**
 * The order of the group of points in the elliptic curve.
 */
static bn_st curve_r;

/**
 * Flag that stores if the configured prime elliptic curve is twisted.
 */
static int curve_is_twist;

#if EP_ADD == BASIC || defined(EP_MIXED) || !defined(STRIP)

/**
 * Adds two points represented in affine coordinates on an ordinary prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param s					- the resulting slope.
 * @param p					- the first point to add.
 * @param q					- the second point to add.
 */
static void ep2_add_basic_impl(ep2_t r, fp2_t s, ep2_t p, ep2_t q) {
	fp2_t t0, t1, t2;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);

		/* t0 = x2 - x1. */
		fp2_sub(t0, q->x, p->x);
		/* t1 = y2 - y1. */
		fp2_sub(t1, q->y, p->y);

		/* If t0 is zero. */
		if (fp2_is_zero(t0)) {
			if (fp2_is_zero(t1)) {
				/* If t1 is zero, q = p, should have doubled. */
				ep2_dbl_basic(r, p);
			} else {
				/* If t1 is not zero and t0 is zero, q = -p and r = infty. */
				ep2_set_infty(r);
			}
		} else {

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
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
	}
}

/**
 * Doubles a point represented in affine coordinates on an ordinary prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param s					- the resulting slope.
 * @param p					- the point to double.
 */
static void ep2_dbl_basic_impl(ep2_t r, fp2_t s, fp2_t e, ep2_t p) {
	fp2_t t0, t1, t2;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);

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
				case OPT_ZERO:
					break;
				case OPT_ONE:
					fp_set_dig(t2[0], 1);
					fp2_mul_art(t2, t2);
					fp2_mul_art(t2, t2);
					fp2_add(t1, t1, t2);
					break;
				case OPT_DIGIT:
					fp_set_dig(t2[0], ep_curve_get_a()[0]);
					fp2_mul_art(t2, t2);
					fp2_mul_art(t2, t2);
					fp2_add(t1, t1, t2);
					break;
				default:
					fp_copy(t2[0], ep_curve_get_a());
					fp_zero(t2[1]);
					fp2_mul_art(t2, t2);
					fp2_mul_art(t2, t2);
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

		r->norm = 1;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fp2_free(t0);
		fp2_free(t1);
		fp2_free(t2);
	}
}

#endif /* EP_ADD == BASIC */

#if EP_ADD == PROJC || defined(EP_MIXED) || !defined(STRIP)

/**
 * Adds two points represented in projective coordinates on an ordinary prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param s					- the resulting slope.
 * @param p					- the first point to add.
 * @param q					- the second point to add.
 */
static void ep2_add_projc_impl(ep2_t r, fp2_t s, ep2_t p, ep2_t q) {
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
	}
}

/**
 * Doubles a point rep2resented in affine coordinates on an ordinary prime
 * elliptic curve.
 *
 * @param r					- the result.
 * @param p					- the point to double.
 */
static void ep2_dbl_projc_impl(ep2_t r, fp2_t s, fp2_t e, ep2_t p) {
	fp2_t t0, t1, t2, t3;

	fp2_null(t0);
	fp2_null(t1);
	fp2_null(t2);
	fp2_null(t3);

	TRY {
		fp2_new(t0);
		fp2_new(t1);
		fp2_new(t2);
		fp2_new(t3);

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
	}
}

#endif /* EP_ADD == BASIC */

#if EP_ADD == PROJC || defined(EP_MIXED)

/**
 * Normalizes a point represented in projective coordinates.
 *
 * @param r			- the result.
 * @param p			- the point to normalize.
 */
void ep2_norm_impl(ep2_t r, ep2_t p) {
	if (!p->norm) {
		fp2_t t0, t1;

		fp2_null(t0);
		fp2_null(t1);

		TRY {
			fp2_new(t0);
			fp2_new(t1);

			fp2_inv(t1, p->z);
			fp2_sqr(t0, t1);
			fp2_mul(r->x, p->x, t0);
			fp2_mul(t0, t0, t1);
			fp2_mul(r->y, p->y, t0);
			fp_zero(r->z[0]);
			fp_zero(r->z[1]);
			fp_set_dig(r->z[0], 1);
		}
		CATCH_ANY {
			THROW(ERR_CAUGHT);
		}
		FINALLY {
			fp2_free(t0);
			fp2_free(t1);
		}
	}

	r->norm = 1;
}

#endif /* EP_ADD == PROJC || EP_MIXED */

/*============================================================================*/
	/* Public definitions                                                         */
/*============================================================================*/

void ep2_curve_init(void) {
#if ALLOC == STATIC
	fp2_new(curve_gx);
	fp2_new(curve_gy);
	fp2_new(curve_gz);
#endif
	curve_g.x[0] = curve_gx[0];
	curve_g.x[1] = curve_gx[1];
	curve_g.y[0] = curve_gy[0];
	curve_g.y[1] = curve_gy[1];
	curve_g.z[0] = curve_gz[0];
	curve_g.z[1] = curve_gz[1];
	ep2_set_infty(&curve_g);
	bn_init(&curve_r, FP_DIGS);
}

void ep2_curve_clean(void) {
#if ALLOC == STATIC
	fp2_free(curve_gx);
	fp2_free(curve_gy);
	fp2_free(curve_gz);
#endif
	bn_clean(&curve_r);
}

int ep2_curve_is_twist() {
	return curve_is_twist;
}

ep2_t ep2_curve_get_gen() {
	return &curve_g;
}

bn_t ep2_curve_get_ord() {
	if (curve_is_twist) {
		return ep_curve_get_ord();
	} else {
		return &curve_r;
	}
}

void ep2_curve_set_twist(int twist) {
	curve_is_twist = twist;
}

void ep2_curve_set() {
	int param;
	char *str;
	ep2_t g;
	bn_t r;

	ep2_null(g);
	bn_null(r);

	TRY {
		ep2_new(g);
		bn_new(r);

		param = ep_param_get();

		switch (param) {
#if FP_PRIME == 256
			case BNN_P256:
				fp_read(g->x[0], BNN_P256_X0, strlen(BNN_P256_X0), 16);
				fp_read(g->x[1], BNN_P256_X1, strlen(BNN_P256_X1), 16);
				fp_read(g->y[0], BNN_P256_Y0, strlen(BNN_P256_Y0), 16);
				fp_read(g->y[1], BNN_P256_Y1, strlen(BNN_P256_Y1), 16);
				bn_read_str(r, BNN_P256_R, strlen(BNN_P256_R), 16);
				break;
#endif
			default:
				(void)str;
				THROW(ERR_INVALID);
				break;
		}

		fp2_zero(g->z);
		fp_set_dig(g->z[0], 1);
		g->norm = 1;

		ep2_copy(&curve_g, g);
		bn_copy(&curve_r, r);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep2_free(g);
		bn_free(r);
	}
}

int ep2_is_infty(ep2_t p) {
	return (fp2_is_zero(p->z) == 1);
}

void ep2_set_infty(ep2_t p) {
	fp2_zero(p->x);
	fp2_zero(p->y);
	fp2_zero(p->z);
}

void ep2_copy(ep2_t r, ep2_t p) {
	fp2_copy(r->x, p->x);
	fp2_copy(r->y, p->y);
	fp2_copy(r->z, p->z);
	r->norm = p->norm;
}

int ep2_cmp(ep2_t p, ep2_t q) {
	if (fp2_cmp(p->x, q->x) != CMP_EQ) {
		return CMP_NE;
	}

	if (fp2_cmp(p->y, q->y) != CMP_EQ) {
		return CMP_NE;
	}

	if (fp2_cmp(p->z, q->z) != CMP_EQ) {
		return CMP_NE;
	}

	return CMP_EQ;
}

void ep2_rand(ep2_t p) {
	bn_t n, k;

	bn_new(k);

	n = ep2_curve_get_ord();

	bn_rand(k, BN_POS, bn_bits(n));
	bn_mod(k, k, n);

	ep2_mul(p, ep2_curve_get_gen(), k);

	bn_free(k);
}

void ep2_print(ep2_t p) {
	fp2_print(p->x);
	fp2_print(p->y);
	fp2_print(p->z);
}

#if EP_ADD == BASIC || defined(EP_MIXED) || !defined(STRIP)

void ep2_neg_basic(ep2_t r, ep2_t p) {
	if (ep2_is_infty(p)) {
		ep2_set_infty(r);
		return;
	}

	if (r != p) {
		fp2_copy(r->x, p->x);
		fp2_copy(r->z, p->z);
	}

	fp2_neg(r->y, p->y);

	r->norm = 1;
}

void ep2_add_basic(ep2_t r, ep2_t p, ep2_t q) {
	if (ep2_is_infty(p)) {
		ep2_copy(r, q);
		return;
	}

	if (ep2_is_infty(q)) {
		ep2_copy(r, p);
		return;
	}

	ep2_add_basic_impl(r, NULL, p, q);
}

void ep2_add_slp_basic(ep2_t r, fp2_t s, ep2_t p, ep2_t q) {
	if (ep2_is_infty(p)) {
		ep2_copy(r, q);
		return;
	}

	if (ep2_is_infty(q)) {
		ep2_copy(r, p);
		return;
	}

	ep2_add_basic_impl(r, s, p, q);
}

void ep2_sub_basic(ep2_t r, ep2_t p, ep2_t q) {
	ep2_t t;

	ep2_null(t);

	if (p == q) {
		ep2_set_infty(r);
		return;
	}

	TRY {
		ep2_new(t);

		ep2_neg_basic(t, q);
		ep2_add_basic(r, p, t);

		r->norm = 1;
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep2_free(t);
	}
}

void ep2_dbl_basic(ep2_t r, ep2_t p) {
	if (ep2_is_infty(p)) {
		ep2_set_infty(r);
		return;
	}

	ep2_dbl_basic_impl(r, NULL, NULL, p);
}

void ep2_dbl_slp_basic(ep2_t r, fp2_t s, fp2_t e, ep2_t p) {
	if (ep2_is_infty(p)) {
		ep2_set_infty(r);
		return;
	}

	ep2_dbl_basic_impl(r, s, e, p);
}

#endif

#if EP_ADD == PROJC || defined(EP_MIXED) || !defined(STRIP)

void ep2_neg_projc(ep2_t r, ep2_t p) {
	if (ep2_is_infty(p)) {
		ep2_set_infty(r);
		return;
	}

	if (p->norm) {
		ep2_neg_basic(r, p);
		return;
	}

	if (r != p) {
		fp2_copy(r->x, p->x);
		fp2_copy(r->z, p->z);
	}

	fp2_neg(r->y, p->y);

	r->norm = 0;
}

void ep2_add_projc(ep2_t r, ep2_t p, ep2_t q) {
	if (ep2_is_infty(p)) {
		ep2_copy(r, q);
		return;
	}

	if (ep2_is_infty(q)) {
		ep2_copy(r, p);
		return;
	}

	/*
	 * TODO: Change this to ep2_add_proj_impl and sort the large code problem
	 * with add_projc functions.
	 */
	ep2_add_basic_impl(r, NULL, p, q);
}

void ep2_add_slp_projc(ep2_t r, fp2_t s, ep2_t p, ep2_t q) {
	if (ep2_is_infty(p)) {
		ep2_copy(r, q);
		return;
	}

	if (ep2_is_infty(q)) {
		ep2_copy(r, p);
		return;
	}

	ep2_add_projc_impl(r, s, p, q);
}

void ep2_sub_projc(ep2_t r, ep2_t p, ep2_t q) {
	ep2_t t;

	ep2_null(t);

	if (p == q) {
		ep2_set_infty(r);
		return;
	}

	TRY {
		ep2_new(t);

		ep2_neg_projc(t, q);
		ep2_add_projc(r, p, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ep2_free(t);
	}
}

void ep2_dbl_projc(ep2_t r, ep2_t p) {
	if (ep2_is_infty(p)) {
		ep2_set_infty(r);
		return;
	}

	ep2_dbl_projc_impl(r, NULL, NULL, p);
}

void ep2_dbl_slp_projc(ep2_t r, fp2_t s, fp2_t e, ep2_t p) {
	if (ep2_is_infty(p)) {
		ep2_set_infty(r);
		return;
	}

	ep2_dbl_projc_impl(r, s, e, p);
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
		ep2_set_infty(t);
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

void ep2_norm(ep2_t r, ep2_t p) {
	if (ep2_is_infty(p)) {
		ep2_set_infty(r);
		return;
	}

	if (p->norm) {
		/* If the point is represented in affine coordinates, we just copy it. */
		ep2_copy(r, p);
	}
#if EP_ADD == PROJC || !defined(STRIP)
	ep2_norm_impl(r, p);
#endif
}

void ep2_frb(ep2_t r, ep2_t p) {
	fp2_t t;

	fp2_null(t);

	TRY {
		fp2_new(t);

		fp2_const_get(t);

		fp2_frb(r->x, p->x);
		fp2_frb(r->y, p->y);
		fp2_mul(r->y, r->y, t);
		fp2_sqr(t, t);
		fp2_mul(r->x, r->x, t);
		fp2_mul(r->y, r->y, t);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t);
	}
}
