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
 * Implementation of the eta_t bilinear pairing over genus 2 supersingular
 * curves.
 *
 * @version $Id$
 * @ingroup pb
 */

#include "relic_core.h"
#include "relic_hb.h"
#include "relic_pb.h"
#include "relic_util.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#define U0		0
#define U1		1
#define V0		2
#define V1		3
#define DELTA0	4
#define DELTA1	5
#define DELTA2	6
#define EPSIL2	7
#define B00		8
#define B01		9
#define B02		10
#define B03		11
#define B04		12
#define B05		13
#define B10		14
#define B20		15
#define B40		16
#define D00		17
#define D01		18
#define D02		19
#define D10		20
#define D12		21
#define D20		23

static void fb12_mul_dxs4(fb12_t c, fb12_t a, fb12_t b) {
	fb6_t t0, t1, t6;

	fb6_null(t0);
	fb6_null(t1);
	fb6_null(t6);

	TRY {
		fb6_new(t0);
		fb6_new(t1);
		fb6_new(t6);
		fb6_add(t0, a[0], a[1]);
		fb6_add(t1, b[0], b[1]);
		fb6_mul(t6, t0, t1);
		fb6_mul(t0, a[0], b[0]);
		fb6_zero(t1);
		fb_mul(t1[0], a[1][0], b[1][0]);
		fb_mul(t1[1], a[1][1], b[1][0]);
		fb_mul(t1[2], a[1][2], b[1][0]);
		fb_mul(t1[4], a[1][4], b[1][0]);
		fb6_mul_nor(t1, t1);
		fb6_add(c[0], t0, t1);
		fb6_add(c[1], t0, t6);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb6_free(t0);
		fb6_free(t1);
		fb6_free(t6);
	}
}

/**
 * Computes the final exponentiation of the eta_t pairing over genus 2 curves.
 *
 * This function maps a random coset element to a fixed coset representative.
 *
 * @param[out] r					- the result.
 * @param[in] a						- the random coset element.
 */
static void pb_map_exp(fb12_t r, fb12_t a) {
	fb12_t v, w;
	int i, to;

	fb12_null(v);
	fb12_null(w);

	TRY {
		fb12_new(v);
		fb12_new(w);

		/* Compute f = f^(2^(6m) - 1). */
		fb12_inv(v, a);
		fb6_add(r[0], a[0], a[1]);
		fb6_copy(r[1], a[1]);
		fb12_mul(r, r, v);

		/* r = f^(2^2m + 1). */
		fb12_copy(v, r);
		fb12_frb(v, v);
		fb12_frb(v, v);
		fb12_mul(r, r, v);

		/* v = f^(m+1)/2. */
		to = ((FB_BITS + 1) / 2) / 6;
		to = to * 6;
		fb12_copy(v, r);
		for (i = 0; i < 6; i++) {
			/* This is faster than calling fb12_sqr(alpha, alpha) (no field additions). */
			fb_itr(v[0][i], v[0][i], to, pb_map_get_tab());
			fb_itr(v[1][i], v[1][i], to, pb_map_get_tab());
		}
		if ((to / 6) % 2 == 1) {
			fb6_add(v[0], v[0], v[1]);
		}
		for (i = to; i < (FB_BITS + 1) / 2; i++) {
			fb12_sqr(v, v);
		}
		fb12_frb(w, v);
		fb12_mul(w, w, v);
		fb6_add(w[0], w[0], w[1]);

		/* v = f^(2^2m + 2^m + 1). */
		fb12_frb(v, r);
		fb12_mul(r, r, v);
		fb12_frb(v, v);
		fb12_mul(r, r, v);
		fb12_mul(r, r, w);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb12_free(v);
		fb12_free(w);
	}
}

/**
 * Computes the etat pairing with points on a genus 2 curve.
 *
 * @param[out] r			- the result.
 * @param[in] xp			- the x-coordinate of the first point.
 * @param[in] yp			- the y-coordinate of the first point.
 * @param[in] xp			- the x-coordinate of the second point.
 * @param[in] yp			- the y-coordinate of the second point.
 */
void pb_map_etat2_dxd(fb12_t r, fb_t xp, fb_t yp, fb_t xq, fb_t yq) {
	fb12_t alpha, beta;
	fb_t x1[FB_BITS], x2[FB_BITS], y1[FB_BITS], y2[FB_BITS], t;
	int i, k1, k2, k3, k4, k5, k6;
	dig_t gamma;

	/* Set gamma = 1 if m = 1 (mod 4); otherwise, gamma = 0. */
	if (FB_BITS % 4 == 1) {
		gamma = 1;
	} else {
		gamma = 0;
	}

	fb12_null(alpha);
	fb12_null(beta);
	for (i = 0; i < FB_BITS; i++) {
		fb_null(x1[i]);
		fb_null(x2[i]);
		fb_null(y1[i]);
		fb_null(y2[i]);
	}
	fb_null(t);

	TRY {
		fb12_new(alpha);
		fb12_new(beta);
		for (i = 0; i < FB_BITS; i++) {
			fb_new(x1[i]);
			fb_new(x2[i]);
			fb_new(y1[i]);
			fb_new(y2[i]);
		}
		fb_new(t);

		/* f = 1. */
		fb12_zero(r);
		fb_set_dig(r[0][0], 1);
		/* x1[i] = xp^2^i, y1[i] = yp^2^i. */
		fb_copy(x1[0], xp);
		fb_copy(y1[0], yp);
		/* x2[i] = xq1^2^i, y2[i] = yq1^2^i. */
		fb_copy(x2[0], xq);
		fb_copy(y2[0], yq);
		for (i = 1; i < FB_BITS; i++) {
			fb_sqr(x1[i], x1[i - 1]);
			fb_sqr(x2[i], x2[i - 1]);
			fb_sqr(y1[i], y1[i - 1]);
			fb_sqr(y2[i], y2[i - 1]);
		}

		fb12_zero(alpha);
		fb_set_dig(alpha[1][0], 1);
		fb12_zero(beta);
		fb_set_dig(beta[1][0], 1);
		for (i = 0; i < (FB_BITS - 1) / 2; i++) {
			/* k1 = (3 * m - 9 - 6 * i)/2 mod m. */
			k1 = (3 * FB_BITS - 9 - 6 * i) / 2;
			k1 %= FB_BITS;
			/* k2 = (k1 + 1) mod m. */
			k2 = (k1 + 1) % FB_BITS;
			/* k3 = (k2 + 1) mod m. */
			k3 = (k2 + 1) % FB_BITS;
			/* k4 = (3 * m - 3 + 6 * i)/2 mod m. */
			k4 = (3 * FB_BITS - 3 + 6 * i) / 2;
			k4 %= FB_BITS;
			/* k5 = (k4 + 1) mod m. */
			k5 = (k4 + 1) % FB_BITS;
			/* k6 = (k5 + 1) mod m. */
			k6 = (k5 + 1) % FB_BITS;

			/* Compute alpha = a + b * w + c * w^2 + d * w^4 + s_0. */
			/* Compute d = x1[k4] + x1[k5]. */
			fb_add(alpha[0][4], x1[k4], x1[k5]);
			/* Compute c = x2[k3] + x1[k4] + 1. */
			fb_add(alpha[0][2], x2[k3], x1[k4]);
			fb_add_dig(alpha[0][2], alpha[0][2], 1);
			/* Compute a = y2[k2] + c * x2[k2] + d * x2[k3] + y1[k4] + gamma. */
			fb_mul(t, alpha[0][2], x2[k2]);
			fb_add(alpha[0][0], y2[k2], t);
			fb_mul(t, alpha[0][4], x2[k3]);
			fb_add(alpha[0][0], alpha[0][0], t);
			fb_add(alpha[0][0], alpha[0][0], y1[k4]);
			fb_add_dig(alpha[0][0], alpha[0][0], gamma);
			/* Compute b = x2[k3] + x2[k2]. */
			fb_add(alpha[0][1], x2[k3], x2[k2]);

			fb_copy(t, alpha[0][4]);
			fb_copy(alpha[0][4], alpha[0][2]);
			fb_copy(alpha[0][2], alpha[0][1]);
			fb_add(alpha[0][0], alpha[0][0], t);
			fb_copy(alpha[0][1], t);
			fb_add(alpha[0][2], alpha[0][2], t);
			fb_add(alpha[0][4], alpha[0][4], t);

			/* Compute beta = e + f2 * w + g * w^2 + h * w^4 + s_0. */
			/* Compute f2 = x1[k5] + x1[k6]. */
			fb_add(beta[0][1], x1[k5], x1[k6]);
			/* Compute e = y2[k1] + f2 * x2[k1] + y1[k5] + x1[k6] * (x1[k5] + x2[k2]) + x1[k5] + gamma. */
			fb_mul(t, beta[0][1], x2[k1]);
			fb_add(beta[0][0], y2[k1], t);
			fb_add(beta[0][0], beta[0][0], y1[k5]);
			fb_add(t, x1[k5], x2[k2]);
			fb_mul(t, t, x1[k6]);
			fb_add(beta[0][0], beta[0][0], t);
			fb_add(beta[0][0], beta[0][0], x1[k5]);
			fb_add_dig(beta[0][0], beta[0][0], gamma);
			/* Compute g = x2[k1] + x1[k6] + 1. */
			fb_add(beta[0][2], x2[k1], x1[k6]);
			fb_add_dig(beta[0][2], beta[0][2], 1);
			/* Compute h = x2[k2] + x2[k1]. */
			fb_add(beta[0][4], x2[k2], x2[k1]);

			fb_copy(t, beta[0][4]);
			fb_copy(beta[0][4], beta[0][2]);
			fb_copy(beta[0][2], beta[0][1]);
			fb_add(beta[0][0], beta[0][0], t);
			fb_copy(beta[0][1], t);
			fb_add(beta[0][2], beta[0][2], t);
			fb_add(beta[0][4], beta[0][4], t);

			fb12_mul_dxs(r, r, alpha);
			fb12_mul_dxs(r, r, beta);
		}

		fb_add_dig(x1[3], x1[0], 1);
		fb_add_dig(x1[2], x1[FB_BITS - 1], 1);
		fb_add(y1[2], y1[FB_BITS - 1], x1[0]);

		fb_add(t, x2[0], x1[3]);
		fb_add(t, t, x1[2]);
		fb_add_dig(t, t, 1);
		fb_mul(t, t, x2[1]);
		fb_add(alpha[0][0], y2[0], t);
		fb_mul(t, x1[2], x2[0]);
		fb_add(alpha[0][0], alpha[0][0], t);
		fb_add(alpha[0][0], alpha[0][0], y1[2]);

		fb_add(alpha[0][1], x2[1], x1[2]);
		fb_add(alpha[0][2], x1[3], x1[2]);
		fb_set_dig(alpha[0][3], 1);
		fb_add(alpha[0][4], x2[1], x2[0]);

		fb_copy(t, alpha[0][4]);
		fb_copy(alpha[0][4], alpha[0][2]);
		fb_copy(alpha[0][2], alpha[0][1]);
		fb_set_dig(alpha[0][1], 1);
		fb_set_dig(alpha[0][3], 1);
		fb_set_dig(alpha[0][5], 1);
		fb_add(alpha[0][0], alpha[0][0], t);
		fb_add(alpha[0][1], alpha[0][1], t);
		fb_add(alpha[0][2], alpha[0][2], t);
		fb_add(alpha[0][4], alpha[0][4], t);

		/* Compute f^4 * l(x,y). */
		fb12_sqr(r, r);
		fb12_sqr(r, r);
		fb12_mul(r, r, alpha);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb12_free(alpha);
		fb12_free(beta);
		fb_free(t);
		for (i = 0; i < FB_BITS; i++) {
			fb_free(x1[i]);
			fb_free(x2[i]);
			fb_free(y1[i]);
			fb_free(y2[i]);
		}
	}
}

/**
 * Computes the etat pairing with points on a genus 2 curve.
 *
 * @param[out] r			- the result.
 * @param[in] xp			- the x-coordinate of the first point.
 * @param[in] yp			- the y-coordinate of the first point.
 * @param[in] xp1			- the x-coordinate of the second point.
 * @param[in] yp1			- the y-coordinate of the second point.
 * @param[in] xp2			- the x-coordinate of the third point.
 * @param[in] yp2			- the y-coordinate of the third point.
 */
void pb_map_etat2_dxa(fb12_t r, fb_t xp, fb_t yp, fb_t xq1, fb_t yq1, fb_t xq2,
		fb_t yq2) {
	fb12_t alpha, beta;
	fb_t x1[FB_BITS], x2[FB_BITS], x3[FB_BITS], y1[FB_BITS], y2[FB_BITS],
			y3[FB_BITS], t;
	int i, k1, k2, k3, k4, k5, k6;
	dig_t gamma;

	/* Set gamma = 1 if m = 1 (mod 4); otherwise, gamma = 0. */
	if (FB_BITS % 4 == 1) {
		gamma = 1;
	} else {
		gamma = 0;
	}

	fb12_null(alpha);
	fb12_null(beta);
	for (i = 0; i < FB_BITS; i++) {
		fb_null(x1[i]);
		fb_null(x2[i]);
		fb_null(x3[i]);
		fb_null(y1[i]);
		fb_null(y2[i]);
		fb_null(y3[i]);
	}
	fb_null(t);

	TRY {
		fb12_new(alpha);
		fb12_new(beta);
		for (i = 0; i < FB_BITS; i++) {
			fb_new(x1[i]);
			fb_new(x2[i]);
			fb_new(x3[i]);
			fb_new(y1[i]);
			fb_new(y2[i]);
			fb_new(y3[i]);
		}
		fb_new(t);

		/* f = 1. */
		fb12_zero(r);
		fb_set_dig(r[0][0], 1);
		/* x1[i] = xp^2^i, y1[i] = yp^2^i. */
		fb_copy(x1[0], xp);
		fb_copy(y1[0], yp);
		/* x2[i] = xq1^2^i, y2[i] = yq1^2^i. */
		fb_copy(x2[0], xq1);
		fb_copy(y2[0], yq1);
		/* x2[i] = xq2^2^i, y2[i] = yq2^2^i. */
		fb_copy(x3[0], xq2);
		fb_copy(y3[0], yq2);
		for (i = 1; i < FB_BITS; i++) {
			fb_sqr(x1[i], x1[i - 1]);
			fb_sqr(x2[i], x2[i - 1]);
			fb_sqr(x3[i], x3[i - 1]);
			fb_sqr(y1[i], y1[i - 1]);
			fb_sqr(y2[i], y2[i - 1]);
			fb_sqr(y3[i], y3[i - 1]);
		}

		fb12_zero(alpha);
		fb_set_dig(alpha[1][0], 1);
		fb12_zero(beta);
		fb_set_dig(beta[1][0], 1);
		for (i = 0; i < (FB_BITS - 1) / 2; i++) {
			/* k1 = (3 * m - 9 - 6 * i)/2 mod m. */
			k1 = (3 * FB_BITS - 9 - 6 * i) / 2;
			k1 %= FB_BITS;
			/* k2 = (k1 + 1) mod m. */
			k2 = (k1 + 1) % FB_BITS;
			/* k3 = (k2 + 1) mod m. */
			k3 = (k2 + 1) % FB_BITS;
			/* k4 = (3 * m - 3 + 6 * i)/2 mod m. */
			k4 = (3 * FB_BITS - 3 + 6 * i) / 2;
			k4 %= FB_BITS;
			/* k5 = (k4 + 1) mod m. */
			k5 = (k4 + 1) % FB_BITS;
			/* k6 = (k5 + 1) mod m. */
			k6 = (k5 + 1) % FB_BITS;

			/* Compute alpha = a + b * w + c * w^2 + d * w^4 + s_0. */
			/* Compute d = x1[k4] + x1[k5]. */
			fb_add(alpha[0][4], x1[k4], x1[k5]);
			/* Compute a = y2[k2] + (x1[k4] + 1 + x2[k3]) * x2[k2] + d * x2[k3] + y1[k4] + gamma. */
			fb_add(t, x1[k4], x2[k3]);
			fb_add_dig(t, t, 1);
			fb_mul(t, t, x2[k2]);
			fb_add(alpha[0][0], y2[k2], t);
			fb_mul(t, alpha[0][4], x2[k3]);
			fb_add(alpha[0][0], alpha[0][0], t);
			fb_add(alpha[0][0], alpha[0][0], y1[k4]);
			fb_add_dig(alpha[0][0], alpha[0][0], gamma);
			/* Compute b = x2[k3] + x2[k2]. */
			fb_add(alpha[0][1], x2[k3], x2[k2]);
			/* Compute c = x2[k3] + x1[k4] + 1. */
			fb_add(alpha[0][2], x2[k3], x1[k4]);
			fb_add_dig(alpha[0][2], alpha[0][2], 1);

			fb_copy(t, alpha[0][4]);
			fb_copy(alpha[0][4], alpha[0][2]);
			fb_copy(alpha[0][2], alpha[0][1]);
			fb_add(alpha[0][0], alpha[0][0], t);
			fb_copy(alpha[0][1], t);
			fb_add(alpha[0][2], alpha[0][2], t);
			fb_add(alpha[0][4], alpha[0][4], t);

			/* Compute beta = e + f2 * w + g * w^2 + h * w^4 + s_0. */
			/* Compute f2 = x1[k5] + x1[k6]. */
			fb_add(beta[0][1], x1[k5], x1[k6]);
			/* Compute e = y2[k1] + f2 * x2[k1] + y1[k5] + x1[k6] * (x1[k5] + x2[k2]) + x1[k5] + gamma. */
			fb_mul(t, beta[0][1], x2[k1]);
			fb_add(beta[0][0], y2[k1], t);
			fb_add(beta[0][0], beta[0][0], y1[k5]);
			fb_add(t, x1[k5], x2[k2]);
			fb_mul(t, t, x1[k6]);
			fb_add(beta[0][0], beta[0][0], t);
			fb_add(beta[0][0], beta[0][0], x1[k5]);
			fb_add_dig(beta[0][0], beta[0][0], gamma);
			/* Compute g = x2[k1] + x1[k6] + 1. */
			fb_add(beta[0][2], x2[k1], x1[k6]);
			fb_add_dig(beta[0][2], beta[0][2], 1);
			/* Compute h = x2[k2] + x2[k1]. */
			fb_add(beta[0][4], x2[k2], x2[k1]);

			fb_copy(t, beta[0][4]);
			fb_copy(beta[0][4], beta[0][2]);
			fb_copy(beta[0][2], beta[0][1]);
			fb_add(beta[0][0], beta[0][0], t);
			fb_copy(beta[0][1], t);
			fb_add(beta[0][2], beta[0][2], t);
			fb_add(beta[0][4], beta[0][4], t);

			fb12_mul_dxs(r, r, alpha);
			fb12_mul_dxs(r, r, beta);

			/* Compute alpha = a + b * w + c * w^2 + d * w^4 + s_0. */
			/* Compute d = x1[k4] + x1[k5]. */
			fb_add(alpha[0][4], x1[k4], x1[k5]);
			/* Compute a = y3[k2] + (x1[k4] + 1 + x3[k3]) * x3[k2] + d * x3[k3] + y1[k4] + gamma. */
			fb_add(t, x1[k4], x3[k3]);
			fb_add_dig(t, t, 1);
			fb_mul(t, t, x3[k2]);
			fb_add(alpha[0][0], y3[k2], t);
			fb_mul(t, alpha[0][4], x3[k3]);
			fb_add(alpha[0][0], alpha[0][0], t);
			fb_add(alpha[0][0], alpha[0][0], y1[k4]);
			fb_add_dig(alpha[0][0], alpha[0][0], gamma);
			/* Compute b = x3[k3] + x3[k2]. */
			fb_add(alpha[0][1], x3[k3], x3[k2]);
			/* Compute c = x3[k3] + x1[k4] + 1. */
			fb_add(alpha[0][2], x3[k3], x1[k4]);
			fb_add_dig(alpha[0][2], alpha[0][2], 1);

			fb_copy(t, alpha[0][4]);
			fb_copy(alpha[0][4], alpha[0][2]);
			fb_copy(alpha[0][2], alpha[0][1]);
			fb_add(alpha[0][0], alpha[0][0], t);
			fb_copy(alpha[0][1], t);
			fb_add(alpha[0][2], alpha[0][2], t);
			fb_add(alpha[0][4], alpha[0][4], t);

			/* Compute beta = e + f2 * w + g * w^2 + h * w^4 + s_0. */
			/* Compute f2 = x1[k5] + x1[k6]. */
			fb_add(beta[0][1], x1[k5], x1[k6]);
			/* Compute e = y3[k1] + f2 * x3[k1] + y1[k5] + x1[k6] * (x1[k5] + x3[k2]) + x1[k5] + gamma. */
			fb_mul(t, beta[0][1], x3[k1]);
			fb_add(beta[0][0], y3[k1], t);
			fb_add(beta[0][0], beta[0][0], y1[k5]);
			fb_add(t, x1[k5], x3[k2]);
			fb_mul(t, t, x1[k6]);
			fb_add(beta[0][0], beta[0][0], t);
			fb_add(beta[0][0], beta[0][0], x1[k5]);
			fb_add_dig(beta[0][0], beta[0][0], gamma);
			/* Compute g = x3[k1] + x1[k6] + 1. */
			fb_add(beta[0][2], x3[k1], x1[k6]);
			fb_add_dig(beta[0][2], beta[0][2], 1);
			/* Compute h = x3[k2] + x3[k1]. */
			fb_add(beta[0][4], x3[k2], x3[k1]);

			fb_copy(t, beta[0][4]);
			fb_copy(beta[0][4], beta[0][2]);
			fb_copy(beta[0][2], beta[0][1]);
			fb_add(beta[0][0], beta[0][0], t);
			fb_copy(beta[0][1], t);
			fb_add(beta[0][2], beta[0][2], t);
			fb_add(beta[0][4], beta[0][4], t);

			fb12_mul_dxs(r, r, alpha);
			fb12_mul_dxs(r, r, beta);
		}

		fb_add_dig(x1[3], x1[0], 1);
		fb_add_dig(x1[2], x1[FB_BITS - 1], 1);
		fb_add(y1[2], y1[FB_BITS - 1], x1[0]);

		fb_add(t, x2[0], x1[3]);
		fb_add(t, t, x1[2]);
		fb_add_dig(t, t, 1);
		fb_mul(t, t, x2[1]);
		fb_add(alpha[0][0], y2[0], t);
		fb_mul(t, x1[2], x2[0]);
		fb_add(alpha[0][0], alpha[0][0], t);
		fb_add(alpha[0][0], alpha[0][0], y1[2]);

		fb_add(alpha[0][1], x2[1], x1[2]);
		fb_add(alpha[0][2], x1[3], x1[2]);
		fb_set_dig(alpha[0][3], 1);
		fb_add(alpha[0][4], x2[1], x2[0]);

		fb_copy(t, alpha[0][4]);
		fb_copy(alpha[0][4], alpha[0][2]);
		fb_copy(alpha[0][2], alpha[0][1]);
		fb_set_dig(alpha[0][1], 1);
		fb_set_dig(alpha[0][3], 1);
		fb_set_dig(alpha[0][5], 1);
		fb_add(alpha[0][0], alpha[0][0], t);
		fb_add(alpha[0][1], alpha[0][1], t);
		fb_add(alpha[0][2], alpha[0][2], t);
		fb_add(alpha[0][4], alpha[0][4], t);

		fb_add(t, x3[0], x1[3]);
		fb_add(t, t, x1[2]);
		fb_add_dig(t, t, 1);
		fb_mul(t, t, x3[1]);
		fb_add(beta[0][0], y3[0], t);
		fb_mul(t, x1[2], x3[0]);
		fb_add(beta[0][0], beta[0][0], t);
		fb_add(beta[0][0], beta[0][0], y1[2]);

		fb_add(beta[0][1], x3[1], x1[2]);
		fb_add(beta[0][2], x1[3], x1[2]);
		fb_set_dig(beta[0][3], 1);
		fb_add(beta[0][4], x3[1], x3[0]);

		fb_copy(t, beta[0][4]);
		fb_copy(beta[0][4], beta[0][2]);
		fb_copy(beta[0][2], beta[0][1]);
		fb_set_dig(beta[0][1], 1);
		fb_set_dig(beta[0][3], 1);
		fb_set_dig(beta[0][5], 1);
		fb_add(beta[0][0], beta[0][0], t);
		fb_add(beta[0][1], beta[0][1], t);
		fb_add(beta[0][2], beta[0][2], t);
		fb_add(beta[0][4], beta[0][4], t);

		/* Compute f^4 * l(x,y). */
		fb12_sqr(r, r);
		fb12_sqr(r, r);
		fb12_mul(r, r, alpha);
		fb12_mul(r, r, beta);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb12_free(alpha);
		fb12_free(beta);
		fb_free(t);
		for (i = 0; i < FB_BITS; i++) {
			fb_free(x1[i]);
			fb_free(x2[i]);
			fb_free(x3[i]);
			fb_free(y1[i]);
			fb_free(y2[i]);
			fb_free(x3[i]);
		}
	}
}

/**
 * Computes the etat pairing with points on a genus 2 curve.
 *
 * @param[out] r			- the result.
 * @param[in] xp			- the x-coordinate of the first point.
 * @param[in] yp			- the y-coordinate of the first point.
 * @param[in] xp1			- the x-coordinate of the second point.
 * @param[in] yp1			- the y-coordinate of the second point.
 * @param[in] xp2			- the x-coordinate of the third point.
 * @param[in] yp2			- the y-coordinate of the third point.
 */
void pb_map_etat2_axd(fb12_t r, fb_t xp1, fb_t yp1, fb_t xp2, fb_t yp2, fb_t xq,
		fb_t yq) {
	fb12_t alpha, beta;
	fb_t x1[FB_BITS], x2[FB_BITS], x3[FB_BITS], y1[FB_BITS], y2[FB_BITS],
			y3[FB_BITS], t;
	int i, k1, k2, k3, k4, k5, k6;
	dig_t gamma;

	/* Set gamma = 1 if m = 1 (mod 4); otherwise, gamma = 0. */
	if (FB_BITS % 4 == 1) {
		gamma = 1;
	} else {
		gamma = 0;
	}

	fb12_null(alpha);
	fb12_null(beta);
	for (i = 0; i < FB_BITS; i++) {
		fb_null(x1[i]);
		fb_null(x2[i]);
		fb_null(x3[i]);
		fb_null(y1[i]);
		fb_null(y2[i]);
		fb_null(y3[i]);
	}
	fb_null(t);

	TRY {
		fb12_new(alpha);
		fb12_new(beta);
		for (i = 0; i < FB_BITS; i++) {
			fb_new(x1[i]);
			fb_new(x2[i]);
			fb_new(x3[i]);
			fb_new(y1[i]);
			fb_new(y2[i]);
			fb_new(y3[i]);
		}
		fb_new(t);

		/* f = 1. */
		fb12_zero(r);
		fb_set_dig(r[0][0], 1);
		/* x1[i] = xp^2^i, y1[i] = yp^2^i. */
		fb_copy(x1[0], xp1);
		fb_copy(y1[0], yp1);
		/* x2[i] = xq1^2^i, y2[i] = yq1^2^i. */
		fb_copy(x2[0], xp2);
		fb_copy(y2[0], yp2);
		/* x2[i] = xq2^2^i, y2[i] = yq2^2^i. */
		fb_copy(x3[0], xq);
		fb_copy(y3[0], yq);
		for (i = 1; i < FB_BITS; i++) {
			fb_sqr(x1[i], x1[i - 1]);
			fb_sqr(x2[i], x2[i - 1]);
			fb_sqr(x3[i], x3[i - 1]);
			fb_sqr(y1[i], y1[i - 1]);
			fb_sqr(y2[i], y2[i - 1]);
			fb_sqr(y3[i], y3[i - 1]);
		}

		fb12_zero(alpha);
		fb_set_dig(alpha[1][0], 1);
		fb12_zero(beta);
		fb_set_dig(beta[1][0], 1);
		for (i = 0; i < (FB_BITS - 1) / 2; i++) {
			/* k1 = (3 * m - 9 - 6 * i)/2 mod m. */
			k1 = (3 * FB_BITS - 9 - 6 * i) / 2;
			k1 %= FB_BITS;
			/* k2 = (k1 + 1) mod m. */
			k2 = (k1 + 1) % FB_BITS;
			/* k3 = (k2 + 1) mod m. */
			k3 = (k2 + 1) % FB_BITS;
			/* k4 = (3 * m - 3 + 6 * i)/2 mod m. */
			k4 = (3 * FB_BITS - 3 + 6 * i) / 2;
			k4 %= FB_BITS;
			/* k5 = (k4 + 1) mod m. */
			k5 = (k4 + 1) % FB_BITS;
			/* k6 = (k5 + 1) mod m. */
			k6 = (k5 + 1) % FB_BITS;

			/* Compute alpha = a + b * w + c * w^2 + d * w^4 + s_0. */
			/* Compute d = x1[k4] + x1[k5]. */
			fb_add(alpha[0][4], x1[k4], x1[k5]);
			/* Compute a = y3[k2] + (x1[k4] + 1 + x3[k3]) * x3[k2] + d * x3[k3] + y1[k4] + gamma. */
			fb_add(t, x1[k4], x3[k3]);
			fb_add_dig(t, t, 1);
			fb_mul(t, t, x3[k2]);
			fb_add(alpha[0][0], y3[k2], t);
			fb_mul(t, alpha[0][4], x3[k3]);
			fb_add(alpha[0][0], alpha[0][0], t);
			fb_add(alpha[0][0], alpha[0][0], y1[k4]);
			fb_add_dig(alpha[0][0], alpha[0][0], gamma);
			/* Compute b = x3[k3] + x3[k2]. */
			fb_add(alpha[0][1], x3[k3], x3[k2]);
			/* Compute c = x3[k3] + x1[k4] + 1. */
			fb_add(alpha[0][2], x3[k3], x1[k4]);
			fb_add_dig(alpha[0][2], alpha[0][2], 1);

			fb_copy(t, alpha[0][4]);
			fb_copy(alpha[0][4], alpha[0][2]);
			fb_copy(alpha[0][2], alpha[0][1]);
			fb_add(alpha[0][0], alpha[0][0], t);
			fb_copy(alpha[0][1], t);
			fb_add(alpha[0][2], alpha[0][2], t);
			fb_add(alpha[0][4], alpha[0][4], t);

			/* Compute beta = e + f2 * w + g * w^2 + h * w^4 + s_0. */
			/* Compute f2 = x1[k5] + x1[k6]. */
			fb_add(beta[0][1], x1[k5], x1[k6]);
			/* Compute e = y3[k1] + f2 * x3[k1] + y1[k5] + x1[k6] * (x1[k5] + x3[k2]) + x1[k5] + gamma. */
			fb_mul(t, beta[0][1], x3[k1]);
			fb_add(beta[0][0], y3[k1], t);
			fb_add(beta[0][0], beta[0][0], y1[k5]);
			fb_add(t, x1[k5], x3[k2]);
			fb_mul(t, t, x1[k6]);
			fb_add(beta[0][0], beta[0][0], t);
			fb_add(beta[0][0], beta[0][0], x1[k5]);
			fb_add_dig(beta[0][0], beta[0][0], gamma);
			/* Compute g = x3[k1] + x1[k6] + 1. */
			fb_add(beta[0][2], x3[k1], x1[k6]);
			fb_add_dig(beta[0][2], beta[0][2], 1);
			/* Compute h = x3[k2] + x3[k1]. */
			fb_add(beta[0][4], x3[k2], x3[k1]);

			fb_copy(t, beta[0][4]);
			fb_copy(beta[0][4], beta[0][2]);
			fb_copy(beta[0][2], beta[0][1]);
			fb_add(beta[0][0], beta[0][0], t);
			fb_copy(beta[0][1], t);
			fb_add(beta[0][2], beta[0][2], t);
			fb_add(beta[0][4], beta[0][4], t);

			fb12_mul_dxs(r, r, alpha);
			fb12_mul_dxs(r, r, beta);

			/* Compute alpha = a + b * w + c * w^2 + d * w^4 + s_0. */
			/* Compute d = x2[k4] + x2[k5]. */
			fb_add(alpha[0][4], x2[k4], x2[k5]);
			/* Compute a = y3[k2] + (x2[k4] + 1 + x3[k3]) * x3[k2] + d * x3[k3] + y2[k4] + gamma. */
			fb_add(t, x2[k4], x3[k3]);
			fb_add_dig(t, t, 1);
			fb_mul(t, t, x3[k2]);
			fb_add(alpha[0][0], y3[k2], t);
			fb_mul(t, alpha[0][4], x3[k3]);
			fb_add(alpha[0][0], alpha[0][0], t);
			fb_add(alpha[0][0], alpha[0][0], y2[k4]);
			fb_add_dig(alpha[0][0], alpha[0][0], gamma);
			/* Compute b = x3[k3] + x3[k2]. */
			fb_add(alpha[0][1], x3[k3], x3[k2]);
			/* Compute c = x3[k3] + x2[k4] + 1. */
			fb_add(alpha[0][2], x3[k3], x2[k4]);
			fb_add_dig(alpha[0][2], alpha[0][2], 1);

			fb_copy(t, alpha[0][4]);
			fb_copy(alpha[0][4], alpha[0][2]);
			fb_copy(alpha[0][2], alpha[0][1]);
			fb_add(alpha[0][0], alpha[0][0], t);
			fb_copy(alpha[0][1], t);
			fb_add(alpha[0][2], alpha[0][2], t);
			fb_add(alpha[0][4], alpha[0][4], t);

			/* Compute beta = e + f2 * w + g * w^2 + h * w^4 + s_0. */
			/* Compute f2 = x2[k5] + x2[k6]. */
			fb_add(beta[0][1], x2[k5], x2[k6]);
			/* Compute e = y3[k1] + f2 * x3[k1] + y2[k5] + x2[k6] * (x2[k5] + x3[k2]) + x2[k5] + gamma. */
			fb_mul(t, beta[0][1], x3[k1]);
			fb_add(beta[0][0], y3[k1], t);
			fb_add(beta[0][0], beta[0][0], y2[k5]);
			fb_add(t, x2[k5], x3[k2]);
			fb_mul(t, t, x2[k6]);
			fb_add(beta[0][0], beta[0][0], t);
			fb_add(beta[0][0], beta[0][0], x2[k5]);
			fb_add_dig(beta[0][0], beta[0][0], gamma);
			/* Compute g = x3[k1] + x2[k6] + 1. */
			fb_add(beta[0][2], x3[k1], x2[k6]);
			fb_add_dig(beta[0][2], beta[0][2], 1);
			/* Compute h = x3[k2] + x3[k1]. */
			fb_add(beta[0][4], x3[k2], x3[k1]);

			fb_copy(t, beta[0][4]);
			fb_copy(beta[0][4], beta[0][2]);
			fb_copy(beta[0][2], beta[0][1]);
			fb_add(beta[0][0], beta[0][0], t);
			fb_copy(beta[0][1], t);
			fb_add(beta[0][2], beta[0][2], t);
			fb_add(beta[0][4], beta[0][4], t);

			fb12_mul_dxs(r, r, alpha);
			fb12_mul_dxs(r, r, beta);
		}

		fb_add_dig(x1[3], x1[0], 1);
		fb_add_dig(x1[2], x1[FB_BITS - 1], 1);
		fb_add(y1[2], y1[FB_BITS - 1], x1[0]);

		fb_add(t, x3[0], x1[3]);
		fb_add(t, t, x1[2]);
		fb_add_dig(t, t, 1);
		fb_mul(t, t, x3[1]);
		fb_add(alpha[0][0], y3[0], t);
		fb_mul(t, x1[2], x3[0]);
		fb_add(alpha[0][0], alpha[0][0], t);
		fb_add(alpha[0][0], alpha[0][0], y1[2]);

		fb_add(alpha[0][1], x3[1], x1[2]);
		fb_add(alpha[0][2], x1[3], x1[2]);
		fb_set_dig(alpha[0][3], 1);
		fb_add(alpha[0][4], x3[1], x3[0]);

		fb_add_dig(x2[3], x2[0], 1);
		fb_add_dig(x2[2], x2[FB_BITS - 1], 1);
		fb_add(y2[2], y2[FB_BITS - 1], x2[0]);

		fb_add(t, x3[0], x2[3]);
		fb_add(t, t, x2[2]);
		fb_add_dig(t, t, 1);
		fb_mul(t, t, x3[1]);
		fb_add(beta[0][0], y3[0], t);
		fb_mul(t, x2[2], x3[0]);
		fb_add(beta[0][0], beta[0][0], t);
		fb_add(beta[0][0], beta[0][0], y2[2]);

		fb_copy(t, alpha[0][4]);
		fb_copy(alpha[0][4], alpha[0][2]);
		fb_copy(alpha[0][2], alpha[0][1]);
		fb_set_dig(alpha[0][1], 1);
		fb_set_dig(alpha[0][3], 1);
		fb_set_dig(alpha[0][5], 1);
		fb_add(alpha[0][0], alpha[0][0], t);
		fb_add(alpha[0][1], alpha[0][1], t);
		fb_add(alpha[0][2], alpha[0][2], t);
		fb_add(alpha[0][4], alpha[0][4], t);

		fb_add(beta[0][1], x3[1], x2[2]);
		fb_add(beta[0][2], x2[3], x2[2]);
		fb_set_dig(beta[0][3], 1);
		fb_add(beta[0][4], x3[1], x3[0]);

		fb_copy(t, beta[0][4]);
		fb_copy(beta[0][4], beta[0][2]);
		fb_copy(beta[0][2], beta[0][1]);
		fb_set_dig(beta[0][1], 1);
		fb_set_dig(beta[0][3], 1);
		fb_set_dig(beta[0][5], 1);
		fb_add(beta[0][0], beta[0][0], t);
		fb_add(beta[0][1], beta[0][1], t);
		fb_add(beta[0][2], beta[0][2], t);
		fb_add(beta[0][4], beta[0][4], t);

		/* Compute f^4 * l(x,y). */
		fb12_sqr(r, r);
		fb12_sqr(r, r);
		fb12_mul(r, r, alpha);
		fb12_mul(r, r, beta);

	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb12_free(alpha);
		fb12_free(beta);
		fb_free(t);
		for (i = 0; i < FB_BITS; i++) {
			fb_free(x1[i]);
			fb_free(x2[i]);
			fb_free(x3[i]);
			fb_free(y1[i]);
			fb_free(y2[i]);
			fb_free(x3[i]);
		}
	}
}

static void fb6_conv(fb6_t c, fb6_t a) {
	fb6_t t;

	fb6_null(t);

	TRY {
		fb6_new(t);
		fb_copy(t[0], a[0]);
		fb_copy(t[1], a[4]);
		fb_copy(t[2], a[1]);
		fb_copy(t[3], a[3]);
		fb_copy(t[4], a[2]);
		fb_copy(t[5], a[5]);
		fb_add(t[0], t[0], a[4]);
		fb_add(t[1], t[1], a[3]);
		fb_add(t[1], t[1], a[5]);
		fb_add(t[2], t[2], a[4]);
		fb_add(t[2], t[2], a[5]);
		fb_add(t[4], t[4], a[4]);
		fb_add(t[4], t[4], a[5]);
		fb_add(t[5], t[5], a[3]);
		fb6_copy(c, t);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb6_free(t);
	}
}

static void pb_map_g4(fb12_t l1, fb12_t l0, fb_t mubar[FB_BITS][8],
		fb_t nubar[FB_BITS][8], hb_t p, hb_t q, int e1, int e2,
		fb_t table[FB_BITS][24]) {
	fb_t t;
	fb12_t g4;

	fb_null(t);
	fb12_null(g4);

	TRY {
		/* l0 = G_0,4. */
		fb6_zero(l0[1]);
		fb_mul(l0[0][0], table[e2][DELTA1], table[e1 + 1][V0]);
		fb_add(l0[0][0], l0[0][0], table[e1 + 2][V0]);
		fb_mul(l0[0][2], table[e2][U1], table[e1 + 1][V0]);
		fb_mul(l0[0][4], table[e2][DELTA0], table[e1 + 1][V0]);
		/* Use precomputed powers of b00-b05. */
		fb_add(l0[0][0], l0[0][0], table[e2][B00]);
		fb_copy(l0[0][1], table[e2][B01]);
		fb_add(l0[0][2], l0[0][2], table[e2][B02]);
		fb_copy(l0[0][3], table[e2][B03]);
		fb_add(l0[0][4], l0[0][4], table[e2][B04]);
		fb_copy(l0[0][5], table[e2][B05]);
		/* b06 = delta1 + 1. */
		fb_add_dig(l0[1][0], table[e2][DELTA1], 1);
		/* b07 = p.u1. */
		fb_copy(l0[1][2], table[e2][U1]);
		/* b08 = delta0. */
		fb_copy(l0[1][4], table[e2][DELTA0]);

		fb_mul(g4[0][0], table[e2][U1], table[e1 + 1][V0]);
		fb_mul(t, table[e2][DELTA1], table[e1 + 1][V1]);
		fb_add(g4[0][0], g4[0][0], t);
		fb_add(g4[0][0], g4[0][0], table[e2][B10]);
		fb_copy(g4[0][1], table[e2][DELTA1]);
		fb_mul(g4[0][2], table[e2][U1], table[e1 + 1][V1]);
		/* b13 = p.u1. */
		fb_copy(g4[0][3], table[e2][U1]);
		fb_add_dig(g4[0][4], table[e1 + 1][V1], 1);
		fb_mul(g4[0][4], g4[0][4], table[e2][DELTA0]);
		fb_add(g4[0][4], g4[0][4], table[e2][DELTA2]);
		/* b14 = delta0. */
		fb_copy(g4[0][5], table[e2][DELTA0]);
		/* b16 = p.u1. */
		fb_copy(g4[1][0], table[e2][U1]);

		/* R_4,0 = nubar[e1] dotvec h_i. */
		fb_mul(t, g4[0][0], nubar[e1][0]);
		fb_add(l0[0][0], l0[0][0], t);
		fb_mul(t, g4[0][1], nubar[e1][0]);
		fb_add(l0[0][1], l0[0][1], t);
		fb_mul(t, g4[0][2], nubar[e1][0]);
		fb_add(l0[0][2], l0[0][2], t);
		fb_mul(t, table[e2][U1], nubar[e1][0]);
		fb_add(l0[0][3], l0[0][3], t);
		fb_add(l0[1][0], l0[1][0], t);
		fb_mul(t, g4[0][4], nubar[e1][0]);
		fb_add(l0[0][4], l0[0][4], t);
		fb_mul(t, g4[0][5], nubar[e1][0]);
		fb_add(l0[0][5], l0[0][5], t);

		fb6_zero(l1[1]);
		/* R_4,1 = mubar[e1] dotvec h_i. */
		fb_mul(l1[0][0], g4[0][0], mubar[e1][0]);
		fb_mul(l1[0][1], g4[0][1], mubar[e1][0]);
		fb_mul(l1[0][2], g4[0][2], mubar[e1][0]);
		fb_mul(l1[0][3], g4[0][3], mubar[e1][0]);
		fb_mul(l1[0][4], g4[0][4], mubar[e1][0]);
		fb_mul(l1[0][5], g4[0][5], mubar[e1][0]);
		fb_mul(l1[1][0], g4[1][0], mubar[e1][0]);

		fb_mul(g4[0][0], table[e2][DELTA0], table[e1 + 1][V0]);
		fb_add(g4[0][0], g4[0][0], table[e1 + 2][V1]);
		fb_mul(t, table[e2][U1], table[e1 + 1][V1]);
		fb_add(g4[0][0], g4[0][0], t);
		fb_add(g4[0][0], g4[0][0], table[e2][B20]);
		fb_add(g4[0][1], table[e2][DELTA1], table[e2][U1]);
		fb_add(g4[0][2], table[e2][DELTA1], table[e2][DELTA2]);
		fb_add_dig(g4[0][2], g4[0][2], 1);
		fb_copy(g4[0][3], table[e2 + 1][U1]);
		fb_copy(g4[0][4], table[e2][U1]);
		fb_copy(g4[1][0], table[e2][DELTA0]);

		fb_mul(t, g4[0][0], mubar[e1][2]);
		fb_add(l1[0][0], l1[0][0], t);
		fb_mul(t, g4[0][1], mubar[e1][2]);
		fb_add(l1[0][1], l1[0][1], t);
		fb_mul(t, g4[0][2], mubar[e1][2]);
		fb_add(l1[0][2], l1[0][2], t);
		fb_mul(t, g4[0][3], mubar[e1][2]);
		fb_add(l1[0][3], l1[0][3], t);
		fb_mul(t, g4[0][4], mubar[e1][2]);
		fb_add(l1[0][4], l1[0][4], t);
		fb_mul(t, g4[1][0], mubar[e1][2]);
		fb_add(l1[1][0], l1[1][0], t);

		fb_mul(t, g4[0][0], nubar[e1][2]);
		fb_add(l0[0][0], l0[0][0], t);
		fb_mul(t, g4[0][1], nubar[e1][2]);
		fb_add(l0[0][1], l0[0][1], t);
		fb_mul(t, g4[0][2], nubar[e1][2]);
		fb_add(l0[0][2], l0[0][2], t);
		fb_mul(t, g4[0][3], nubar[e1][2]);
		fb_add(l0[0][3], l0[0][3], t);
		fb_mul(t, g4[0][4], nubar[e1][2]);
		fb_add(l0[0][4], l0[0][4], t);
		fb_mul(t, g4[1][0], nubar[e1][2]);
		fb_add(l0[1][0], l0[1][0], t);

		fb_mul(g4[0][0], table[e2][DELTA0], table[e1 + 1][V1]);
		fb_add(g4[0][0], g4[0][0], table[e2][DELTA0]);
		fb_add(g4[0][0], g4[0][0], table[e2][DELTA1]);
		fb_add(g4[0][0], g4[0][0], table[e2][DELTA2]);
		fb_copy(g4[0][1], table[e2 + 1][U1]);
		fb_copy(g4[0][4], table[e2][DELTA0]);

		fb_mul(t, g4[0][0], mubar[e1][4]);
		fb_add(l1[0][0], l1[0][0], t);
		fb_mul(t, g4[0][1], mubar[e1][4]);
		fb_add(l1[0][1], l1[0][1], t);
		fb_mul(t, g4[0][4], mubar[e1][4]);
		fb_add(l1[0][4], l1[0][4], t);

		fb_mul(t, g4[0][0], nubar[e1][4]);
		fb_add(l0[0][0], l0[0][0], t);
		fb_mul(t, g4[0][1], nubar[e1][4]);
		fb_add(l0[0][1], l0[0][1], t);
		fb_mul(t, g4[0][4], nubar[e1][4]);
		fb_add(l0[0][4], l0[0][4], t);

		fb_copy(g4[0][0], table[e2][B40]);
		fb_copy(g4[0][1], table[e2][DELTA0]);
		fb_add_dig(g4[0][2], table[e2][DELTA0], 1);
		fb_set_dig(g4[0][4], 1);

		fb_mul(t, g4[0][0], mubar[e1][5]);
		fb_add(l1[0][0], l1[0][0], t);
		fb_mul(t, g4[0][1], mubar[e1][5]);
		fb_add(l1[0][1], l1[0][1], t);
		fb_mul(t, g4[0][2], mubar[e1][5]);
		fb_add(l1[0][2], l1[0][2], t);
		fb_add(l1[0][4], l1[0][4], mubar[e1][5]);

		fb_mul(t, g4[0][0], nubar[e1][5]);
		fb_add(l0[0][0], l0[0][0], t);
		fb_mul(t, g4[0][1], nubar[e1][5]);
		fb_add(l0[0][1], l0[0][1], t);
		fb_mul(t, g4[0][2], nubar[e1][5]);
		fb_add(l0[0][2], l0[0][2], t);
		fb_mul(t, g4[0][4], nubar[e1][5]);
		fb_add(l0[0][4], l0[0][4], t);

		/* G_i,4[8] = delta0^2^e2. */
		fb_mul(t, table[e2][DELTA0], mubar[e1][6]);
		fb_add(l1[0][0], l1[0][0], t);
		/* G_i,4[9] =  1. */
		fb_add(l1[0][0], l1[0][0], mubar[e1][7]);

		/* G_i,4[8] = delta0^2^e2. */
		fb_mul(t, table[e2][DELTA0], nubar[e1][6]);
		fb_add(l0[0][0], l0[0][0], t);
		/* G_i,4[9] =  1. */
		fb_add(l0[0][0], l0[0][0], nubar[e1][7]);

		fb6_conv(l0[0], l0[0]);
		fb6_conv(l0[1], l0[1]);
		fb6_conv(l1[0], l1[0]);
		fb6_conv(l1[1], l1[1]);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(t);
		fb12_free(g4);
	}
}

static void pb_map_g8(fb12_t l1, fb12_t l0, fb_t mubar[FB_BITS][8],
		fb_t nubar[FB_BITS][8], hb_t p, hb_t q, int e1, int e2,
		fb_t table[FB_BITS][24]) {
	fb_t t;
	fb12_t g8;

	fb_null(t);
	fb12_null(g8);

	TRY {
		fb_new(t);
		fb12_new(g8);

		fb12_zero(l0);
		fb_mul(l0[0][0], table[e2 + 1][EPSIL2], table[e1][V0]);
		fb_add(l0[0][0], l0[0][0], table[e1 + 1][V0]);
		fb_add(l0[0][0], l0[0][0], table[e2 + 1][D00]);
		fb_mul(l0[0][1], table[e2 + 1][DELTA0], table[e1][V0]);
		fb_add(l0[0][1], l0[0][1], table[e2 + 1][D01]);
		fb_mul(l0[0][2], table[e2 + 2][U1], table[e1][V0]);
		fb_add(l0[0][2], l0[0][2], table[e2 + 1][D02]);
		fb_add(l0[0][3], l0[0][3], table[e2 + 1][DELTA2]);
		fb_add(l0[0][3], l0[0][3], table[e2 + 1][DELTA0]);
		fb_add_dig(l0[0][3], l0[0][3], 1);
		fb_add(l0[0][4], l0[0][4], table[e2 + 2][U0]);
		fb_add(l0[0][4], l0[0][4], table[e2 + 2][U1]);
		fb_add_dig(l0[0][4], l0[0][4], 1);
		fb_add_dig(l0[0][5], l0[0][5], 1);
		fb_add(l0[1][0], l0[1][0], table[e2 + 1][EPSIL2]);
		fb_add_dig(l0[1][0], l0[1][0], 1);
		fb_add(l0[1][1], l0[1][1], table[e2 + 1][DELTA0]);
		fb_add(l0[1][2], l0[1][2], table[e2 + 2][U1]);

		fb12_zero(l1);
		fb_mul(l1[0][0], table[e2 + 1][DELTA0], table[e1][V0]);
		fb_mul(t, table[e2 + 1][EPSIL2], table[e1][V1]);
		fb_add(l1[0][0], l1[0][0], t);
		fb_mul(t, table[e2 + 1][DELTA0], table[e1][V1]);
		fb_add(l1[0][1], l1[0][1], t);
		fb_mul(t, table[e2 + 2][U1], table[e1][V1]);
		fb_add(l1[0][2], l1[0][2], t);
		fb_add(l1[0][0], l1[0][0], table[e2 + 1][D10]);
		fb_add(l1[0][2], l1[0][2], table[e2 + 1][D12]);
		fb_add(l1[0][3], l1[0][3], table[e2 + 1][U1]);
		fb_add(l1[0][4], l1[0][4], table[e2 + 1][EPSIL2]);
		fb_add(l1[0][4], l1[0][4], table[e2 + 2][U1]);
		fb_add(l1[0][5], l1[0][5], table[e2 + 1][U1]);
		fb_add(l1[1][0], l1[1][0], table[e2 + 1][DELTA0]);

		fb_mul(g8[0][0], table[e2 + 2][U1], table[e1][V0]);
		fb_add(g8[0][0], g8[0][0], table[e1 + 1][V1]);
		fb_mul(t, table[e2 + 1][DELTA0], table[e1][V1]);
		fb_add(g8[0][0], g8[0][0], t);
		fb_add(g8[0][0], g8[0][0], table[e2 + 1][D20]);
		fb_add_dig(g8[0][1], table[e2 + 1][DELTA2], 1);
		fb_copy(g8[0][2], table[e2 + 1][DELTA0]);
		fb_copy(g8[0][3], table[e2 + 2][U1]);
		fb_add(g8[0][4], table[e2 + 1][EPSIL2], table[e2 + 1][DELTA0]);
		fb_add_dig(g8[0][4], g8[0][4], 1);
		fb_copy(g8[0][5], table[e2 + 1][U1]);
		fb_copy(g8[1][0], table[e2 + 2][U1]);

		fb_mul(t, g8[0][0], mubar[e1][0]);
		fb_add(l1[0][0], l1[0][0], t);
		fb_mul(t, g8[0][1], mubar[e1][0]);
		fb_add(l1[0][1], l1[0][1], t);
		fb_mul(t, g8[0][2], mubar[e1][0]);
		fb_add(l1[0][2], l1[0][2], t);
		fb_mul(t, g8[0][3], mubar[e1][0]);
		fb_add(l1[0][3], l1[0][3], t);
		fb_add(l1[1][0], l1[1][0], t);
		fb_mul(t, g8[0][4], mubar[e1][0]);
		fb_add(l1[0][4], l1[0][4], t);
		fb_mul(t, g8[0][5], mubar[e1][0]);
		fb_add(l1[0][5], l1[0][5], t);

		fb_mul(t, g8[0][0], nubar[e1][0]);
		fb_add(l0[0][0], l0[0][0], t);
		fb_mul(t, g8[0][1], nubar[e1][0]);
		fb_add(l0[0][1], l0[0][1], t);
		fb_mul(t, g8[0][2], nubar[e1][0]);
		fb_add(l0[0][2], l0[0][2], t);
		fb_mul(t, g8[0][3], nubar[e1][0]);
		fb_add(l0[0][3], l0[0][3], t);
		fb_add(l0[1][0], l0[1][0], t);
		fb_mul(t, g8[0][4], nubar[e1][0]);
		fb_add(l0[0][4], l0[0][4], t);
		fb_mul(t, g8[0][5], nubar[e1][0]);
		fb_add(l0[0][5], l0[0][5], t);

		fb_mul(g8[0][0], table[e2 + 2][U1], table[e1][V1]);
		fb_add(g8[0][0], g8[0][0], table[e2 + 1][DELTA2]);
		fb_copy(g8[0][2], table[e2 + 2][U1]);
		fb_copy(g8[0][4], table[e2 + 1][U1]);

		fb_mul(t, g8[0][0], mubar[e1][1]);
		fb_add(l1[0][0], l1[0][0], t);
		fb_mul(t, g8[0][2], mubar[e1][1]);
		fb_add(l1[0][2], l1[0][2], t);
		fb_mul(t, g8[0][4], mubar[e1][1]);
		fb_add(l1[0][4], l1[0][4], t);

		fb_mul(t, g8[0][0], nubar[e1][1]);
		fb_add(l0[0][0], l0[0][0], t);
		fb_mul(t, g8[0][2], nubar[e1][1]);
		fb_add(l0[0][2], l0[0][2], t);
		fb_mul(t, g8[0][4], nubar[e1][1]);
		fb_add(l0[0][4], l0[0][4], t);

		fb_add_dig(g8[0][0], table[e2 + 2][U0], 1);
		//fb_set_dig(g8[0][1], 1);
		fb_copy(g8[0][4], table[e2 + 2][U1]);

		fb_mul(t, g8[0][0], mubar[e1][2]);
		fb_add(l1[0][0], l1[0][0], t);
		fb_add(l1[0][1], l1[0][1], mubar[e1][2]);
		fb_mul(t, g8[0][4], mubar[e1][2]);
		fb_add(l1[0][4], l1[0][4], t);

		fb_mul(t, g8[0][0], nubar[e1][2]);
		fb_add(l0[0][0], l0[0][0], t);
		fb_add(l0[0][1], l0[0][1], nubar[e1][2]);
		fb_mul(t, g8[0][4], nubar[e1][2]);
		fb_add(l0[0][4], l0[0][4], t);

		fb6_conv(l0[0], l0[0]);
		fb6_conv(l0[1], l0[1]);
		fb6_conv(l1[0], l1[0]);
		fb6_conv(l1[1], l1[1]);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(t);
		fb12_free(g8);
	}
}

static void line(fb12_t l0, fb12_t l1, fb_t *mu, fb_t *nu, hb_t p, hb_t q,
		int gamma, fb_t table[FB_BITS][24], fb_t delta3, fb_t epsil0,
		fb_t epsil3) {
	fb_t t;
	fb12_t l;

	fb_null(t);
	fb12_null(l);

	TRY {
		fb_new(t);
		fb12_new(l);

		fb12_zero(l);
		if (gamma) {
			fb_add_dig(l[0][0], table[0][DELTA1], 1);
			fb_copy(l[0][1], p->u1);
			fb_copy(l[0][2], table[0][DELTA0]);
			fb_copy(l[1][0], table[0][DELTA0]);
			fb_copy(l[2][0], table[0][DELTA0]);
		}
		fb_mul(t, table[1][U1], p->v0);
		fb_add(l[0][0], l[0][0], t);
		fb_add(t, p->u0, p->v0);
		fb_mul(t, t, table[0][DELTA1]);
		fb_add(l[0][0], l[0][0], t);
		fb_add(l[0][0], l[0][0], table[1][U0]);
		fb_sqr(t, p->v1);
		fb_mul(t, t, p->u0);
		fb_add(l[0][0], l[0][0], t);
		fb_sqr(t, p->v0);
		fb_add(l[0][0], l[0][0], t);
		fb_add_dig(l[0][0], l[0][0], 1);
		fb_srt(l[0][0], l[0][0]);
		fb_add(t, table[FB_BITS - 1][DELTA1], p->u1);
		fb_mul(t, t, q->v0);
		fb_add(l[0][0], l[0][0], t);
		fb_add(l[0][0], l[0][0], table[1][V0]);

		fb_add(l[0][1], l[0][1], delta3);
		fb_add(l[0][1], l[0][1], table[0][DELTA2]);
		fb_add(l[0][1], l[0][1], table[0][DELTA1]);
		fb_add(l[0][1], l[0][1], table[1][U1]);
		fb_srt(t, l[0][1]);
		fb_mul(l[0][1], table[FB_BITS - 1][U1], q->v0);
		fb_add(l[0][1], l[0][1], t);

		fb_mul(t, p->v0, table[0][DELTA0]);
		fb_add(l[0][2], l[0][2], t);
		fb_add_dig(t, p->v1, 1);
		fb_mul(t, t, table[0][DELTA2]);
		fb_add(l[0][2], l[0][2], t);
		fb_add(l[0][2], l[0][2], p->u1);
		fb_add(l[0][2], l[0][2], p->u0);
		fb_srt(t, l[0][2]);
		fb_zero(l[0][2]);
		fb_mul(l[0][2], table[FB_BITS - 1][DELTA0], q->v0);
		fb_add(l[0][2], l[0][2], t);

		fb_add(l[0][3], table[FB_BITS - 1][DELTA2], table[FB_BITS - 1][DELTA1]);
		fb_add(l[0][3], l[0][3], table[FB_BITS - 1][U1]);
		fb_srt(l[0][4], epsil0);
		fb_add(l[0][4], l[0][4], table[FB_BITS - 1][DELTA2]);
		fb_add(l[0][4], l[0][4], table[FB_BITS - 1][U1]);
		fb_copy(l[0][5], table[FB_BITS - 1][DELTA0]);
		fb_add(l[1][0], table[FB_BITS][U1], table[FB_BITS - 1][DELTA1]);
		fb_add_dig(l[1][0], l[1][0], 1);
		fb_copy(l[1][1], table[FB_BITS - 1][U1]);
		//fb_copy(l[0][1][2], delta0[FB_BITS - 1]);

		fb12_zero(l0);
		fb_mul(l0[0][0], l[0][0], nu[0]);
		fb_mul(l0[0][1], l[0][1], nu[0]);
		fb_mul(l0[0][2], l[0][2], nu[0]);
		fb_mul(l0[0][3], l[0][3], nu[0]);
		fb_mul(l0[0][4], l[0][4], nu[0]);
		fb_mul(l0[0][5], l[0][5], nu[0]);
		fb_mul(l0[1][0], l[1][0], nu[0]);
		fb_mul(l0[1][1], l[1][1], nu[0]);
		fb_copy(l0[1][2], l0[0][5]);

		fb12_zero(l1);
		fb_mul(l1[0][0], l[0][0], mu[0]);
		fb_mul(l1[0][1], l[0][1], mu[0]);
		fb_mul(l1[0][2], l[0][2], mu[0]);
		fb_mul(l1[0][3], l[0][3], mu[0]);
		fb_mul(l1[0][4], l[0][4], mu[0]);
		fb_mul(l1[0][5], l[0][5], mu[0]);
		fb_mul(l1[1][0], l[1][0], mu[0]);
		fb_mul(l1[1][1], l[1][1], mu[0]);
		fb_copy(l1[1][2], l1[0][5]);

		fb_mul(l[0][0], table[FB_BITS - 1][U1], q->v0);
		fb_add(t, table[FB_BITS - 1][DELTA1], p->u1);
		fb_mul(t, t, q->v1);
		fb_add(l[0][0], l[0][0], t);
		fb_add(l[1][0], table[0][DELTA2], table[0][DELTA1]);
		fb_add(l[1][0], l[1][0], delta3);
		fb_add(l[1][0], l[1][0], p->u1);
		fb_srt(l[1][0], l[1][0]);
		fb_add(l[0][0], l[0][0], l[1][0]);
		fb_mul(l[0][1], table[FB_BITS - 1][U1], q->v1);
		fb_mul(l[0][2], table[FB_BITS - 1][DELTA0], q->v1);
		fb_add(l[0][2], l[0][2], table[FB_BITS - 1][DELTA2]);
		fb_copy(l[0][3], p->u1);
		fb_add(l[0][4], table[FB_BITS - 1][DELTA1], p->u1);
		//fb_add(l[1][0][5], l[1][0][5], p->u1);
		//fb_add(l[1][1][0], l[1][1][0], u1[FB_BITS - 1]);

		fb_mul(t, l[0][0], nu[1]);
		fb_add(l0[0][0], l0[0][0], t);
		fb_mul(t, l[0][1], nu[1]);
		fb_add(l0[0][1], l0[0][1], t);
		fb_mul(t, l[0][2], nu[1]);
		fb_add(l0[0][2], l0[0][2], t);
		fb_mul(t, l[0][3], nu[1]);
		fb_add(l0[0][3], l0[0][3], t);
		fb_add(l0[0][5], l0[0][5], t);
		fb_mul(t, l[0][4], nu[1]);
		fb_add(l0[0][4], l0[0][4], t);
		fb_mul(t, table[FB_BITS - 1][U1], nu[1]);
		fb_add(l0[1][0], l0[1][0], t);

		fb_mul(t, l[0][0], mu[1]);
		fb_add(l1[0][0], l1[0][0], t);
		fb_mul(t, l[0][1], mu[1]);
		fb_add(l1[0][1], l1[0][1], t);
		fb_mul(t, l[0][2], mu[1]);
		fb_add(l1[0][2], l1[0][2], t);
		fb_mul(t, l[0][3], mu[1]);
		fb_add(l1[0][3], l1[0][3], t);
		fb_add(l1[0][5], l1[0][5], t);
		fb_mul(t, l[0][4], mu[1]);
		fb_add(l1[0][4], l1[0][4], t);
		fb_mul(t, table[FB_BITS - 1][U1], mu[1]);
		fb_add(l1[1][0], l1[1][0], t);

		fb_mul(l[0][0], table[FB_BITS - 1][DELTA0], q->v0);
		fb_add(l[0][0], l[0][0], table[1][V1]);
		fb_mul(t, table[FB_BITS - 1][U1], q->v1);
		fb_add(l[0][0], l[0][0], t);
		fb_add(t, table[0][DELTA1], epsil3);
		fb_add(t, t, table[0][DELTA2]);
		fb_add(t, t, delta3);
		fb_add(t, t, p->u0);
		fb_srt(t, t);
		fb_add(l[0][0], l[0][0], t);
		fb_add(l[0][1], table[FB_BITS - 1][DELTA1], table[FB_BITS - 1][DELTA2]);
		fb_add_dig(l[0][1], l[0][1], 1);
		fb_copy(l[0][2], table[FB_BITS - 1][U1]);
		fb_copy(l[0][3], table[FB_BITS - 1][DELTA0]);
		fb_add(l[0][4], table[FB_BITS - 1][DELTA1], table[FB_BITS - 1][DELTA0]);
		fb_copy(l[0][5], p->u1);
		//fb_add(_l[1][0], _l[1][0], delta0[FB_BITS - 1]);

		fb_mul(t, l[0][0], nu[2]);
		fb_add(l0[0][0], l0[0][0], t);
		fb_mul(t, l[0][1], nu[2]);
		fb_add(l0[0][1], l0[0][1], t);
		fb_mul(t, l[0][2], nu[2]);
		fb_add(l0[0][2], l0[0][2], t);
		fb_mul(t, l[0][3], nu[2]);
		fb_add(l0[0][3], l0[0][3], t);
		fb_add(l0[1][0], l0[1][0], t);
		fb_mul(t, l[0][4], nu[2]);
		fb_add(l0[0][4], l0[0][4], t);
		fb_mul(t, l[0][5], nu[2]);
		fb_add(l0[0][5], l0[0][5], t);

		fb_mul(t, l[0][0], mu[2]);
		fb_add(l1[0][0], l1[0][0], t);
		fb_mul(t, l[0][1], mu[2]);
		fb_add(l1[0][1], l1[0][1], t);
		fb_mul(t, l[0][2], mu[2]);
		fb_add(l1[0][2], l1[0][2], t);
		fb_mul(t, l[0][3], mu[2]);
		fb_add(l1[0][3], l1[0][3], t);
		fb_add(l1[1][0], l1[1][0], t);
		fb_mul(t, l[0][4], mu[2]);
		fb_add(l1[0][4], l1[0][4], t);
		fb_mul(t, l[0][5], mu[2]);
		fb_add(l1[0][5], l1[0][5], t);

		fb_mul(l[0][0], table[FB_BITS - 1][DELTA0], q->v1);
		fb_add(l[0][0], l[0][0], table[FB_BITS - 1][DELTA1]);
		fb_add(l[0][0], l[0][0], table[FB_BITS - 1][DELTA2]);
		fb_copy(l[0][2], table[FB_BITS - 1][DELTA0]);
		fb_copy(l[0][4], p->u1);

		fb_mul(t, l[0][0], nu[3]);
		fb_add(l0[0][0], l0[0][0], t);
		fb_mul(t, l[0][2], nu[3]);
		fb_add(l0[0][2], l0[0][2], t);
		fb_mul(t, l[0][4], nu[3]);
		fb_add(l0[0][4], l0[0][4], t);
		fb_mul(t, l[0][0], mu[3]);
		fb_add(l1[0][0], l1[0][0], t);
		fb_mul(t, l[0][2], mu[3]);
		fb_add(l1[0][2], l1[0][2], t);
		fb_mul(t, l[0][4], mu[3]);
		fb_add(l1[0][4], l1[0][4], t);

		fb_add(l[0][0], epsil0, table[0][DELTA2]);
		fb_add(l[0][0], l[0][0], table[1][U1]);
		fb_srt(l[0][0], l[0][0]);
		fb_add_dig(l[0][1], table[FB_BITS - 1][DELTA0], 1);
		//fb_set_dig(_l[0][2], 1);
		//fb_copy(_l[0][4], delta0[FB_BITS - 1]);

		fb_mul(t, l[0][0], nu[4]);
		fb_add(l0[0][0], l0[0][0], t);
		fb_mul(t, l[0][1], nu[4]);
		fb_add(l0[0][1], l0[0][1], t);
		fb_add(l0[0][2], l0[0][2], nu[4]);
		fb_add(l0[0][4], l0[0][4], t);
		fb_add(l0[0][4], l0[0][4], nu[4]);
		fb_mul(t, l[0][0], mu[4]);
		fb_add(l1[0][0], l1[0][0], t);
		fb_mul(t, l[0][1], mu[4]);
		fb_add(l1[0][1], l1[0][1], t);
		fb_add(l1[0][2], l1[0][2], mu[4]);
		fb_add(l1[0][4], l1[0][4], t);
		fb_add(l1[0][4], l1[0][4], mu[4]);

		fb_copy(l[0][0], table[FB_BITS - 1][DELTA0]);
		fb_mul(t, l[0][0], nu[5]);
		fb_add(l0[0][0], l0[0][0], t);
		fb_mul(t, l[0][0], mu[5]);
		fb_add(l1[0][0], l1[0][0], t);

		fb_add(l0[0][0], l0[0][0], nu[6]);
		fb_add(l1[0][0], l1[0][0], mu[6]);

		fb6_conv(l0[0], l0[0]);
		fb6_conv(l0[1], l0[1]);
		fb6_conv(l1[0], l1[0]);
		fb6_conv(l1[1], l1[1]);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t);
		fb12_free(l);
	}
}

void pb_map_etat2_gxg(fb12_t r, hb_t p, hb_t q) {
	fb_t mu[11], nu[11], t;
	int i, j, e1, e2, gamma;
	fb12_t f4, f8;
	fb12_t l1, l0, t0;
	fb_t table[FB_BITS + 2][24];
	fb_t delta[4], epsil[4];
	fb_t mubar[FB_BITS][8], nubar[FB_BITS][8];

	if (FB_BITS % 4 == 1) {
		gamma = 1;
	} else {
		gamma = 0;
	}

	fb_null(t);
	for (i = 0; i < 4; i++) {
		fb_null(delta[i]);
		fb_null(epsil[i]);
	}
	for (i = 0; i < 11; i++) {
		fb_null(mu[i]);
		fb_null(nu[i]);
	}
	for (i = 0; i < FB_BITS; i++) {
		for (j = 0; j < 8; j++) {
			fb_null(mubar[i][j]);
			fb_null(nubar[i][j]);
		}
	}

	TRY {
		fb_new(t);
		for (i = 0; i < 4; i++) {
			fb_new(delta[i]);
			fb_new(epsil[i]);
		}
		for (i = 0; i < 11; i++) {
			fb_new(mu[i]);
			fb_new(nu[i]);
		}
		for (i = 0; i < FB_BITS; i++) {
			for (j = 0; j < 8; j++) {
				fb_new(mubar[i][j]);
				fb_new(nubar[i][j]);
			}
		}

		/* d0 = p.u1^2+p.u1. */
		fb_sqr(delta[0], p->u1);
		fb_add(delta[0], delta[0], p->u1);
		/* d1 = p.u1*p.v1. */
		fb_mul(delta[1], p->u1, p->v1);
		/* d2 = p.u0*p.u1. */
		fb_mul(delta[2], p->u0, p->u1);
		/* d3 = p.u1 * p.v0. */
		fb_mul(delta[3], p->u1, p->v0);

		/* e0 = p.u0^2 + p->u0. */
		fb_sqr(epsil[0], p->u0);
		fb_add(epsil[0], epsil[0], p->u0);
		/* e1 = p.u0 * p.v1 + p.u1 * p.v0. */
		fb_mul(t, p->u1, p->v0);
		fb_mul(epsil[1], p->u0, p->v1);
		fb_add(epsil[1], epsil[1], t);
		/* e2 = p.u0 * p.u1 + p.u1^3 + p.u1 * p.v1 + p.u1. */
		fb_mul(epsil[2], p->u0, p->u1);
		fb_sqr(t, p->u1);
		fb_mul(t, t, p->u1);
		fb_add(epsil[2], epsil[2], t);
		fb_mul(t, p->u1, p->v1);
		fb_add(epsil[2], epsil[2], t);
		fb_add(epsil[2], epsil[2], p->u1);
		/* e3 = p.u1 * e1. */
		fb_mul(epsil[3], p->u1, epsil[1]);

		/* Precompute mubar_i, nubar_i. */
		fb_copy(mu[0], q->u1);
		fb_copy(nu[0], q->u0);
		for (i = 1; i < 11; i++) {
			fb_mul(mu[i], q->u1, mu[i - 1]);
			fb_add(mu[i], mu[i], nu[i - 1]);
			fb_mul(nu[i], q->u0, mu[i - 1]);
		}
		for (i = 0; i < 5; i++) {
			fb_copy(mubar[0][i], mu[i]);
			fb_copy(nubar[0][i], nu[i]);
		}
		fb_copy(mubar[0][5], mu[6]);
		fb_copy(mubar[0][6], mu[8]);
		fb_copy(mubar[0][7], mu[10]);
		fb_copy(nubar[0][5], nu[6]);
		fb_copy(nubar[0][6], nu[8]);
		fb_copy(nubar[0][7], nu[10]);
		for (i = 1; i < FB_BITS; i++) {
			for (j = 0; j < 8; j++) {
				fb_sqr(mubar[i][j], mubar[i - 1][j]);
				fb_sqr(nubar[i][j], nubar[i - 1][j]);
			}
		}

		/* Precompute v0[i] = q.v0^2^i, v1[i] = q.v1^2^i. */
		fb_copy(table[0][V0], q->v0);
		fb_copy(table[0][V1], q->v1);
		for (i = 1; i <= FB_BITS + 1; i++) {
			fb_sqr(table[i][V0], table[i - 1][V0]);
			fb_sqr(table[i][V1], table[i - 1][V1]);
		}

		/* Compute powers of components of a0. */
		/* a0 = (delta1, 0, p.u1, 0, delta0, 0, 0, 0, 0, 0, 0, 0). */
		fb_copy(table[0][DELTA0], delta[0]);
		fb_copy(table[0][DELTA1], delta[1]);
		fb_copy(table[0][DELTA2], delta[2]);
		for (i = 1; i < FB_BITS; i++) {
			fb_sqr(table[i][DELTA0], table[i - 1][DELTA0]);
			fb_sqr(table[i][DELTA1], table[i - 1][DELTA1]);
			fb_sqr(table[i][DELTA2], table[i - 1][DELTA2]);
		}
		fb_copy(table[0][U1], p->u1);
		fb_copy(table[0][U0], p->u0);
		for (i = 1; i <= FB_BITS; i++) {
			fb_sqr(table[i][U1], table[i - 1][U1]);
			fb_sqr(table[i][U0], table[i - 1][U0]);
		}

		/* Compute powers of components of b0,b1,b2,b4. */
		fb_zero(table[0][B00]);
		fb_zero(table[0][B02]);
		fb_zero(table[0][B04]);
		fb_zero(table[0][B10]);
		fb_zero(table[0][B20]);
		fb_zero(table[0][B40]);
		if (gamma) {
			fb_add_dig(table[0][B00], delta[1], 1);
			fb_copy(table[0][B02], p->u1);
			fb_copy(table[0][B10], delta[0]);
			fb_copy(table[0][B20], p->u1);
			fb_copy(table[0][B40], delta[0]);
		}
		fb_add(table[0][B00], table[0][B00], delta[0]);
		fb_add(table[0][B00], table[0][B00], epsil[0]);
		fb_mul(t, p->v1, epsil[1]);
		fb_add(table[0][B00], table[0][B00], t);
		fb_sqr(t, p->v0);
		fb_add(table[0][B00], table[0][B00], t);
		fb_add(table[0][B01], epsil[0], delta[2]);
		fb_add(table[0][B02], table[0][B02], delta[0]);
		fb_add(table[0][B02], table[0][B02], delta[1]);
		fb_add(table[0][B02], table[0][B02], delta[2]);
		fb_add(table[0][B02], table[0][B02], delta[3]);
		fb_add(table[0][B03], delta[0], delta[2]);
		fb_add_dig(table[0][B03], table[0][B03], 1);
		fb_add(table[0][B04], table[0][B04], p->u0);
		fb_add(table[0][B04], table[0][B04], epsil[3]);
		fb_add(table[0][B04], table[0][B04], delta[3]);
		fb_add(table[0][B04], table[0][B04], p->u1);
		fb_add_dig(table[0][B04], table[0][B04], 1);
		fb_add(table[0][B05], delta[2], delta[0]);
		fb_add_dig(table[0][B05], table[0][B05], 1);
		fb_add(table[0][B10], table[0][B10], delta[3]);
		fb_add(table[0][B10], table[0][B10], delta[1]);
		fb_add(table[0][B20], table[0][B20], epsil[3]);
		fb_add(table[0][B20], table[0][B20], delta[3]);
		fb_add(table[0][B20], table[0][B20], table[1][U1]);
		fb_add(table[0][B20], table[0][B20], p->u0);
		fb_add_dig(table[0][B20], table[0][B20], 1);
		fb_add(table[0][B40], epsil[0], delta[2]);
		fb_add(table[0][B40], table[0][B40], p->u1);
		for (i = 1; i < FB_BITS; i++) {
			fb_sqr(table[i][B00], table[i - 1][B00]);
			fb_sqr(table[i][B01], table[i - 1][B01]);
			fb_sqr(table[i][B02], table[i - 1][B02]);
			fb_sqr(table[i][B03], table[i - 1][B03]);
			fb_sqr(table[i][B04], table[i - 1][B04]);
			fb_sqr(table[i][B05], table[i - 1][B05]);
			fb_sqr(table[i][B10], table[i - 1][B10]);
			fb_sqr(table[i][B20], table[i - 1][B20]);
			fb_sqr(table[i][B40], table[i - 1][B40]);
		}

		/* Compute powers of components of c0,c1,c2. */
		fb_copy(table[0][EPSIL2], epsil[2]);
		for (i = 1; i < FB_BITS; i++) {
			fb_sqr(table[i][EPSIL2], table[i - 1][EPSIL2]);
		}

		/* Compute powers of components of d0,d1,d2. */
		fb_zero(table[0][D00]);
		fb_zero(table[0][D01]);
		fb_zero(table[0][D02]);
		fb_zero(table[0][D10]);
		fb_zero(table[0][D20]);
		if (gamma) {
			fb_add_dig(table[0][D00], epsil[2], 1);
			fb_copy(table[0][D01], delta[0]);
			fb_sqr(table[0][D02], p->u1);
			fb_copy(table[0][D10], delta[0]);
			fb_sqr(table[0][D20], p->u1);
		}
		fb_sqr(t, p->v0);
		fb_add(table[0][D00], table[0][D00], t);
		fb_add(t, table[1][U1], p->v1);
		fb_mul(t, t, epsil[1]);
		fb_add(table[0][D00], table[0][D00], t);
		fb_add(table[0][D00], table[0][D00], delta[3]);
		fb_add_dig(t, table[1][U0], 1);
		fb_add(t, t, delta[3]);
		fb_add(t, t, table[1][U1]);
		fb_mul(t, t, p->u0);
		fb_add(table[0][D00], table[0][D00], t);
		fb_add(t, delta[2], epsil[0]);
		fb_mul(t, t, p->u1);
		fb_add(table[0][D01], table[0][D01], t);
		fb_add(table[0][D01], table[0][D01], epsil[3]);
		fb_add(table[0][D01], table[0][D01], delta[3]);
		fb_add(t, table[1][U0], epsil[1]);
		fb_mul(t, t, p->u1);
		fb_add(table[0][D02], table[0][D02], t);
		fb_add(table[0][D02], table[0][D02], epsil[0]);
		fb_add(table[0][D02], table[0][D02], epsil[2]);
		fb_add(t, epsil[0], delta[2]);
		fb_mul(t, t, p->u1);
		fb_add(table[0][D10], table[0][D10], t);
		fb_add(table[0][D10], table[0][D10], table[1][U1]);
		fb_add(table[0][D10], table[0][D10], epsil[3]);
		fb_add(table[0][D10], table[0][D10], delta[3]);
		fb_mul(t, table[1][U1], p->u1);
		fb_add(table[0][D12], delta[1], t);
		fb_add(table[0][D20], table[0][D20], epsil[0]);
		fb_add_dig(table[0][D20], table[0][D20], 1);
		fb_sqr(t, p->u0);
		fb_add(t, t, epsil[1]);
		fb_mul(t, t, p->u1);
		fb_add(table[0][D20], table[0][D20], t);
		fb_sqr(t, p->u1);
		fb_add(table[0][D20], table[0][D20], t);
		for (i = 1; i < FB_BITS; i++) {
			fb_sqr(table[i][D00], table[i - 1][D00]);
			fb_sqr(table[i][D01], table[i - 1][D01]);
			fb_sqr(table[i][D02], table[i - 1][D02]);
			fb_sqr(table[i][D10], table[i - 1][D10]);
			fb_sqr(table[i][D12], table[i - 1][D12]);
			fb_sqr(table[i][D20], table[i - 1][D20]);
		}

		/* f = 1. */
		fb12_zero(r);
		fb12_zero(t0);
		fb_set_dig(r[0][0], 1);

		for (i = 0; i <= (FB_BITS - 3) / 2; i++) {
			/* e1 = (3 * m - 9 - 6 * i)/2 mod m. */
			e1 = (3 * FB_BITS - 9 - 6 * i) / 2;
			e1 %= FB_BITS;
			/* e2 = (3 * m - 3 + 6 * i)/2 mod m. */
			e2 = (3 * FB_BITS - 3 + 6 * i) / 2;
			e2 %= FB_BITS;

			pb_map_g4(l1, l0, mubar, nubar, p, q, e1, e2, table);

			fb12_sqr(f4, l0);
			for (j = 0; j < 6; j++) {
				fb_mul(t0[0][j], l1[0][j], mubar[e1][0]);
			}
			fb_mul(t0[1][0], l1[1][0], mubar[e1][0]);
			fb12_mul_dxs4(l0, l0, t0);
			fb12_sqr(l1, l1);
			for (j = 0; j < 6; j++) {
				fb_mul(l1[0][j], l1[0][j], nubar[e1][0]);
			}
			fb_mul(l1[1][0], l1[1][0], nubar[e1][0]);
			fb12_add(f4, f4, l0);
			fb12_add(f4, f4, l1);

			pb_map_g8(l1, l0, mubar, nubar, p, q, e1, e2, table);
			fb12_sqr(f8, l0);
			for (j = 0; j < 6; j++) {
				fb_mul(t0[0][j], l1[0][j], mubar[e1][0]);
			}
			fb_mul(t0[1][0], l1[1][0], mubar[e1][0]);
			fb12_mul_dxs4(l0, l0, t0);
			fb12_sqr(l1, l1);
			for (j = 0; j < 6; j++) {
				fb_mul(l1[0][j], l1[0][j], nubar[e1][0]);
			}
			fb_mul(l1[1][0], l1[1][0], nubar[e1][0]);
			fb12_add(f8, f8, l0);
			fb12_add(f8, f8, l1);

			fb12_mul(r, r, f4);
			fb12_mul(r, r, f8);
		}

		fb12_sqr(r, r);
		fb12_sqr(r, r);

		line(l0, l1, mu, nu, p, q, gamma, table, delta[3], epsil[0], epsil[3]);

		fb12_zero(t0);
		fb12_sqr(t0, l0);
		fb12_mul(l0, l0, l1);
		fb_mul(l0[0][0], l0[0][0], q->u1);
		fb_mul(l0[0][1], l0[0][1], q->u1);
		fb_mul(l0[0][2], l0[0][2], q->u1);
		fb_mul(l0[0][3], l0[0][3], q->u1);
		fb_mul(l0[0][4], l0[0][4], q->u1);
		fb_mul(l0[0][5], l0[0][5], q->u1);
		fb_mul(l0[1][0], l0[1][0], q->u1);
		fb_mul(l0[1][1], l0[1][1], q->u1);
		fb_mul(l0[1][2], l0[1][2], q->u1);
		fb_mul(l0[1][3], l0[1][3], q->u1);
		fb_mul(l0[1][4], l0[1][4], q->u1);
		fb_mul(l0[1][5], l0[1][5], q->u1);
		fb12_sqr(l1, l1);
		fb_mul(l1[0][0], l1[0][0], q->u0);
		fb_mul(l1[0][1], l1[0][1], q->u0);
		fb_mul(l1[0][2], l1[0][2], q->u0);
		fb_mul(l1[0][3], l1[0][3], q->u0);
		fb_mul(l1[0][4], l1[0][4], q->u0);
		fb_mul(l1[0][5], l1[0][5], q->u0);
		fb_mul(l1[1][0], l1[1][0], q->u0);
		fb_mul(l1[1][1], l1[1][1], q->u0);
		fb_mul(l1[1][2], l1[1][2], q->u0);
		fb_mul(l1[1][3], l1[1][3], q->u0);
		fb_mul(l1[1][4], l1[1][4], q->u0);
		fb_mul(l1[1][5], l1[1][5], q->u0);
		fb12_add(t0, t0, l0);
		fb12_add(t0, t0, l1);
		fb12_mul(r, r, t0);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t);
		for (i = 0; i < 4; i++) {
			fb_free(delta[i]);
			fb_free(epsil[i]);
		}
		for (i = 0; i < 11; i++) {
			fb_free(mu[i]);
			fb_free(nu[i]);
		}
		for (i = 0; i < FB_BITS; i++) {
			for (j = 0; j < 8; j++) {
				fb_free(mubar[i][j]);
				fb_free(nubar[i][j]);
			}
		}
	}
}

/*============================================================================*/
/* Public definitions                                                        */
/*============================================================================*/

void pb_map_etat2(fb12_t r, hb_t p, hb_t q) {
	fb_t xp, yp, xq1, xq2, yq1, yq2;

	fb_null(xp);
	fb_null(yp);
	fb_null(xq1);
	fb_null(yq1);
	fb_null(xq2);
	fb_null(yq2);

	TRY {
		fb_new(xp);
		fb_new(yp);
		fb_new(xq1);
		fb_new(yq1);
		fb_new(xq2);
		fb_new(yq2);

		if (!p->deg && !q->deg) {
			pb_map_etat2_gxg(r, p, q);
			pb_map_exp(r, r);
			return;
		}

		if (p->deg && q->deg) {
			fb_neg(xp, p->u0);
			fb_copy(yp, p->v0);
			fb_neg(xq1, q->u0);
			fb_copy(yq1, q->v0);

			pb_map_etat2_dxd(r, xp, yp, xq1, yq1);
			pb_map_exp(r, r);
			return;
		}

		if (p->deg && !q->deg) {
			fb_neg(xp, p->u0);
			fb_copy(yp, p->v0);

			/* Solve X^2 + X = u0/u1^.2 */
			fb_sqr(xq1, q->u1);
			fb_inv(xq1, xq1);
			fb_mul(xq1, xq1, q->u0);
			fb_slv(xq1, xq1);
			fb_add_dig(xq2, xq1, 1);
			/* Revert change of variables by computing X = X * u1. */
			fb_mul(xq1, xq1, q->u1);
			fb_mul(xq2, xq2, q->u1);
			/* Recover the two points stored in the divisor. */
			fb_mul(yq1, xq1, q->v1);
			fb_add(yq1, yq1, q->v0);
			fb_mul(yq2, xq2, q->v1);
			fb_add(yq2, yq2, q->v0);

			pb_map_etat2_dxa(r, xp, yp, xq1, yq1, xq2, yq2);
			pb_map_exp(r, r);
			return;
		}

		if (!p->deg && q->deg) {
			fb_neg(xp, q->u0);
			fb_copy(yp, q->v0);

			/* Solve X^2 + X = u0/u1^.2 */
			fb_sqr(xq1, p->u1);
			fb_inv(xq1, xq1);
			fb_mul(xq1, xq1, p->u0);
			fb_slv(xq1, xq1);
			fb_add_dig(xq2, xq1, 1);
			/* Revert change of variables by computing X = X * u1. */
			fb_mul(xq1, xq1, p->u1);
			fb_mul(xq2, xq2, p->u1);
			/* Recover the two points stored in the divisor. */
			fb_mul(yq1, xq1, p->v1);
			fb_add(yq1, yq1, p->v0);
			fb_mul(yq2, xq2, p->v1);
			fb_add(yq2, yq2, p->v0);

			pb_map_etat2_axd(r, xq1, yq1, xq2, yq2, xp, yp);
			pb_map_exp(r, r);
			return;
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(xp);
		fb_free(yp);
		fb_free(xq1);
		fb_free(yq1);
		fb_free(xq2);
		fb_free(yq2);
	}
}
