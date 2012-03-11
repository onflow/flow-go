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
 * Implementation of divisor class addition on binary hyperelliptic curves.
 *
 * @version $Id$
 * @ingroup hb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_hb.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if defined(HB_SUPER)
/**
 * Adds two divisor classes represented in affine coordinates on a supersingular
 * binary hyperelliptic curve.
 *
 * @param r					- the result.
 * @param p					- the first divisor class to add.
 * @param q					- the second divisor class to add.
 */
static void hb_add_basic_super(hb_t r, hb_t p, hb_t q) {
	fb_t z0, z1, z2, z3, z4, z5, s0, s1, l0, l1, l2, res;
	hb_t t;

	fb_null(z0);
	fb_null(z1);
	fb_null(z2);
	fb_null(z3);
	fb_null(z4);
	fb_null(z5);
	fb_null(s0);
	fb_null(s1);
	fb_null(l0);
	fb_null(l1);
	fb_null(l2);
	fb_null(res);
	hb_null(t);

	TRY {
		fb_new(z0);
		fb_new(z1);
		fb_new(z2);
		fb_new(z3);
		fb_new(z4);
		fb_new(z5);
		fb_new(s0);
		fb_new(s1);
		fb_new(l0);
		fb_new(l1);
		fb_new(l2);
		fb_new(res);
		hb_new(t);

		/* Let p = [[u11, u10], [v11, v10]], q = [[u21, u20], [v21, v20]]. */

		if (!q->deg) {
			/* z1 = u11 - u21, z2 = u20 - u10. */
			fb_sub(z1, p->u1, q->u1);
			fb_sub(z2, q->u0, p->u0);
			/* z3 = u11 * z1 + z2, r = z2 * z3 + (z1^2) * u10. */
			fb_mul(z3, p->u1, z1);
			fb_add(z3, z3, z2);
			fb_mul(res, z2, z3);
			fb_sqr(z4, z1);
			fb_mul(z4, z4, p->u0);
			fb_add(res, res, z4);

			if (fb_is_zero(res)) {
				/* Recover x1. */
				if (fb_is_zero(p->u1)) {
					fb_srt(z0, p->u0);
				} else {
					/* Solve X^2 + X = u0/u1^.2 */
					fb_sqr(z1, p->u1);
					fb_inv(z1, z1);
					fb_mul(z1, z1, p->u0);
					fb_slv(z1, z1);
					fb_add_dig(z2, z1, 1);
					/* Revert change of variables by computing X = X * u1. */
					fb_mul(z1, z1, p->u1);
					fb_mul(z2, z2, p->u1);
					/* Solve X^2 + X = u0/u1^.2 */
					fb_sqr(z3, q->u1);
					fb_inv(z3, z3);
					fb_mul(z3, z3, q->u0);
					fb_slv(z3, z3);
					fb_add_dig(z4, z3, 1);
					/* Revert change of variables by computing X = X * u1. */
					fb_mul(z3, z3, q->u1);
					fb_mul(z4, z4, q->u1);
					if (fb_cmp(z1, z3) == CMP_EQ || fb_cmp(z1, z4) == CMP_EQ) {
						fb_copy(z0, z1);
					}
					if (fb_cmp(z2, z3) == CMP_EQ || fb_cmp(z2, z4) == CMP_EQ) {
						fb_copy(z0, z2);
					}
				}
				/* Compute v1(x1) and v2(x1). */
				fb_mul(z1, z0, p->v1);
				fb_add(z1, z1, p->v0);
				fb_mul(z2, z0, q->v1);
				fb_add(z2, z2, q->v0);
				/* p2 = (z3 = u11 + x1, z4 = v1(z3)) .*/
				fb_add(z3, p->u1, z0);
				fb_mul(z4, z3, p->v1);
				fb_add(z4, z4, p->v0);
				/* p3 = (s0 = u21 + x1, s1 = v2(s0)). */
				fb_add(s0, q->u1, z0);
				fb_mul(s1, s0, q->v1);
				fb_add(s1, s1, q->v0);
				/* If v1(x1) = v2(x1). */
				if (fb_cmp(z1, z2) == CMP_EQ) {
					/* r = 2 * (p1 - inf). */
					fb_set_dig(t->u1, 1);
					fb_copy(t->u0, z0);
					fb_zero(t->v1);
					fb_copy(t->v0, z1);
					t->deg = 1;
					hb_dbl_basic(r, t);
					/* r = r + (p2 - inf). */
					fb_set_dig(t->u1, 1);
					fb_neg(t->u0, z3);
					fb_zero(t->v1);
					fb_copy(t->v0, z4);
					t->deg = 1;
					hb_add_basic(r, r, t);
					/* r = r + (p3 - inf). */
					fb_set_dig(t->u1, 1);
					fb_neg(t->u0, s0);
					fb_zero(t->v1);
					fb_copy(t->v0, s1);
					t->deg = 1;
					hb_add_basic(r, r, t);
				} else {
					/* r = p2 + p3 - 2*inf. */
					fb_set_dig(r->u1, 1);
					fb_neg(r->u0, z3);
					fb_zero(r->v1);
					fb_copy(r->v0, z4);
					r->deg = 1;
					fb_set_dig(t->u1, 1);
					fb_neg(t->u0, s0);
					fb_zero(t->v1);
					fb_copy(t->v0, s1);
					t->deg = 1;
					hb_add_basic(r, r, t);
				}
			} else {
				/* inv1 = z1, inv0 = z3. */
				/* z0 = w0 = v10 - v20, z4 = w1 = v11 - v21. */
				fb_sub(z0, p->v0, q->v0);
				fb_sub(z4, p->v1, q->v1);
				/* s1 = (inv0 + inv1)(w0 + w1) - w2 - w3 * (1 + u11). */
				/* s1 = inv0 * w1 + inv1 * w0 - inv1 * w1 * u11. */
				/* s1 = inv1 * w0 + w1 * z2. */
				fb_mul(s1, z1, z0);
				fb_mul(s0, z2, z4);
				fb_add(s1, s1, s0);
				/* s0 = w2 - u10 * w3 = z3 * w0 - u10 * w3. */
				fb_mul(s0, z3, z0);
				fb_mul(z3, z1, z4);
				fb_mul(z3, z3, p->u0);
				fb_sub(s0, s0, z3);

				/* If s1 != 0. */
				if (!fb_is_zero(s1)) {
					/* z0 = w1 = 1/(r * s1), z2 = w2 = r * w1, z3 = w3 = (s1^2) * w1. */
					fb_mul(z0, res, s1);
					fb_inv(z0, z0);
					fb_mul(z2, res, z0);
					fb_sqr(z3, s1);
					fb_mul(z3, z3, z0);
					/* z4 = w4 = r * w2, z5 = w5 = w4^2, s0 = s0 * w2. */
					fb_mul(z4, res, z2);
					fb_sqr(z5, z4);
					fb_mul(s0, s0, z2);

					/* l2 = u21 + s0, l1 = u21 * s0 + u20, l0 = u20 * s0. */
					fb_add(l2, q->u1, s0);
					fb_mul(l1, q->u1, s0);
					fb_add(l1, l1, q->u0);
					fb_mul(l0, q->u0, s0);
					/* u30 = (s0 - u11)(s0 - z1 + (h2 = 0) * w4) - u10. */
					fb_sub(z0, s0, p->u1);
					fb_sub(s0, s0, z1);
					fb_mul(z0, z0, s0);
					fb_sub(r->u0, z0, p->u0);

					/* u31 = 2 * s0 - z1 + h2 * w4 - w5 = z1 + w5. */
					fb_add(r->u1, z1, z5);
					/* u30 = u30 + l1 + (h1 + 2*v1) * w4 + (2 * u21 + z1 - f4) * w5. */
					/* u30 = u30 + l1 + z1 * w5. */
					fb_add(r->u0, r->u0, l1);
					fb_mul(z0, z1, z5);
					fb_add(r->u0, r->u0, z0);
					/* w1 = l2 - u31, w2 = u31 * w1 + u30 - l1. */
					fb_sub(z1, l2, r->u1);
					fb_mul(z2, r->u1, z1);
					fb_add(z2, z2, r->u0);
					fb_sub(z2, z2, l1);
					/* v31 = w2 * w3 - v21 - h1 + h2 * u31 = w2 * w3 - v21. */
					fb_mul(z0, z2, z3);
					fb_sub(r->v1, z0, q->v1);
					/* w2 = u30 * w1 - l0. */
					fb_mul(z2, r->u0, z1);
					fb_sub(z2, z2, l0);
					/* v30 = w2 * w3 - v20 - h0 + h2 * u30 = w2 * w3 - v20 - 1. */
					fb_mul(z0, z2, z3);
					fb_sub(r->v0, z0, q->v0);
					fb_sub_dig(r->v0, r->v0, 1);

					r->deg = 0;
				} else {
					/* inv = 1/r, s0 = s0 * inv. */
					fb_inv(res, res);
					fb_mul(s0, s0, res);
					/* w2 = u20 * s0 + v20 + h0. */
					fb_mul(z2, q->u0, s0);
					fb_add(z2, z2, q->v0);
					fb_add_dig(z2, z2, 1);
					/* u30 = f4 - u21 - u11 - s0^2 - s0 * h2 = u21 - u11 - s0^2. */
					fb_sqr(z0, s0);
					fb_sub(r->u0, z1, z0);
					/* w1 = s0 * (u21 + u30) + (h1 = 0) + v21 - (h2 = 0) * u30. */
					fb_add(z1, r->u0, q->u1);
					fb_mul(z1, z1, s0);
					fb_add(z1, z1, q->v1);
					/* v30 = u30 * w1 - w2. */
					fb_mul(r->v0, r->u0, z1);
					fb_sub(r->v0, r->v0, z2);
					/* u31 = 1, v31 = 0. */
					fb_set_dig(r->u1, 1);
					fb_zero(r->v1);

					r->deg = 1;
				}
			}
		} else {
			if (!p->deg) {
				/* res = u10 - (u11 - u20) * u20. */
				fb_sub(res, p->u1, q->u0);
				fb_mul(res, res, q->u0);
				fb_sub(res, p->u0, res);
				fb_copy(l0, res);

				if (!fb_is_zero(res)) {
					/* z3 = inv = 1/res. */
					fb_inv(z3, res);
					/* s0 = inv * (v20 - v10 - v11 * u20). */
					fb_mul(s0, p->v1, q->u0);
					fb_add(s0, p->v0, s0);
					fb_sub(s0, q->v0, s0);
					fb_mul(s0, z3, s0);

					/* l1 = s0 * u11, l0 = s0 * u10, delayed. */
					fb_copy(l1, p->u1);
					fb_copy(l0, p->u0);
					/* t2 = f4 - u11, t1 = f3 - (f4 - u11) * u11 - v11 * h2 - u10. */
					/* t2 = u11, t1 = f3 + u11^2 - u10, delayed. */

					/* u31 = t2 - s0^2 - s0*h2 - u20 = u11 - s0^2 - u20. */
					fb_sqr(z2, s0);
					fb_sub(r->u1, p->u1, z2);
					fb_sub(r->u1, r->u1, q->u0);
					/* u30 = t1 - s0 * (l1 + h1 + 2*v11) - u20 * u31. */
					/* u30 = t1 - s0 * l1 - u20 * u31 = t1 - s0^2 * u11 - u20 * u31. */
					/* u30 = t1 - s0^2 * u11 - u20 * (u11 - s0^2 - u20). */
					/* u30 = f3 + u11^2 - s0^2 * (u11 - u20) + r. */
					fb_sqr(z1, l1);
					fb_add(z1, z1, hb_curve_get_f3());
					fb_add(z3, l1, q->u0);
					fb_mul(z3, z2, z3);
					fb_add(z3, z3, res);
					fb_sub(r->u0, z1, z3);
					/* v31 = (h2 + s0)*u31 - (h1 + l1 + v11). */
					/* v31 = s0 * u31 - l1 - v11 = s0 * (u31 - u11) - v11. */
					fb_sub(z2, r->u1, l1);
					fb_mul(z2, s0, z2);
					fb_sub(r->v1, z2, p->v1);
					/* v30 = (h2 + s0)*u30 - (h0 + l0 + v10). */
					/* v30 = s0 * u30 - 1 - l0 - v10 = s0 * (u30 - u10) - 1 - v10. */
					fb_sub(z2, r->u0, l0);
					fb_mul(z2, s0, z2);
					fb_sub(r->v0, z2, p->v0);
					fb_sub_dig(r->v0, r->v0, 1);

					r->deg = 0;
				} else {
					fb_sqr(z0, q->u0);
					if (fb_is_zero(p->u1) && (fb_cmp(p->u0, z0) == CMP_EQ)) {
						hb_dbl_basic(t, p);
						hb_sub_basic(r, t, q);
					} else {
						/* z0 = v11 * u20 + v10 + 1. */
						fb_mul(z0, p->v1, q->u0);
						fb_add(z0, z0, p->v0);
						fb_add_dig(z0, z0, 1);
						if (fb_cmp(z0, q->v0) == CMP_EQ) {
							/* u30 = u11 + q20. */
							fb_add(r->u0, p->u1, q->u0);
							/* u31 = 1. */
							fb_set_dig(r->u1, 1);
							/* v30 = v10 + v11 * u30. */
							fb_mul(z0, p->v1, r->u0);
							fb_add(r->v0, z0, p->v0);
							/* v31 = 0. */
							fb_zero(r->v1);

							r->deg = 1;
						} else {
							hb_dbl_basic(t, q);
							/* u30 = u11 + q20. */
							fb_add(r->u0, p->u1, q->u0);
							/* u31 = 1. */
							fb_set_dig(r->u1, 1);
							/* v30 = v10 + v11 * u30. */
							fb_mul(z0, p->v1, r->u0);
							fb_add(r->v0, z0, p->v0);
							/* v31 = 0. */
							fb_zero(r->v1);
							hb_add_basic(r, r, t);
						}

					}
				}
			} else {
				/* u31 = u10 + u20. */
				fb_add(r->u1, p->u0, q->u0);
				if (fb_is_zero(r->u1)) {
					if (fb_cmp(r->v0, q->v0) == CMP_EQ) {
						hb_dbl_basic(r, p);
					} else {
						hb_set_infty(r);
					}
				} else {
					/* inv = 1/u31. */
					fb_inv(z0, r->u1);
					/* v31 = inv * (v10 + v20). */
					fb_add(z1, p->v0, q->v0);
					fb_mul(r->v1, z0, z1);
					/* v30 = inv * (v10 * u20 + u10 * v20). */
					fb_mul(z1, p->v0, q->u0);
					fb_mul(z2, p->u0, q->v0);
					fb_add(z1, z1, z2);
					fb_mul(r->v0, z0, z1);
					/* u30 = u10 * u20. */
					fb_mul(r->u0, p->u0, q->u0);
				}
				r->deg = 0;
			}
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(z0);
		fb_free(z1);
		fb_free(z2);
		fb_free(z3);
		fb_free(z4);
		fb_free(z5);
		fb_free(s0);
		fb_free(s1);
		fb_free(l0);
		fb_free(l1);
		fb_free(l2);
		fb_free(res);
		hb_free(t);
	}
}

#endif /* EB_SUPER */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void hb_add_basic(hb_t r, hb_t p, hb_t q) {
	if (hb_is_infty(p)) {
		hb_copy(r, q);
		return;
	}

	if (hb_is_infty(q)) {
		hb_copy(r, p);
		return;
	}

	if (p == q) {
		hb_dbl(r, p);
		return;
	}
#if defined(HB_SUPER)
	if (hb_curve_is_super()) {
		if (hb_cmp(p, q) == CMP_EQ) {
			hb_dbl_basic(r, p);
			return;
		}
		fb_add_dig(q->v0, q->v0, 1);
		if (hb_cmp(p, q) == CMP_EQ) {
			hb_set_infty(r);
			return;
		} else {
			fb_add_dig(q->v0, q->v0, 1);
			if (!p->deg) {
				hb_add_basic_super(r, p, q);
			} else {
				hb_add_basic_super(r, q, p);
			}
			return;
		}
	}
#endif
}

void hb_sub_basic(hb_t r, hb_t p, hb_t q) {
	hb_t t;

	hb_null(t);

	if (p == q) {
		hb_set_infty(r);
		return;
	}

	TRY {
		hb_new(t);

		hb_neg_basic(t, q);
		hb_add_basic(r, p, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		hb_free(t);
	}
}
