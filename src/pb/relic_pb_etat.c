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
 * @param[out] r					- the result.
 * @param[in] a						- the random coset element.
 */
static void etat_exp(fb4_t r, fb4_t a) {
	fb2_t t0, t1, t2;
	fb4_t v, w;
	int i = 0, delta, mod, to;
	dig_t b;

	fb2_null(t0);
	fb2_null(t1);
	fb2_null(t2);
	fb4_null(v);
	fb4_null(w);

	TRY {
		fb2_new(t0);
		fb2_new(t1);
		fb2_new(t2);
		fb4_new(v);
		fb4_new(w);

		mod = FB_BITS % 8;
		b = eb_curve_get_b()[0];

		switch (mod) {
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

		to = ((FB_BITS + 1) / 2) >> 2;
		to = to << 2;
		for (i = 0; i < to; i++) {
			/* This is faster than calling fb4_sqr(w, w) (no field additions). */
			fb_sqr(w[0], w[0]);
			fb_sqr(w[1], w[1]);
			fb_sqr(w[2], w[2]);
			fb_sqr(w[3], w[3]);
		}
		for (; i < (FB_BITS + 1) / 2; i++) {
			fb4_sqr(w, w);
		}
		fb4_mul(r, v, w);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb2_free(t0);
		fb2_free(t1);
		fb2_free(t2);
		fb4_free(v);
		fb4_free(w);
	}
}

#if PB_MAP == ETATS || !defined(STRIP)

static void pb_map_etats_impl(fb4_t r, eb_t p, eb_t q) {
	dig_t alpha, beta, delta, b;
	int mod;

	if (FB_BITS % 4 == 3) {
		alpha = 0;
	} else {
		alpha = 1;
	}

	b = eb_curve_get_b()[0];
	mod = FB_BITS % 8;
	switch (mod) {
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

	fb_null(xp);
	fb_null(yp);
	fb_null(xq);
	fb_null(yq);
	fb_null(u);
	fb_null(v);
	fb4_null(g);
	fb4_null(l);

	TRY {
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
		fb4_mul_sxs(r, l, g);

		for (int i = 0; i < ((FB_BITS - 1) / 2); i ++) {
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
			fb4_mul_dxs(r, r, g);
		}
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(xp);
		fb_free(yp);
		fb_free(xq);
		fb_free(yq);
		fb_free(u);
		fb_free(v);
		fb4_free(g);
		fb4_free(l);
	}
#else
	/* F = 1, L = L * G. */
	fb4_zero(r);
	fb_set_bit(r[0], 0, 1);

	fb4_t _f[CORES];
	omp_set_num_threads(CORES);
	TRY {
		for (int j = 0; j < CORES; j++) {
			fb4_null(_f[j]);
			fb4_new(_f[j]);
		}
#pragma omp parallel firstprivate(alpha, beta, delta) shared(r, _f, p, q) default(shared)
		{
			int i = omp_get_thread_num();
			int from, to;
			fb_t xp, yp, xq, yq, u, v;
			fb4_t l, g;

			fb_null(xp);
			fb_null(yp);
			fb_null(xq);
			fb_null(yq);
			fb_null(u);
			fb_null(v);
			fb4_null(g);
			fb4_null(l);

			TRY {
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

				fb4_zero(_f[i]);
				fb_zero(g[2]);
				fb_set_bit(g[2], 0, 1);
				fb_zero(g[3]);

				if (i == 0) {
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

					fb4_mul_sxs(_f[0], l, g);
				} else {
					fb_set_bit(_f[i][0], 0, 1);
				}
#ifdef CORE45NM
				int s2[] = { 0, 308, (FB_BITS - 1) / 2 };
				int s4[] = { 0, 156, 310, 462, (FB_BITS - 1) / 2 };
				int s8[] = { 0, 80, 160, 238, 315, 391, 465, 539,
					(FB_BITS - 1) / 2
				};
				int s16[] = { 0, 43, 85, 127, 168, 208, 248, 287, 325, 363, 400,
					437,
					473,
					508, 543, 577, (FB_BITS - 1) / 2
				};

				switch (CORES) {
					case 1:
						from = 0;
						to = (FB_BITS - 1) / 2;
						break;
					case 2:
						from = s2[i];
						to = s2[i + 1];
						break;
					case 4:
						from = s4[i];
						to = s4[i + 1];
						break;
					case 8:
						from = s8[i];
						to = s8[i + 1];
						break;
					case 16:
						from = s16[i];
						to = s16[i + 1];
						break;
					case 32:
						if (i == 0) {
							from = 0;
							to = 24;
						}
						if (i == 1) {
							from = 24;
							to = 48;
						}
						if (i == 2) {
							from = 48;
							to = 71;
						}
						if (i == 3) {
							from = 71;
							to = 94;
						}
						if (i == 4) {
							from = 94;
							to = 117;
						}
						if (i == 5) {
							from = 117;
							to = 140;
						}
						if (i == 6) {
							from = 140;
							to = 162;
						}
						if (i == 7) {
							from = 162;
							to = 183;
						}
						if (i == 8) {
							from = 183;
							to = 205;
						}
						if (i == 9) {
							from = 205;
							to = 226;
						}
						if (i == 10) {
							from = 226;
							to = 246;
						}
						if (i == 11) {
							from = 246;
							to = 267;
						}
						if (i == 12) {
							from = 267;
							to = 287;
						}
						if (i == 13) {
							from = 287;
							to = 306;
						}
						if (i == 14) {
							from = 306;
							to = 326;
						}
						if (i == 15) {
							from = 326;
							to = 345;
						}
						if (i == 16) {
							from = 345;
							to = 363;
						}
						if (i == 17) {
							from = 363;
							to = 382;
						}
						if (i == 18) {
							from = 382;
							to = 400;
						}
						if (i == 19) {
							from = 400;
							to = 418;
						}
						if (i == 20) {
							from = 418;
							to = 435;
						}
						if (i == 21) {
							from = 435;
							to = 453;
						}
						if (i == 22) {
							from = 453;
							to = 470;
						}
						if (i == 23) {
							from = 470;
							to = 486;
						}
						if (i == 24) {
							from = 486;
							to = 503;
						}
						if (i == 25) {
							from = 503;
							to = 519;
						}
						if (i == 26) {
							from = 519;
							to = 535;
						}
						if (i == 27) {
							from = 535;
							to = 551;
						}
						if (i == 28) {
							from = 551;
							to = 566;
						}
						if (i == 29) {
							from = 566;
							to = 581;
						}
						if (i == 30) {
							from = 581;
							to = 596;
						}
						if (i == 31) {
							from = 596;
							to = (FB_BITS - 1) / 2;
						}
						break;
				}
#else
				int s2[] = { 0, 309, (FB_BITS - 1) / 2 };
				int s4[] = { 0, 158, 312, 463, (FB_BITS - 1) / 2 };
				int s8[] = { 0, 82, 163, 242, 319, 395, 468, 540,
					(FB_BITS - 1) / 2
				};
				int s16[] = { 0, 45, 89, 132, 174, 215, 255, 294, 333, 370, 407,
					443,
					478,
					513, 546, 579, (FB_BITS - 1) / 2
				};

				switch (CORES) {
					case 1:
						from = 0;
						to = (FB_BITS - 1) / 2;
						break;
					case 2:
						from = s2[i];
						to = s2[i + 1];
						break;
					case 4:
						from = s4[i];
						to = s4[i + 1];
						break;
					case 8:
						from = s8[i];
						to = s8[i + 1];
						break;
					case 16:
						from = s16[i];
						to = s16[i + 1];
						break;
					case 32:
						if (i == 0) {
							from = 0;
							to = 26;
						}
						if (i == 1) {
							from = 26;
							to = 52;
						}
						if (i == 2) {
							from = 52;
							to = 77;
						}
						if (i == 3) {
							from = 77;
							to = 102;
						}
						if (i == 4) {
							from = 102;
							to = 126;
						}
						if (i == 5) {
							from = 126;
							to = 150;
						}
						if (i == 6) {
							from = 150;
							to = 173;
						}
						if (i == 7) {
							from = 173;
							to = 196;
						}
						if (i == 8) {
							from = 196;
							to = 218;
						}
						if (i == 9) {
							from = 218;
							to = 240;
						}
						if (i == 10) {
							from = 240;
							to = 261;
						}
						if (i == 11) {
							from = 261;
							to = 282;
						}
						if (i == 12) {
							from = 282;
							to = 302;
						}
						if (i == 13) {
							from = 302;
							to = 322;
						}
						if (i == 14) {
							from = 322;
							to = 341;
						}
						if (i == 15) {
							from = 341;
							to = 360;
						}
						if (i == 16) {
							from = 360;
							to = 378;
						}
						if (i == 17) {
							from = 378;
							to = 397;
						}
						if (i == 18) {
							from = 397;
							to = 414;
						}
						if (i == 19) {
							from = 414;
							to = 431;
						}
						if (i == 20) {
							from = 431;
							to = 448;
						}
						if (i == 21) {
							from = 448;
							to = 465;
						}
						if (i == 22) {
							from = 465;
							to = 481;
						}
						if (i == 23) {
							from = 481;
							to = 497;
						}
						if (i == 24) {
							from = 497;
							to = 512;
						}
						if (i == 25) {
							from = 512;
							to = 527;
						}
						if (i == 26) {
							from = 527;
							to = 542;
						}
						if (i == 27) {
							from = 542;
							to = 556;
						}
						if (i == 28) {
							from = 556;
							to = 571;
						}
						if (i == 29) {
							from = 571;
							to = 584;
						}
						if (i == 30) {
							from = 584;
							to = 598;
						}
						if (i == 31) {
							from = 598;
							to = (FB_BITS - 1) / 2;
						}
						break;
				}
#endif
				for (int j = 0; j < from; j++) {
					/* Compute starting point. */
					fb_srt(xp, xp);
					fb_srt(yp, yp);
					fb_sqr(xq, xq);
					fb_sqr(yq, yq);
				}

				for (int j = from; j < to; j++) {
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
					fb4_mul_dxs(_f[i], _f[i], g);
				}
			} CATCH_ANY {
				THROW(ERR_CAUGHT);
			} FINALLY {
				fb_free(xp);
				fb_free(yp);
				fb_free(xq);
				fb_free(yq);
				fb_free(u);
				fb_free(v);
				fb4_free(f);
				fb4_free(g);
				fb4_free(l);
			}
#pragma omp barrier
			for (int s = 1; s < CORES; s *= 2) {
				if (i % (2 * s) == 0) {
					fb4_mul(_f[i], _f[i], _f[i + s]);
				}
#pragma omp barrier
			}
		}

		fb4_copy(r, _f[0]);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT)
	}
	FINALLY {
		for (int j = 1; j < CORES; j++) {
			fb4_free(_f[j]);
		}
	}
#endif
}

#endif

#if PB_MAP == ETATN || !defined(STRIP)

static void pb_map_etatn_impl(fb4_t r, eb_t p, eb_t q) {
	dig_t delta, b;
	int mod;

	b = eb_curve_get_b()[0];
	mod = FB_BITS % 8;
	switch (mod) {
		case 1:
			delta = b;
			break;
		case 3:
			delta = 1 - b;
			break;
		case 5:
			delta = 1 - b;
			break;
		case 7:
			delta = b;
			break;
	}

#ifndef PB_PARAL
	fb_t xp, yp, xq, yq, u, v;
	fb4_t l, g;

	fb_null(xp);
	fb_null(yp);
	fb_null(xq);
	fb_null(yq);
	fb_null(u);
	fb_null(v);
	fb4_null(g);
	fb4_null(l);

	TRY {
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
		fb4_mul_sxs(r, l, g);

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
			fb4_mul_dxs(r, r, g);
		}
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(xp);
		fb_free(yp);
		fb_free(xq);
		fb_free(yq);
		fb_free(u);
		fb_free(v);
		fb4_free(g);
		fb4_free(l);
	}
#else
	/* F = 1, L = L * G. */
	fb4_zero(r);
	fb_set_bit(r[0], 0, 1);

	fb4_t _f[CORES];
	omp_set_num_threads(CORES);

	TRY {
		for (int j = 0; j < CORES; j++) {
			fb4_null(_f[j]);
			fb4_new(_f[j]);
		}

#pragma omp parallel firstprivate(b, delta) shared(r, _f, p, q) default(shared)
		{
			int i = omp_get_thread_num();
			int from, to, chunk;
			fb_t xp, yp, xq, yq, u, v;
			fb4_t f, l, g;

			chunk = (int)ceilf((FB_BITS - 1) / (2.0 * CORES));

			from = i * chunk;
			to = MIN((FB_BITS - 1) / 2, from + chunk);

			fb_null(xp);
			fb_null(yp);
			fb_null(xq);
			fb_null(yq);
			fb_null(u);
			fb_null(v);
			fb4_null(f);
			fb4_null(g);
			fb4_null(l);

			TRY {
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

				fb4_zero(_f[i]);
				fb_zero(g[2]);
				fb_set_bit(g[2], 0, 1);
				fb_zero(g[3]);

				/* x_P = x_P^2. */
				fb_sqr(xp, xp);
				/* y_P = y_P^2. */
				fb_sqr(yp, yp);
				/* y_P = y_P + b. */
				fb_add_dig(yp, yp, b);
				/* u = x_P + 1. */
				fb_add_dig(u, xp, 1);

				if (i == 0) {
					/* g_1 = u + x_Q. */
					fb_add(g[1], u, xq);
					/* g_0 = x_P * x_Q + y_P + y_Q + g1. */
					fb_mul(g[0], xp, xq);
					fb_add(g[0], g[0], yp);
					fb_add(g[0], g[0], yq);
					fb_add(g[0], g[0], g[1]);
					/* x_Q = x_Q + 1. */
					fb_add_dig(xq, xq, 1);
					/* l_0 = g_0 + x_Q + x_P^2. */
					fb_sqr(v, xp);
					fb_add(l[0], g[0], xq);
					fb_add(l[0], l[0], v);
					/* L = l_0 + (g_1 + 1) * s + t. */
					fb_add_dig(l[1], g[1], 1);
					fb_zero(l[2]);
					fb_set_bit(l[2], 0, 1);
					fb_zero(l[3]);

					fb4_mul_sxs(_f[0], l, g);
				} else {
					/* x_Q = x_Q + 1. */
					fb_add_dig(xq, xq, 1);

					fb_set_bit(_f[i][0], 0, 1);
				}

				for (int j = 0; j < from; j++) {
					/* Compute starting point. */
					fb_sqr(xq, xq);
					fb_sqr(xq, xq);
					fb_sqr(yq, yq);
					fb_sqr(yq, yq);
					fb_add_dig(xq, xq, 1);
					fb_add(yq, yq, xq);
				}

				for (int j = from; j < to; j++) {
					/* F = F^2. */
					fb4_sqr(_f[i], _f[i]);

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

					/* F = F * G. */
					fb4_mul_dxs(_f[i], _f[i], g);
				}

				to = (FB_BITS - 1) / 2 - to;

				for (int j = 0; j < to; j++) {
					fb4_sqr(_f[i], _f[i]);
				}
			} CATCH_ANY {
				THROW(ERR_CAUGHT);
			} FINALLY {
				fb_free(xp);
				fb_free(yp);
				fb_free(xq);
				fb_free(yq);
				fb_free(u);
				fb_free(v);
				fb4_free(f);
				fb4_free(g);
				fb4_free(l);
			}
#pragma omp barrier
			for (int s = 1; s < CORES; s *= 2) {
				if (i % (2 * s) == 0) {
					fb4_mul(_f[i], _f[i], _f[i + s]);
				}
#pragma omp barrier
			}
		}
		fb4_copy(r, _f[0]);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		for (int j = 1; j < CORES; j++) {
			fb4_free(_f[j]);
		}
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
