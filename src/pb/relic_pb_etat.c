/*
 * Copyright 2007-2009 RELIC Project
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
#include "relic_pb.h"
#include "relic_util.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/**
 * Computes the final exponentiation of the eta_t pairing.
 *
 * This function maps a random coset element to a fixed coset representative.
 *
 * @param r
 * @param delta
 */
void etat_exp(fb4_t r, fb4_t a) {
	fb2_t t0, t1, t2;
	fb4_t v, w, v2m;
	int delta;
	dig_t b;

	fb2_new(t0);
	fb2_new(t1);
	fb2_new(t2);
	fb4_new(v);
	fb4_new(v2m);
	fb4_new(w);

	b = eb_curve_get_b()[0];
	switch (FB_BITS % 8) {
		case 1:
		case 7:
			delta = b;
			break;
		case 3:
		case 5:
			delta = 1 - b;
			break;
	}

	/* t0 = (m0 + m1) + m1 * s. */
	fb_sqr(t0[0], a[0]);
	fb_sqr(t0[1], a[1]);
	fb_add(t0[0], t0[0], t0[1]);
	/* t1 = (m2 + m3) + m3 * s, t2 = m3 + m2 * s. */
	fb_sqr(t1[0], a[2]);
	fb_sqr(t1[1], a[3]);
	fb_copy(t2[0], t1[1]);
	fb_copy(t2[1], t1[0]);
	fb_add(t1[0], t1[0], t1[1]);
	/* t4 = t0 + t2. */
	fb2_add(t0, t0, t2);
	/* t3 = (u_0 + u_1 * s) * (u_2 + u_3 * s). */
	fb2_mul(t2, a, a + 2);
	/* d = t3 + t4. */
	fb2_add(t2, t2, t0);
	/* d = d^(-1). */
	fb2_inv(t2, t2);
	/* t5 = t1 * d. */
	fb2_mul(t1, t1, t2);
	/* t6 = t4 * d. */
	fb2_mul(t0, t0, t2);
	/* v0 = t5 + t6. */
	fb2_add(v, t1, t0);
	/* v1, w1 = t5. */
	fb_copy(v[2], t1[0]);
	fb_copy(v[3], t1[1]);
	fb_copy(w[2], t1[0]);
	fb_copy(w[3], t1[1]);
	/* if v = -1. */
	if (delta == 1) {
		/* w0 = v0. */
		fb2_copy(w, v);
	} else {
		/* w0 = t6. */
		fb2_copy(w, t0);
	}
	/* v = v0 + v1 * t. */
	/* w = w0 + w1 * t. */

	/* v = v^(2m+1). */
	fb4_exp_2m(r, v);
	fb4_mul(v, r, v);
	for (int i = 0; i < (FB_BITS + 1) / 2; i++) {
		fb4_sqr(w, w);
	}
	fb4_mul(r, v, w);

	fb2_free(t0);
	fb2_free(t1);
	fb2_free(t2);
	fb4_free(v);
	fb4_free(w);
}

void fb4_mul_sparse(fb4_t c, fb4_t a, fb4_t b) {
	fb_t t0, t1, t2, t3, t4, t5, t6;

	fb_new(t0);
	fb_new(t1);
	fb_new(t2);
	fb_new(t3);
	fb_new(t4);
	fb_new(t5);
	fb_new(t6);

	fb_add(t0, b[0], b[1]);
	fb_add(t1, a[0], a[1]);
	fb_add(t2, a[2], a[3]);

	fb_mul(t2, t0, t2);
	fb_mul(t0, t0, t1);

	fb_mul(t3, a[0], b[0]);
	fb_mul(t4, a[1], b[1]);
	fb_mul(t5, a[2], b[0]);
	fb_mul(t6, a[3], b[1]);

	fb_copy(t1, a[3]);

	fb_add(c[3], a[1], a[3]);
	fb_add(c[3], c[3], t2);
	fb_add(c[3], c[3], t5);

	fb_copy(t2, a[2]);

	fb_add(c[2], a[0], a[2]);
	fb_add(c[2], c[2], t5);
	fb_add(c[2], c[2], t6);

	fb_add(c[0], t3, t4);
	fb_add(c[0], c[0], t1);

	fb_add(c[1], t2, t1);
	fb_add(c[1], c[1], t3);
	fb_add(c[1], c[1], t0);

	fb_free(t0);
	fb_free(t1);
	fb_free(t2);
	fb_free(t3);
	fb_free(t4);
	fb_free(t5);
	fb_free(t6);
}

#if PB_MAP == ETATS || !defined(STRIP)

void pb_map_etats_impl(fb4_t r, eb_t p, eb_t q) {
	dig_t alpha, beta, delta, b;

	if (FB_BITS % 4 == 3) {
		alpha = 0;
	} else {
		alpha = 1;
	}

	b = eb_curve_get_b()[0];
	switch (FB_BITS % 8) {
		case 1:
			beta = b;
			delta = b;
			break;
		case 3:
			beta = b;
			delta = 1 - b;
			break;
		case 5:
			beta = 1 - b;
			delta = 1 - b;
			break;
		case 7:
			beta = 1 - b;
			delta = b;
			break;
	}

#ifndef PB_PARAL
	fb_t xp, yp, xq, yq, u, v;
	fb4_t l, g;

	fb_new(xp);
	fb_new(yp);
	fb_new(xq);
	fb_new(yq);
	fb_new(u);
	fb_new(v);
	fb4_new(g);
	fb4_new(l);

	fb_copy(xp, p->x);
	fb_copy(yp, p->y);
	fb_copy(xq, q->x);
	fb_copy(yq, q->y);

	/* y_P = y_P + delta^bar. */
	fb_add_dig(yp, yp, 1 - delta);

	/* u = x_P + alpha, v = x_q + alpha. */
	fb_add_dig(u, xp, alpha);
	fb_add_dig(v, xq, alpha);
	/* g_0 = u * v + y_P + y_Q + beta. */
	fb_mul(g[0], u, v);
	fb_add(g[0], g[0], yp);
	fb_add(g[0], g[0], yq);
	fb_add_dig(g[0], g[0], beta);
	/* g_1 = u + x_Q. */
	fb_add(g[1], u, xq);
	/* G = g_0 + g_1 * s + t. */
	fb_zero(g[2]);
	fb_set_bit(g[2], 0, 1);
	fb_zero(g[3]);
	/* l_0 = g_0 + v + x_P^2. */
	fb_sqr(u, xp);
	fb_add(l[0], g[0], v);
	fb_add(l[0], l[0], u);
	/* L = l_0 + (g_1 + 1) * s + t. */
	fb_add_dig(l[1], g[1], 1);
	fb_zero(l[2]);
	fb_set_bit(l[2], 0, 1);
	fb_zero(l[3]);
	/* F = L * G. */
	fb4_mul_sparse(r, l, g);

	for (int i = 0; i < (FB_BITS - 1) / 2; i++) {
		/* x_P = sqrt(x_P), y_P = sqr(y_P). */
		fb_srt(xp, xp);
		fb_srt(yp, yp);
		/* x_Q = x_Q^2, y_Q = y_Q^2. */
		fb_sqr(xq, xq);
		fb_sqr(yq, yq);

		/* u = x_P + alpha, v = x_q + alpha. */
		fb_add_dig(u, xp, alpha);
		fb_add_dig(v, xq, alpha);
		/* g_0 = u * v + y_P + y_Q + beta. */
		fb_mul(g[0], u, v);
		fb_add(g[0], g[0], yp);
		fb_add(g[0], g[0], yq);
		fb_add_dig(g[0], g[0], beta);
		/* g_1 = u + x_Q. */
		fb_add(g[1], u, xq);
		/* G = g_0 + g_1 * s + t. */

		/* F = F * G. */
		fb4_mul_sparse(r, r, g);
	}
#endif
}

#endif

#if PB_MAP == ETATN || !defined(STRIP)

void pb_map_etatn_impl(fb4_t r, eb_t p, eb_t q) {
	dig_t alpha, beta, delta, b;

	if (FB_BITS % 4 == 3) {
		alpha = 0;
	} else {
		alpha = 1;
	}

	b = eb_curve_get_b()[0];
	switch (FB_BITS % 8) {
		case 1:
			beta = b;
			delta = b;
			break;
		case 3:
			beta = b;
			delta = 1 - b;
			break;
		case 5:
			beta = 1 - b;
			delta = 1 - b;
			break;
		case 7:
			beta = 1 - b;
			delta = b;
			break;
	}

#ifndef PB_PARAL
	fb_t xp, yp, xq, yq, u, v;
	fb4_t f, l, g;

	fb_new(xp);
	fb_new(yp);
	fb_new(xq);
	fb_new(yq);
	fb_new(u);
	fb_new(v);
	fb4_new(f);
	fb4_new(g);
	fb4_new(l);

	fb_copy(xp, p->x);
	fb_copy(yp, p->y);
	fb_copy(xq, q->x);
	fb_copy(yq, q->y);

	/* y_P = y_P + delta^bar. */
	fb_add_dig(yp, yp, 1 - delta);
	/* x_P = x_P^2. */
	fb_sqr(xp, xp);
	/* y_P = y_P^2. */
	fb_sqr(yp, yp);
	/* y_P = y_P + b. */
	fb_add_dig(yp, yp, b);
	/* u = x_P + 1. */
	fb_add_dig(u, xp, 1);
	/* g_1 = u + x_Q. */
	fb_add(g[1], u, xq);
	/* g_0 = x_P * x_Q + y_P + y_Q + g1. */
	fb_mul(g[0], xp, xq);
	fb_add(g[0], g[0], yp);
	fb_add(g[0], g[0], yq);
	fb_add(g[0], g[0], g[1]);
	/* x_Q = x_Q + 1. */
	fb_add_dig(xq, xq, 1);
	/* G = g_0 + g_1 * s + t. */
	fb_zero(g[2]);
	fb_set_bit(g[2], 0, 1);
	fb_zero(g[3]);
	/* l_0 = g_0 + x_Q + x_P^2. */
	fb_sqr(v, xp);
	fb_add(l[0], g[0], xq);
	fb_add(l[0], l[0], v);
	/* L = l_0 + (g_1 + 1) * s + t. */
	fb_add_dig(l[1], g[1], 1);
	fb_zero(l[2]);
	fb_set_bit(l[2], 0, 1);
	fb_zero(l[3]);

	/* F = L * G. */
	fb4_mul_sparse(r, l, g);

	for (int i = 0; i < (FB_BITS - 1) / 2; i++) {
		/* F = F^2. */
		fb4_sqr(r, r);
		/* x_Q = x_Q^4, y_Q = y_Q^4. */
		fb_sqr(xq, xq);
		fb_sqr(xq, xq);
		fb_sqr(yq, yq);
		fb_sqr(yq, yq);

		/* x_Q = x_Q + 1, y_Q = y_Q + x_Q. */
		fb_add_dig(xq, xq, 1);
		fb_add(yq, yq, xq);
		/* g_0 = u * x_Q + y_P + y_Q. */
		fb_mul(g[0], u, xq);
		fb_add(g[0], g[0], yp);
		fb_add(g[0], g[0], yq);
		/* g_1 = x_P + x_Q. */
		fb_add(g[1], xp, xq);
		/* G = g_0 + g_1 * s + t. */
		/* F = F * G. */
		fb4_mul_sparse(r, r, g);
	}
#endif
}

#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if PB_MAP == ETATS || !defined(STRIP)

void pb_map_etats(fb4_t r, eb_t p, eb_t q) {
	pb_map_etats_impl(r, p, q);
	etat_exp(r, r);
}

#endif

#if PB_MAP == ETATN || !defined(STRIP)

void pb_map_etatn(fb4_t r, eb_t p, eb_t q) {
	pb_map_etatn_impl(r, p, q);
	etat_exp(r, r);
}

#endif
