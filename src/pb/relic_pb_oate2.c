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
 * Implementation of the eta_t bilinear pairing over genus 2 supersingular
 * curves.
 *
 * @version $Id: relic_pb_etat.c 217 2010-01-25 03:11:24Z dfaranha $
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
		fb12_copy(r, a);
		for (i = 0; i < 6; i++) {
			fb12_frb(r, r);
		}
		fb12_mul(r, r, v);

		/* r = f^(2^2m + 1). */
		fb12_copy(v, r);
		for (i = 0; i < 2; i++) {
			fb12_frb(v, v);
		}
		fb12_mul(r, r, v);

		/* v = f^(m+1)/2. */
		to = ((FB_BITS + 1) / 2) / 6;
		to = to * 6;
		fb12_copy(v, r);
		for (i = 0; i < to; i++) {
			/* This is faster than calling fb12_sqr(alpha, alpha) (no field additions). */
			fb_sqr(v[0][0], v[0][0]);
			fb_sqr(v[0][1], v[0][1]);
			fb_sqr(v[0][2], v[0][2]);
			fb_sqr(v[0][3], v[0][3]);
			fb_sqr(v[0][4], v[0][4]);
			fb_sqr(v[0][5], v[0][5]);
			fb_sqr(v[1][0], v[1][0]);
			fb_sqr(v[1][1], v[1][1]);
			fb_sqr(v[1][2], v[1][2]);
			fb_sqr(v[1][3], v[1][3]);
			fb_sqr(v[1][4], v[1][4]);
			fb_sqr(v[1][5], v[1][5]);
		}
		if ((to / 6) % 2 == 1) {
			fb6_add(v[0], v[0], v[1]);
		}
		for (; i < (FB_BITS + 1) / 2; i++) {
			fb12_sqr(v, v);
		}
		fb12_frb(w, v);
		fb12_mul(w, w, v);
		fb12_inv(w, w);

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

void pb_map_res3(fb12_t l, fb12_t f1, fb12_t f0, hb_t q) {
	fb_t s0[10];
	fb_t s1[7], t0, t1, t2, t3, t4, t5, t6, t7, r[12];
	fb_t u0, u1, u2, u3, u6, u7, u8, u9, u10, u11, u12, u13, u14;
	fb_t v0, v1, v2, v3, v4, v6, v7, v8, v9, v10, v11, v12, v13, v14;
	fb_t p0, p1, p2, p3, p4, p5, p6, p7, p8, p9, p10, p11, p12, p13, p14;

	for (int i = 0; i < 6; i++) {
		fb_sqr(s0[i], f0[0][i]);
		fb_sqr(s1[i], f1[0][i]);
		fb_mul(f1[0][i], f1[0][i], q->u1);
	}

	fb_add(t0, s0[2], s0[4]);
	fb_add(t1, s0[2], s0[5]);
	fb_add(r[0], s0[0], t0);
	fb_add_dig(r[0], r[0], 1);
	fb_copy(r[1], t1);
	fb_copy(r[2], t0);
	fb_copy(r[3], s0[5]);
	fb_add(r[4], s0[1], t1);
	fb_add_dig(r[4], r[4], 1);
	fb_copy(r[5], s0[5]);

	fb_mul(p0, s1[0], q->u0);
	fb_mul(p1, s1[0], q->u1);
	fb_mul(p2, s1[1], q->u0);
	fb_mul(p3, s1[1], q->u1);
	fb_mul(p4, s1[2], q->u0);
	fb_mul(p5, s1[2], q->u1);
	fb_mul(p6, s1[4], q->u0);
	fb_mul(p7, s1[4], q->u1);

	fb_add(t0, s1[1], p4);
	fb_add(t1, p5, p7);
	fb_add(t2, s1[4], p3);
	fb_add(t3, s1[2], p6);
	fb_add(t3, t3, t0);
	fb_add(t4, p5, t2);

	fb_add(r[0], r[0], p0);
	fb_add(r[0], r[0], t3);
	fb_add(r[1], r[1], t0);
	fb_add(r[1], r[1], t4);
	fb_add(r[2], r[2], p1);
	fb_add(r[2], r[2], t1);
	fb_add(r[2], r[2], t3);
	fb_add(r[3], r[3], s1[2]);
	fb_add(r[3], r[3], t2);
	fb_add(r[4], r[4], s1[0]);
	fb_add(r[4], r[4], s1[4]);
	fb_add(r[4], r[4], p2);
	fb_add(r[4], r[4], t0);
	fb_add(r[4], r[4], t1);
	fb_add(r[5], r[5], t4);

	fb_add(t0, f1[0][0], f1[0][1]);
	fb_add(t1, f1[0][2], f1[0][4]);
	fb_add(t2, f1[0][1], t1);
	fb_add(u0, f1[0][0], f1[0][4]);
	fb_copy(u1, f1[0][4]);
	fb_copy(u2, f1[0][0]);
	fb_copy(u3, t1);
	fb_add(u6, f1[0][0], t2);
	fb_copy(u7, f1[0][4]);
	fb_add(u8, f1[0][2], t0);
	fb_add(u9, f1[0][0], f1[0][2]);
	fb_copy(u10, t2);
	fb_add(u11, f1[0][4], t0);
	fb_copy(u12, t0);
	fb_copy(u13, t2);
	fb_add(u14, f1[0][0], t1);

	fb_add(t0, f0[0][0], f0[0][1]);
	fb_add(t1, f0[0][4], f0[0][5]);
	fb_add(t2, f0[0][2], f0[0][4]);
	fb_add(t2, t2, t0);
	fb_add_dig(t3, f0[0][4], 1);
	fb_add_dig(t4, f0[0][5], 1);
	fb_add(t5, f0[0][2], t1);

	fb_add(v0, f0[0][0], f0[0][4]);
	fb_add_dig(v1, t1, 1);
	fb_add(v2, f0[0][0], t4);
	fb_copy(v3, t5);
	fb_copy(v4, t4);
	fb_copy(v6, t2);
	fb_copy(v7, t3);
	fb_add(v8, t2, t3);
	fb_add(v9, f0[0][0], f0[0][2]);
	fb_add(v10, f0[0][1], t5);
	fb_add(v11, t0, t1);
	fb_copy(v12, t0);
	fb_add(v13, f0[0][0], t2);
	fb_add(v14, f0[0][1], t2);

	fb_mul(p0, u0, v0);
	fb_mul(p1, u1, v1);
	fb_mul(p2, u2, v2);
	fb_mul(p3, u3, v3);
	fb_mul(p4, u3, v4);
	fb_mul(p6, u6, v6);
	fb_mul(p7, u7, v7);
	fb_mul(p8, u8, v8);
	fb_mul(p9, u9, v9);
	fb_mul(p10, u10, v10);
	fb_mul(p11, u11, v11);
	fb_mul(p12, u12, v12);
	fb_mul(p13, u13, v13);
	fb_mul(p14, u14, v14);

	fb_add(t0, p7, p8);
	fb_add(t1, p1, p2);
	fb_add(t2, p6, p7);
	fb_add(t3, p0, p10);
	fb_add(t3, t3, t0);
	fb_add(t4, p12, p14);
	fb_add(t4, t4, t1);
	fb_add(t5, p9, p11);
	fb_add(t5, t5, t2);
	fb_add(t6, p4, t4);
	fb_add(t7, p1, p3);
	fb_add(t7, t7, p13);
	fb_add(t7, t7, p14);
	fb_add(t7, t7, t3);

	fb_add(r[0], r[0], p3);
	fb_add(r[0], r[0], t0);
	fb_add(r[0], r[0], t5);
	fb_add(r[0], r[0], t6);
	fb_add(r[1], r[1], p11);
	fb_add(r[1], r[1], t7);
	fb_add(r[2], r[2], p2);
	fb_add(r[2], r[2], p9);
	fb_add(r[2], r[2], t3);
	fb_add(r[3], r[3], t1);
	fb_add(r[3], r[3], t5);
	fb_add(r[4], r[4], p10);
	fb_add(r[4], r[4], t4);
	fb_add(r[4], r[4], t7);
	fb_add(r[5], r[5], t2);
	fb_add(r[5], r[5], t6);
	fb_add_dig(r[6], u0, 1);
	fb_copy(r[7], f1[0][4]);
	fb_add(r[8], f1[0][1], f1[0][4]);
	fb_zero(r[9]);
	fb_copy(r[10], u3);
	fb_zero(r[11]);

	fb12_zero(l);
	for (int i = 0; i < 6; i++) {
		fb_add(l[0][i], l[0][i], r[i]);
	}
	for (int i = 0; i < 6; i++) {
		fb_add(l[1][i], l[1][i], r[i+6]);
	}
}

void pb_map_res2(fb12_t l, fb12_t f1, fb12_t f0, hb_t q) {
	fb_t s0[10];
	fb_t s1[7], t, t0, t1, t2, t3, t4, t5, t6, t7, r[12];

	for (int i = 0; i < 6; i++) {
		fb_sqr(s0[i], f0[0][i]);
		fb_sqr(s1[i], f1[0][i]);
		fb_mul(f1[0][i], f1[0][i], q->u1);
	}
	fb_add(t0, s0[2], s0[4]);
	fb_add(t1, s0[2], s0[5]);
	fb_add(r[0], s0[0], s0[3]);
	fb_add(r[0], r[0], t0);
	fb_copy(r[1], t1);
	fb_copy(r[2], t0);
	fb_copy(r[3], s0[5]);
	fb_add(r[4], s0[1], s0[3]);
	fb_add(r[4], r[4], t1);
	fb_copy(r[5], s0[5]);
	fb_copy(r[6], s0[3]);

	fb_t u0, u1, u2, u3, u6, u7, u8, u9, u10, u11, u12, u13, u14;
	fb_t v0, v1, v2, v3, v4, v6, v7, v8, v9, v10, v11, v12, v13, v14;
	fb_t p0, p1, p2, p3, p4, p5, p6, p7, p8, p9, p10, p11, p12, p13, p14;

	fb_mul(p0, s1[0], q->u0);
	fb_mul(p1, s1[0], q->u1);
	fb_mul(p2, s1[1], q->u0);
	fb_mul(p3, s1[1], q->u1);
	fb_mul(p4, s1[2], q->u0);
	fb_mul(p5, s1[2], q->u1);
	fb_mul(p6, s1[4], q->u0);
	fb_mul(p7, s1[4], q->u1);

	fb_add(t0, s1[1], p4);
	fb_add(t1, p5, p7);
	fb_add(t2, s1[4], p3);
	fb_add(t3, s1[2], p6);
	fb_add(t3, t3, t0);
	fb_add(t4, p5, t2);

	fb_add(r[0], r[0], p0);
	fb_add(r[0], r[0], t3);
	fb_add(r[1], r[1], t0);
	fb_add(r[1], r[1], t4);
	fb_add(r[2], r[2], p1);
	fb_add(r[2], r[2], t1);
	fb_add(r[2], r[2], t3);
	fb_add(r[3], r[3], s1[2]);
	fb_add(r[3], r[3], t2);
	fb_add(r[4], r[4], s1[0]);
	fb_add(r[4], r[4], s1[4]);
	fb_add(r[4], r[4], p2);
	fb_add(r[4], r[4], t0);
	fb_add(r[4], r[4], t1);
	fb_add(r[5], r[5], t4);

	fb_add(t0, f1[0][0], f1[0][1]);
	fb_add(t1, f1[0][2], f1[0][4]);
	fb_add(t2, f1[0][1], t1);
	fb_add(u0, f1[0][0], f1[0][4]);
	fb_copy(u1, f1[0][4]);
	fb_copy(u2, f1[0][0]);
	fb_copy(u3, t1);
	fb_add(u6, f1[0][0], t2);
	fb_copy(u7, f1[0][4]);
	fb_add(u8, f1[0][2], t0);
	fb_add(u9, f1[0][0], f1[0][2]);
	fb_copy(u10, t2);
	fb_add(u11, f1[0][4], t0);
	fb_copy(u12, t0);
	fb_copy(u13, t2);
	fb_add(u14, f1[0][0], t1);

	fb_add(t0, f0[0][0], f0[0][1]);
	fb_add(t1, f0[0][4], f0[0][5]);
	fb_add(t2, f0[0][2], f0[0][4]);
	fb_add(t2, t2, t0);
	fb_add(t3, f0[0][3], f0[0][4]);
	fb_add(t4, f0[0][3], f0[0][5]);
	fb_add(t5, f0[0][2], t1);

	fb_add(v0, f0[0][0], f0[0][4]);
	fb_add(v1, f0[0][3], t1);
	fb_add(v2, f0[0][0], t4);
	fb_copy(v3, t5);
	fb_copy(v4, t4);
	fb_copy(v6, t2);
	fb_copy(v7, t3);
	fb_add(v8, t2, t3);
	fb_add(v9, f0[0][0], f0[0][2]);
	fb_add(v10, f0[0][1], t5);
	fb_add(v11, t0, t1);
	fb_copy(v12, t0);
	fb_add(v13, f0[0][0], t2);
	fb_add(v14, f0[0][1], t2);

	fb_mul(p0, u0, v0);
	fb_mul(p1, u1, v1);
	fb_mul(p2, u2, v2);
	fb_mul(p3, u3, v3);
	fb_mul(p4, u3, v4);
	fb_mul(p6, u6, v6);
	fb_mul(p7, u7, v7);
	fb_mul(p8, u8, v8);
	fb_mul(p9, u9, v9);
	fb_mul(p10, u10, v10);
	fb_mul(p11, u11, v11);
	fb_mul(p12, u12, v12);
	fb_mul(p13, u13, v13);
	fb_mul(p14, u14, v14);

	fb_add(t0, p7, p8);
	fb_add(t1, p1, p2);
	fb_add(t2, p6, p7);
	fb_add(t3, p0, p10);
	fb_add(t3, t3, t0);
	fb_add(t4, p12, p14);
	fb_add(t4, t4, t1);
	fb_add(t5, p9, p11);
	fb_add(t5, t5, t2);
	fb_add(t6, p4, t4);
	fb_add(t7, p1, p3);
	fb_add(t7, t7, p13);
	fb_add(t7, t7, p14);
	fb_add(t7, t7, t3);

	fb_add(r[0], r[0], p3);
	fb_add(r[0], r[0], t0);
	fb_add(r[0], r[0], t5);
	fb_add(r[0], r[0], t6);
	fb_add(r[1], r[1], p11);
	fb_add(r[1], r[1], t7);
	fb_add(r[2], r[2], p2);
	fb_add(r[2], r[2], p9);
	fb_add(r[2], r[2], t3);
	fb_add(r[3], r[3], t1);
	fb_add(r[3], r[3], t5);
	fb_add(r[4], r[4], p10);
	fb_add(r[4], r[4], t4);
	fb_add(r[4], r[4], t7);
	fb_add(r[5], r[5], t2);
	fb_add(r[5], r[5], t6);
	fb_mul(t, f0[0][3], u0);
	fb_add(r[6], r[6], t);
	fb_mul(r[7], f0[0][3], f1[0][4]);
	fb_add(t, f1[0][1], f1[0][4]);
	fb_mul(r[8], f0[0][3], t);
	fb_zero(r[9]);
	fb_mul(r[10], f0[0][3], u3);
	fb_zero(r[11]);

	fb12_zero(l);
	for (int i = 0; i < 6; i++) {
		fb_add(l[0][i], l[0][i], r[i]);
	}
	for (int i = 0; i < 6; i++) {
		fb_add(l[1][i], l[1][i], r[i+6]);
	}
}

void pb_map_res(fb12_t l, fb12_t f1, fb12_t f0, hb_t q) {
	fb_t s0[10];
	fb_t s1[7], t, t0, t1, t2, t3, t4, t5, t6, t7, r[12];
	fb_t u0, u1, u2, u3, u4, u5, u6, u7, u8, u9, u10, u11, u12, u13, u14;
	fb_t v0, v1, v2, v3, v4, v5, v6, v7, v8, v9, v10, v11, v12, v13, v14;
	fb_t p0, p1, p2, p3, p4, p5, p6, p7, p8, p9, p10, p11, p12, p13, p14;
	fb_t w0, w1, z6, w3;

	for (int i = 0; i < 6; i++) {
		fb_sqr(s0[i], f0[0][i]);
		fb_sqr(s1[i], f1[0][i]);
		fb_mul(f1[0][i], f1[0][i], q->u1);
	}
	fb_sqr(s0[6], f0[1][0]);
	fb_sqr(s0[7], f0[1][1]);
	fb_sqr(s0[8], f0[1][2]);
	fb_sqr(s1[6], f1[1][0]);
	fb_mul(f1[1][0], f1[1][0], q->u1);

	fb_add(t0, s0[2], s0[3]);
	fb_add(t0, t0, s0[4]);
	fb_add(t1, s0[2], s0[5]);
	fb_add(r[0], s0[0], t0);
	fb_add(r[1], s0[7], t1);
	fb_add(r[2], s0[6], t0);
	fb_add(r[3], s0[3], s0[5]);
	fb_add(r[3], r[3], s0[6]);
	fb_add(r[4], s0[1], s0[6]);
	fb_add(r[4], r[4], t1);
	fb_add(r[5], s0[5], s0[8]);
	fb_add(r[6], s0[6], s0[8]);
	fb_copy(r[7], s0[8]);
	fb_copy(r[8], s0[8]);
	fb_zero(r[9]);
	fb_add(r[10], s0[7], s0[8]);

	fb_add_dig(u0, q->u0, 1);
	fb_add_dig(u1, q->u1, 1);
	fb_add(u2, q->u0, q->u1);
	fb_add_dig(u3, u2, 1);

	fb_add(t0, s1[2], s1[5]);
	fb_add(t1, s1[0], s1[1]);
	fb_add(t1, t1, t0);
	fb_add(t2, s1[4], t1);
	fb_add(t3, s1[0], s1[3]);
	fb_add(t4, s1[0], s1[6]);

	fb_add(v0, s1[2], s1[4]);
	fb_add(v0, v0, t3);
	fb_copy(v1, t0);
	fb_add(v3, t1, t4);
	fb_copy(v4, s1[5]);
	fb_copy(v6, t1);
	fb_add(v7, s1[3], s1[6]);
	fb_add(v7, v7, t0);
	fb_add(v9, s1[2], t2);
	fb_add(v10, t2, t4);
	fb_add(v11, s1[2], t4);
	fb_copy(v12, t3);
	fb_add(v13, t2, t3);
	fb_copy(v14, t2);

	fb_mul(p0, q->u0, v0);
	fb_mul(p1, q->u0, v1);
	fb_copy(p3, v3);
	fb_copy(p4, v4);
	fb_mul(p6, u3, v6);
	fb_mul(p7, u3, v7);
	fb_mul(p9, u0, v9);
	fb_mul(p10, u1, v10);
	fb_mul(p11, u2, v11);
	fb_mul(p12, u2, v12);
	fb_mul(p13, u1, v13);
	fb_mul(p14, u0, v14);
	fb_add(t0, p3, p7);
	fb_add(t1, p9, p11);
	fb_add(t2, p10, p11);
	fb_add(t3, p0, p14);
	fb_add(t4, p1, p12);
	fb_add(t5, p4, t3);
	fb_add(t5, t5, t4);
	fb_add(t6, p6, p13);
	fb_add(t6, t6, t0);
	fb_add(t7, p1, p6);
	fb_add(t7, t7, t1);
	fb_add(r[0], r[0], t0);
	fb_add(r[0], r[0], t1);
	fb_add(r[0], r[0], t5);
	fb_add(r[1], r[1], t2);
	fb_add(r[1], r[1], t3);
	fb_add(r[1], r[1], t6);
	fb_add(r[2], r[2], p7);
	fb_add(r[2], r[2], t2);
	fb_add(r[2], r[2], t7);
	fb_add(r[3], r[3], p0);
	fb_add(r[3], r[3], t7);
	fb_add(r[4], r[4], t4);
	fb_add(r[4], r[4], t6);
	fb_add(r[5], r[5], p6);
	fb_add(r[5], r[5], t5);
	fb_mul(t, s1[6], q->u0);
	fb_add(r[6], r[6], t);
	fb_mul(t, s1[6], q->u1);
	fb_add(r[8], r[8], t);
	fb_add(r[10], r[10], s1[6]);

	fb_add(t0, f1[0][0], f1[0][1]);
	fb_add(t1, f1[0][4], f1[0][5]);
	fb_add(t2, f1[0][2], f1[0][4]);
	fb_add(t2, t2, t0);
	fb_add(t3, f1[0][3], f1[0][4]);
	fb_add(t4, f1[0][3], f1[0][5]);
	fb_add(t5, f1[0][2], t1);
	fb_add(u0, f1[0][0], f1[0][4]);
	fb_add(u1, f1[0][3], t1);
	fb_add(u2, f1[0][0], t4);
	fb_copy(u3, t5);
	fb_copy(u4, t4);
	fb_add(u5, f1[0][2], t3);
	fb_copy(u6, t2);
	fb_copy(u7, t3);
	fb_add(u8, t2, t3);
	fb_add(u9, f1[0][0], f1[0][2]);
	fb_add(u10, f1[0][1], t5);
	fb_add(u11, t0, t1);
	fb_copy(u12, t0);
	fb_add(u13, f1[0][0], t2);
	fb_add(u14, f1[0][1], t2);
	fb_add(t0, f0[0][0], f0[0][1]);
	fb_add(t1, f0[0][4], f0[0][5]);
	fb_add(t2, f0[0][2], f0[0][4]);
	fb_add(t2, t2, t0);
	fb_add(t3, f0[0][3], f0[0][4]);
	fb_add(t4, f0[0][3], f0[0][5]);
	fb_add(t5, f0[0][2], t1);
	fb_add(v0, f0[0][0], f0[0][4]);
	fb_add(v1, f0[0][3], t1);
	fb_add(v2, f0[0][0], t4);
	fb_copy(v3, t5);
	fb_copy(v4, t4);
	fb_add(v5, f0[0][2], t3);
	fb_copy(v6, t2);
	fb_copy(v7, t3);
	fb_add(v8, t2, t3);
	fb_add(v9, f0[0][0], f0[0][2]);
	fb_add(v10, f0[0][1], t5);
	fb_add(v11, t0, t1);
	fb_copy(v12, t0);
	fb_add(v13, f0[0][0], t2);
	fb_add(v14, f0[0][1], t2);

	fb_add(w0, f0[1][0], f0[1][2]);
	fb_add(w1, f0[1][1], f0[1][2]);
	fb_add(z6, f0[1][0], f0[1][1]);
	fb_add(w3, z6, f0[1][2]);

	fb_mul(p0, u0, v0);
	fb_mul(p1, u1, v1);
	fb_mul(p2, u2, v2);
	fb_mul(p3, u3, v3);
	fb_mul(p4, u4, v4);
	fb_mul(p5, u5, v5);
	fb_mul(p6, u6, v6);
	fb_mul(p7, u7, v7);
	fb_mul(p8, u8, v8);
	fb_mul(p9, u9, v9);
	fb_mul(p10, u10, v10);
	fb_mul(p11, u11, v11);
	fb_mul(p12, u12, v12);
	fb_mul(p13, u13, v13);
	fb_mul(p14, u14, v14);

	fb_add(t0, p1, p14);
	fb_add(t1, p2, p12);
	fb_add(t2, p3, p7);
	fb_add(t3, p4, p8);
	fb_add(t4, p5, p6);
	fb_add(t4, t4, t0);
	fb_add(t4, t4, t1);
	fb_add(t5, p0, p13);
	fb_add(t5, t5, t2);
	fb_add(t5, t5, t3);
	fb_add(t6, p2, p7);
	fb_add(t6, t6, p9);

	fb_add(r[0], r[0], p9);
	fb_add(r[0], r[0], p11);
	fb_add(r[0], r[0], t3);
	fb_add(r[0], r[0], t4);

	fb_add(r[1], r[1], p10);
	fb_add(r[1], r[1], p11);
	fb_add(r[1], r[1], t0);
	fb_add(r[1], r[1], t5);
	fb_add(r[2], r[2], p0);
	fb_add(r[2], r[2], p8);
	fb_add(r[2], r[2], p10);
	fb_add(r[2], r[2], t6);
	fb_add(r[3], r[3], p1);
	fb_add(r[3], r[3], p6);
	fb_add(r[3], r[3], p11);
	fb_add(r[3], r[3], t6);
	fb_add(r[4], r[4], t1);
	fb_add(r[4], r[4], t5);
	fb_add(r[5], r[5], t2);
	fb_add(r[5], r[5], t4);

	fb_mul(p0, f0[0][0], f1[1][0]);
	fb_mul(p1, f0[0][1], f1[1][0]);
	fb_mul(p2, f0[0][2], f1[1][0]);
	fb_mul(p3, f0[0][3], f1[1][0]);
	fb_mul(p4, f0[0][4], f1[1][0]);
	fb_mul(p5, f0[0][5], f1[1][0]);
	fb_mul(p6, f0[1][0], f1[1][0]);
	fb_mul(p7, f0[1][1], f1[1][0]);
	fb_mul(p8, f0[1][2], f1[1][0]);

	fb_add(t0, p4, p5);
	fb_add(t1, p6, p7);
	fb_add(r[1], r[1], p7);
	fb_add(r[1], r[1], p8);
	fb_add(r[2], r[2], p6);
	fb_add(r[3], r[3], t1);
	fb_add(r[4], r[4], t1);
	fb_add(r[6], r[6], p0);
	fb_add(r[6], r[6], p4);
	fb_add(r[6], r[6], p6);
	fb_add(r[7], r[7], p3);
	fb_add(r[7], r[7], t0);
	fb_add(r[8], r[8], p1);
	fb_add(r[8], r[8], p7);
	fb_add(r[8], r[8], t0);
	fb_copy(r[9], p3);
	fb_add(r[10], r[10], p2);
	fb_add(r[10], r[10], p8);
	fb_add(r[10], r[10], t0);
	fb_add(r[11], p3, p5);

	fb_mul(p0, f0[1][0], u0);
	fb_mul(p1, f0[1][0], u1);
	fb_mul(p3, f0[1][2], u3);
	fb_mul(p4, f0[1][2], u4);
	fb_mul(p6, w3, u6);
	fb_mul(p7, w3, u7);
	fb_mul(p9, w0, u9);
	fb_mul(p10, w1, u10);
	fb_mul(p11, z6, u11);
	fb_mul(p12, z6, u12);
	fb_mul(p13, w1, u13);
	fb_mul(p14, w0, u14);

	fb_add(t0, p3, p7);
	fb_add(t1, p9, p11);
	fb_add(t2, p10, p11);
	fb_add(t3, p0, p14);
	fb_add(t4, p1, p12);
	fb_add(t5, p4, t3);
	fb_add(t5, t5, t4);
	fb_add(t6, p6, p13);
	fb_add(t6, t6, t0);
	fb_add(t7, p1, p6);
	fb_add(t7, t7, t1);
	fb_add(r[6], r[6], t0);
	fb_add(r[6], r[6], t1);
	fb_add(r[6], r[6], t5);
	fb_add(r[7], r[7], t2);
	fb_add(r[7], r[7], t3);
	fb_add(r[7], r[7], t6);
	fb_add(r[8], r[8], p7);
	fb_add(r[8], r[8], t2);
	fb_add(r[8], r[8], t7);
	fb_add(r[9], r[9], p0);
	fb_add(r[9], r[9], t7);
	fb_add(r[10], r[10], t4);
	fb_add(r[10], r[10], t6);
	fb_add(r[11], r[11], p6);
	fb_add(r[11], r[11], t5);

	fb12_zero(l);
	for (int i = 0; i < 6; i++) {
		fb_add(l[0][i], l[0][i], r[i]);
	}
	for (int i = 0; i < 6; i++) {
		fb_add(l[1][i], l[1][i], r[i+6]);
	}
}

void pb_map_l4(fb12_t f1, fb12_t f0, fb_t f[], fb_t tab[], hb_t q) {
	fb_t t, t0, t1, t2, t3, t4, t5, t6, t7, t8;

	fb_null(t);
	fb_null(t0);
	fb_null(t1);
	fb_null(t2);
	fb_null(t3);
	fb_null(t4);
	fb_null(t5);
	fb_null(t6);
	fb_null(t7);
	fb_null(t8);

	TRY {
		fb_new(t);
		fb_new(t0);
		fb_new(t1);
		fb_new(t2);
		fb_new(t3);
		fb_new(t4);
		fb_new(t5);
		fb_new(t6);
		fb_new(t7);
		fb_new(t8);

		/* t = f9 * (U1^2 + V1). */
		fb_mul(t, f[9], tab[8]);
		/* t0 = f9 * U0, t1 = f9 * U1. */
		fb_mul(t0, f[9], q->u0);
		fb_mul(t1, f[9], q->u1);
		/* t2 = f9 * U1 + f9 = f9 * (U1 + 1). */
		fb_add(t2, t1, f[9]);
		/* t3 = f9 * (U1^2 + U1 + V1). */
		fb_add(t3, t, t1);
		/* t4 = f9 * (U1^3 + U1^2 + U1). */
		fb_mul(t4, t2, tab[7]);
		fb_add(t4, t4, t1);
		/* t5 = f9 * (U1^2 + V1 + U0 + 1). */
		fb_add(t5, t, t0);
		fb_add(t5, t5, f[9]);
		/* t6 = t3 + t4 + t5. */
		fb_add(t6, t3, t4);
		fb_add(t6, t6, t5);

		/* f0_0,3 = f9 * (U1^2 + U1 + V1) + f8 + f7 + (U1^3 + 1). */
		fb_add(f0[0][3], t3, f[8]);
		fb_add(f0[0][3], f0[0][3], f[7]);
		fb_add(f0[0][3], f0[0][3], tab[1]);
		fb_add_dig(f0[0][3], f0[0][3], 1);
		/* f0_1,0 = f9 * U0 + f7. */
		fb_add(f0[1][0], t0, f[7]);
		/* f0_1,1 = f9 * U1. */
		fb_copy(f0[1][1], t1);
		/* f0_1,2 = f9. */
		fb_copy(f0[1][2], f[9]);
		/* f1_0,3 = f9 * (U1 + 1) + f8. */
		fb_add(f1[0][3], t2, f[8]);
		/* f1_1,0 = f9 * U1 + f8. */
		fb_add(f1[1][0], f0[1][1], f[8]);

		/* t0 = f8 * U0, t1 = f8 * V1, t = f8 * U1. */
		fb_mul(t0, f[8], q->u0);
		fb_mul(t1, f[8], q->v1);
		fb_mul(t, f[8], q->u1);
		/* t3 = f8 * (U1 + 1). */
		fb_add(t3, t, f[8]);
		/* f1_0,5 = f9 * (U1 + 1) + f8 * (U1 + 1). */
		fb_add(f1[0][5], t2, t3);
		/* t2 = f8 * (U1^2 + 1). */
		fb_mul(t2, t, q->u1);
		fb_add(t2, t2, f[8]);
		/* f0_0,5 = t4 + f8 * (U1^2 + U1). */
		fb_add(f0[0][5], t4, t2);
		/* f1_0,2 = t5 + f8 * U1. */
		fb_add(f1[0][2], t5, t);

		/* t4 = f8 * (U0 + V0). */
		fb_mul(t4, f[8], q->v0);
		fb_add(t4, t4, t0);
		/* t5 = f8 * (U1 + V1). */
		fb_add(t5, t, t1);
		fb_add(t7, t0, t1);
		fb_add(t7, t7, f[8]);
		fb_add(t8, t3, t2);
		fb_add(t8, t8, t0);

		/* t0 = f5 * U1^3 + f3 * U1. */
		fb_mul(t0, f[5], tab[1]);
		fb_mul(t, f[3], q->u1);
		fb_add(t0, t0, t);
		/* t1 = f5 * U1^3 + f3 * U1 + f2. */
		fb_add(t1, t0, f[2]);

		/* f1_0,1 = f5 * U1^3 + f3 * U1 + f8 * (U1 + V1). */
		fb_add(f1[0][1], t0, t5);
		/* f1_0,2 = t5 + f8 * U1 + f7 + U1^3 + f3. */
		fb_add(f1[0][2], f1[0][2], f[7]);
		fb_add(f1[0][2], f1[0][2], tab[1]);
		fb_add(f1[0][2], f1[0][2], f[3]);

		/* t = f9 * (U0 * U1 + U1 + V0 + 1). */
		fb_add(t, tab[9], q->v0);
		fb_mul(t, t, f[9]);
		fb_add(f0[0][2], t, t7);
		fb_add(f0[0][2], f0[0][2], tab[2]);
		fb_add(f0[0][2], f0[0][2], t1);
		fb_add_dig(f0[0][2], f0[0][2], 1);

		/* t0 = f7 * (U1 + 1). */
		fb_add_dig(t0, q->u1, 1);
		fb_mul(t0, t0, f[7]);

		/* f0_0,5 = t4 + f8 * (U1^2 + U1) + f7 * (U1 + 1) + U1 + 1. */
		fb_add(f0[0][5], f0[0][5], t0);
		fb_add(f0[0][5], f0[0][5], q->u1);
		fb_add_dig(f0[0][5], f0[0][5], 1);

		fb_add(f1[0][4], t6, t8);
		fb_add(f1[0][4], f1[0][4], t0);
		fb_add(f1[0][4], f1[0][4], q->u1);
		fb_add(f1[0][4], f1[0][4], f[5]);

		/* t =  f9 * (U0 * (U1 * V1 + U1^2 + V0 + 1) + U0^2 + U1 + 1). */
		fb_mul(t, f[9], tab[6]);
		/* t0 = f8 * (U1 * U0 + V1 * U0 + U1 + 1). */
		fb_mul(t0, t5, q->u0);
		fb_add(t0, t0, t3);
		fb_add(f0[0][0], t, t0);
		/* t = f7 * (U0 + V0). */
		fb_add(t, q->u0, q->v0);
		fb_mul(t, t, f[7]);
		fb_add(f0[0][0], f0[0][0], t);
		fb_add(f0[0][0], f0[0][0], tab[4]);
		fb_mul(t, t1, q->u0);
		fb_add(f0[0][0], f0[0][0], t);
		fb_mul(t, f[4], tab[2]);
		fb_add(f0[0][0], f0[0][0], t);
		fb_add(f0[0][0], f0[0][0], f[0]);

		/* t = f9 * (U1 * (U1 * V1 + U1^2 + V0 + 1) + U0 * V1 + 1). */
		fb_mul(t, f[9], tab[5]);
		/* f0_0,1 = t5 * U1 + t + f7 * (U1 + V1) + tab3 + t1 * U1 + f4 * U1^3. */
		fb_mul(f0[0][1], t5, q->u1);
		fb_add(f0[0][1], f0[0][1], t);
		fb_add(t, q->u1, q->v1);
		fb_mul(t, t, f[7]);
		fb_add(f0[0][1], f0[0][1], t);
		fb_add(f0[0][1], f0[0][1], tab[3]);
		fb_mul(t, t1, q->u1);
		fb_add(f0[0][1], f0[0][1], t);
		fb_mul(t, f[4], tab[1]);
		fb_add(f0[0][1], f0[0][1], t);

		/* t = f9 * (U0 * U1 + U1 + 1 + U0 * U1^2 + U0^2). */
		fb_add(t, tab[9], tab[2]);
		fb_mul(t, t, f[9]);
		/* f0_0,4 = f8 * (U0 * U1 + U0 + 1). */
		fb_mul(f0[0][4], t3, q->u0);
		fb_add(f0[0][4], f0[0][4], f[8]);
		/* f0_0,4 += t8 + t + f7 * U0 + U0 + f4. */
		fb_add(f0[0][4], f0[0][4], t);
		fb_mul(t, f[7], q->u0);
		fb_add(f0[0][4], f0[0][4], t);
		fb_add(f0[0][4], f0[0][4], q->u0);
		fb_add(f0[0][4], f0[0][4], f[4]);

		/* f1_0,0 = f0_0,1 + t4 + f5 * tab2 + f3 * U0 + f2. */
		fb_add(f1[0][0], f0[0][1], t4);
		fb_mul(t, f[5], tab[2]);
		fb_add(f1[0][0], f1[0][0], t);
		fb_mul(t, f[3], q->u0);
		fb_add(f1[0][0], f1[0][0], t);
		fb_add(f1[0][0], f1[0][0], f[1]);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(t);
		fb_free(t0);
		fb_free(t1);
		fb_free(t2);
		fb_free(t3);
		fb_free(t4);
		fb_free(t5);
		fb_free(t6);
		fb_free(t7);
		fb_free(t8);
	}
}

void pb_map_l8(fb12_t f1, fb12_t f0, fb_t f[], fb_t tab[], hb_t q) {
	fb_t t, t0, t1, t2, t3, t4, t5, t6, t7, t8, t9, t10;

	fb_null(t);
	fb_null(t0);
	fb_null(t1);
	fb_null(t2);
	fb_null(t3);
	fb_null(t4);
	fb_null(t5);
	fb_null(t6);
	fb_null(t7);
	fb_null(t8);
	fb_null(t9);
	fb_null(t10);

	TRY {
		fb_new(t);
		fb_new(t0);
		fb_new(t1);
		fb_new(t2);
		fb_new(t3);
		fb_new(t4);
		fb_new(t5);
		fb_new(t6);
		fb_new(t7);
		fb_new(t8);
		fb_new(t9);
		fb_new(t10);

		fb_add(t, tab[7], q->v1);
		fb_mul(t, t, f[9]);

		fb_mul(t0, f[9], q->u0);
		fb_mul(t1, f[9], q->u1);
		fb_add(t2, t1, f[9]);
		fb_add(t3, t, t1);
		fb_mul(t4, t2, tab[7]);
		fb_add(t4, t4, t1);
		fb_add(t5, t, t0);
		fb_add(t5, t5, f[9]);
		fb_add(t6, t3, t4);
		fb_add(t6, t6, t5);

		fb_add(f0[1][0], t0, f[7]);
		fb_copy(f0[1][1], t1);
		fb_copy(f0[1][2], f[9]);
		fb_add(f1[1][0], t1, f[8]);
		fb_add(f1[0][3], t2, f[8]);
		fb_add(f0[0][3], t3, f[8]);
		fb_add(f0[0][3], f0[0][3], f[7]);

		fb_mul(t0, f[8], q->u0);
		fb_mul(t1, f[8], q->v1);
		fb_mul(t3, f[8], q->u1);
		fb_add(t, t3, f[8]);

		fb_add(f1[0][5], t2, t);
		fb_add(f1[0][2], t5, t3);
		fb_add(f1[0][2], f1[0][2], f[7]);
		fb_add(f1[0][2], f1[0][2], f[3]);

		fb_mul(t2, t3, q->u1);
		fb_add(t2, t2, f[8]);
		fb_mul(t5, f[8], q->v0);
		fb_add(t5, t5, t0);
		fb_add(t3, t3, t1);
		fb_add(t10, t0, t1);
		fb_add(t10, t10, f[8]);
		fb_add(t7, t, t2);
		fb_add(t7, t7, t0);
		fb_mul(t8, t, q->u0);
		fb_add(t8, t8, f[8]);
		fb_mul(t9, t3, q->u0);
		fb_add(t9, t9, t);

		fb_mul(t0, f[3], q->u1);
		fb_add(t0, t0, tab[1]);
		fb_add(t1, t0, f[2]);

		fb_add(f1[0][1], t3, t0);
		fb_add(t, tab[9], q->v0);
		fb_mul(t, t, f[9]);
		fb_add(f0[0][2], t, t10);
		fb_add(f0[0][2], f0[0][2], t1);

		fb_mul(t, f[9], tab[6]);
		fb_add(f0[0][0], t, t9);
		fb_add(t, q->u0, q->v0);
		fb_mul(t, t, f[7]);
		fb_add(f0[0][0], f0[0][0], t);
		fb_mul(t, t1, q->u0);
		fb_add(f0[0][0], f0[0][0], t);
		fb_mul(t, f[4], tab[2]);
		fb_add(f0[0][0], f0[0][0], t);
		fb_add(f0[0][0], f0[0][0], f[0]);

		fb_mul(f0[0][1], f[9], tab[5]);
		fb_mul(t, t3, q->u1);
		fb_add(f0[0][1], f0[0][1], t);
		fb_add(t, q->u1, q->v1);
		fb_mul(t, t, f[7]);
		fb_add(f0[0][1], f0[0][1], t);
		fb_mul(t, t1, q->u1);
		fb_add(f0[0][1], f0[0][1], t);
		fb_mul(t, f[4], tab[1]);
		fb_add(f0[0][1], f0[0][1], t);

		fb_add(t, tab[9], tab[2]);
		fb_mul(t, t, f[9]);
		fb_add(f0[0][4], t, t8);
		fb_mul(t, f[7], q->u0);
		fb_add(f0[0][4], f0[0][4], t);
		fb_add(f0[0][4], f0[0][4], f[4]);

		fb_add(f0[0][5], t4, t2);
		fb_add_dig(t, q->u1, 1);
		fb_mul(t, t, f[7]);
		fb_add(f0[0][5], f0[0][5], t);
		fb_add(f1[0][4], t6, t7);
		fb_add(f1[0][4], f1[0][4], t);
		fb_add_dig(f1[0][4], f1[0][4], 1);

		fb_add(f1[0][0], f0[0][1], t5);
		fb_add(f1[0][0], f1[0][0], tab[2]);
		fb_mul(t, f[3], q->u0);
		fb_add(f1[0][0], f1[0][0], t);
		fb_add(f1[0][0], f1[0][0], f[1]);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(t);
		fb_free(t0);
		fb_free(t1);
		fb_free(t2);
		fb_free(t3);
		fb_free(t4);
		fb_free(t5);
		fb_free(t6);
		fb_free(t7);
		fb_free(t8);
		fb_free(t9);
		fb_free(t10);
	}
}

void pb_map_add(fb12_t f1, fb12_t f0, fb_t f[], fb_t z[], hb_t q) {
	fb_t t, t0, t1, t2, t3, t4, t5;
	fb_t t7[4];

	fb_mul(t0, f[7], q->u1);
	fb_mul(t1, f[3], q->u0);
	fb_mul(t2, f[3], q->u1);
	fb_add(t3, t2, f[2]);
	fb_mul(t4, t3, q->u0);
	fb_mul(t5, t3, q->u1);

	fb_mul(t7[0], f[7], q->u0);
	fb_add(t7[1], t0, f[7]);
	fb_mul(t, f[7], q->v0);
	fb_add(t7[2], t, t7[0]);
	fb_mul(t, f[7], q->v1);
	fb_add(t7[3], t, t0);

	fb_add(f0[0][0], t7[2], t4);
	fb_add(f0[0][0], f0[0][0], f[0]);
	fb_add(f0[0][1], t7[3], t5);
	fb_copy(f0[0][2], t3);
	fb_copy(f0[0][3], f[7]);
	fb_copy(f0[0][4], t7[0]);
	fb_copy(f0[0][5], t7[1]);

	fb_add(f1[0][0], t7[3], t1);
	fb_add(f1[0][0], f1[0][0], t5);
	fb_add(f1[0][0], f1[0][0], f[1]);
	fb_copy(f1[0][1], t2);
	fb_add(f1[0][2], f[7], f[3]);
	fb_copy(f1[0][4], t7[1]);
}

void pb_map_l2(fb12_t f1, fb12_t f0, fb_t f[], fb_t z[], hb_t q) {
	fb_t t0, t1, t2, t3, t4;
	fb_t t7[4];

	fb_copy(t7[0], q->u0);
	fb_add_dig(t7[1], q->u1, 1);
	fb_add(t7[2], q->u0, q->v0);
	fb_add(t7[3], q->u1, q->v1);
	fb_mul(t0, f[3], q->u0);
	fb_mul(t1, f[3], q->u1);
	fb_add(t2, t1, f[2]);
	fb_mul(t3, t2, q->u0);
	fb_mul(t4, t2, q->u1);

	fb_add(f0[0][0], t7[2], t3);
	fb_add(f0[0][0], f0[0][0], f[0]);
	fb_add(f0[0][1], t7[3], t4);
	fb_copy(f0[0][2], t2);
	fb_copy(f0[0][4], t7[0]);
	fb_copy(f0[0][5], t7[1]);

	fb_add(f1[0][0], t7[3], t0);
	fb_add(f1[0][0], f1[0][0], t4);
	fb_add(f1[0][0], f1[0][0], f[1]);
	fb_copy(f1[0][1], t1);
	fb_add_dig(f1[0][2], f[3], 1);
	fb_copy(f1[0][4], t7[1]);
}

void pb_map_oate2_gxg(fb12_t r, hb_t p, hb_t q) {
	fb12_t l0, l1, f4, f8;
	fb_t t, t0, t1, t2, t3, t4, t5, t6, t7, t8, t9, t10, t11, t12, t13, t14, t15;
	fb_t tab[10], f[10];
	fb_t z0, z1, z2, z3, z4, z5, z6, z7, z8;
	fb_t u1, u0, v1, v0;
	fb12_t r0, r1;
	int i;

	/* Let p = (u1,u0,v1,v0) and q = (U1,U0,V1,V0). */
	fb_copy(u1, p->u1);
	fb_copy(u0, p->u0);
	fb_copy(v1, p->v1);
	fb_copy(v0, p->v0);

	/* t0 = U0^2. */
	fb_sqr(t0, q->u0);
	/* t1 = U1^2. */
	fb_sqr(t1, q->u1);
	/* t2 = U1^4. */
	fb_sqr(t2, t1);
	/* t3 = U1 * V1 + U1^2 + V0 + 1. */
	fb_mul(t3, q->u1, q->v1);
	fb_add(t3, t3, t1);
	fb_add(t3, t3, q->v0);
	fb_add_dig(t3, t3, 1);

	/* tab0 = U0 * U1. */
	fb_mul(tab[0], q->u0, q->u1);
	/* tab1 = U1^3. */
	fb_mul(tab[1], q->u1, t1);
	/* tab2 = U0 * U1^2 + U0^2. */
	fb_mul(tab[2], q->u0, t1);
	fb_add(tab[2], tab[2], t0);
	/* tab3 = U1 * U0^2 + U1^5. */
	fb_add(tab[3], t0, t2);
	fb_mul(tab[3], tab[3], q->u1);
	/* tab4 = (U0 * U1^2 + U0^2 + U1^4) * U0 + 1. */
	fb_add(tab[4], tab[2], t2);
	fb_mul(tab[4], tab[4], q->u0);
	fb_add_dig(tab[4], tab[4], 1);
	/* tab5 = U1 * (U1 * V1 + U1^2 + V0 + 1) + U0 * V1 + 1. */
	fb_mul(tab[5], q->u1, t3);
	fb_mul(t4, q->u0, q->v1);
	fb_add(tab[5], tab[5], t4);
	fb_add_dig(tab[5], tab[5], 1);
	/* tab6 = U0 * (U1 * V1 + U1^2 + V0 + 1) + U0^2 + U1 + 1. */
	fb_mul(tab[6], q->u0, t3);
	fb_add(tab[6], tab[6], t0);
	fb_add(tab[6], tab[6], q->u1);
	fb_add_dig(tab[6], tab[6], 1);
	/* tab7 = U1^2. */
	fb_sqr(tab[7], q->u1);
	/* tab8 = U1^2 + V1. */
	fb_add(tab[8], tab[7], q->v1);
	/* tab9 = U0 * U1 + U1 + 1. */
	fb_add(tab[9], tab[0], q->u1);
	fb_add_dig(tab[9], tab[9], 1);

	/* j = 1, mu(i) = 1, vu(i) = 1. */
	fb_sqr(u1, u1);
	fb_sqr(u0, u0);
	fb_sqr(v1, v1);
	fb_sqr(v0, v0);
	fb_copy(t1, u1);
	fb_copy(t0, u0);

	/* z0 = u0 * u1, z1 = u1 * v0, z2 = u1 * v1. */
	fb_mul(z0, u0, u1);
	fb_mul(z1, u1, v0);
	fb_mul(z2, u1, v1);
	/* z6 = u0 * u1 * v1 + u1^2 * v0. */
	fb_mul(z6, z0, v1);
	fb_mul(t3, z1, u1);
	fb_add(z6, z6, t3);
	/* z7 = u1 * v0 * v1 + u0 * v1^2 + v0^2. */
	fb_mul(z7, z1, v1);
	fb_sqr(u1, u1);
	fb_sqr(u0, u0);
	fb_sqr(v1, v1);
	fb_sqr(v0, v0);
	fb_mul(t3, t0, v1);
	fb_add(z7, z7, t3);
	fb_add(z7, z7, v0);
	/* t2 = u1^2 + 1, t3 = u0 * u1 + u1. */
	fb_add_dig(t2, u1, 1);
	fb_add(t3, z0, t1);
	/* t4 = u1 * v1 + u1^2 + 1, t5 = u1 * v1 + u0 * u1 + u1. */
	fb_add(t4, z2, t2);
	fb_add(t5, z2, t3);
	/* f0 = u1 * v0 * v1 + u0 * v1^2 + v0^2 + u0 * u1 * v1 + u1^2 * v0 +
	 * u0^2 + u1 * v1 + u1^2 + 1. */
	fb_add(f[0], z6, z7);
	fb_add(f[0], f[0], u0);
	fb_add(f[0], f[0], t4);
	/* f1 = u1 * v0 + u1^2 + u1 * v1 + u0 * u1 + u1. */
	fb_add(f[1], z1, u1);
	fb_add(f[1], f[1], t5);
	/* f2 = u0 * u1 + u1 * v0 + u0 * u1 * v1 + u1^2 * v0 + u0 + u1^2 + 1. */
	fb_add(f[2], z0, z1);
	fb_add(f[2], f[2], z6);
	fb_add(f[2], f[2], t0);
	fb_add(f[2], f[2], t2);
	/* f3 = u1 * v1 + u0 * u1 + u1 + 1. */
	fb_add_dig(f[3], t5, 1);
	/* f4 = u0^2 + u0 + u0 * u1 + u1. */
	fb_add(f[4], u0, t0);
	fb_add(f[4], f[4], t3);
	/* f5 = u1^2 + 1 + u1. */
	fb_add(f[5], t1, t2);
	/* f6 = 1, f7 = u1 * v1 + u1^2 + 1, f8 = u1, f9 = u1^2 + u1. */
	fb_set_dig(f[6], 1);
	fb_copy(f[7], t4);
	fb_copy(f[8], t1);
	fb_add(f[9], u1, t1);

	/* l1,l0 = l_4,R2(x,y). */
	pb_map_l4(l1, l0, f, tab, q);
	/* r0 = resultant^2. */
	pb_map_res(r0, l1, l0, q);
	fb12_sqr(r0, r0);

	/* Difference between l_4,R1(x,y) and l_4,R2(x,y) is one squaring, so
	 * square everything. */
	fb_copy(t0, u0);
	fb_copy(t1, u1);
	fb_sqr(u1, u1);
	fb_sqr(u0, u0);
	fb_sqr(z0, z0);
	fb_sqr(z1, z1);
	fb_sqr(z2, z2);
	fb_sqr(z6, z6);
	fb_sqr(z7, z7);

	/* t2 = z2 + 1. */
	fb_add_dig(t2, z2, 1);
	/* t3 = u1^2 + u1. */
	fb_add(t3, u1, t1);
	/* f0 = z7, f1 = z1, f2 = z1 + z6 + t0, f3 = z0 + t2. */
	fb_copy(f[0], z7);
	fb_copy(f[1], z1);
	fb_add(f[2], z1, z6);
	fb_add(f[2], f[2], t0);
	fb_add(f[3], z0, t2);
	fb_add(f[4], z0, u0);
	/* f4 = t0 + t1, f5 = t3 + 1, f6 = 1, f7 = t2, f8 = t1, f9 = t3. */
	fb_add(f[4], f[4], t0);
	fb_add(f[4], f[4], t1);
	fb_add_dig(f[5], t3, 1);
	fb_set_dig(f[6], 1);
	fb_copy(f[7], t2);
	fb_copy(f[8], t1);
	fb_copy(f[9], t3);

	/* l1,l0 = l_4,R1(x,y). */
	pb_map_l4(l1, l0, f, tab, q);
	/* r1 = resultant^2. */
	pb_map_res(r1, l1, l0, q);
	fb12_sqr(r1, r1);

	/* Difference between l_8,R2(x,y) and l_4,R1(x,y) is one squaring, so
	 * square everything. */
	fb_copy(t0, u0);
	fb_copy(t1, u1);
	fb_sqr(u1, u1);
	fb_sqr(u0, u0);
	fb_sqr(z0, z0);
	fb_sqr(z1, z1);
	fb_sqr(z2, z2);
	fb_sqr(z6, z6);
	fb_sqr(z7, z7);

	fb_add(t2, u0, z1);
	fb_mul(z3, t2, t0);
	fb_mul(t2, z6, t1);
	fb_add(z3, z3, t2);
	fb_add(z3, z3, z7);
	fb_mul(z4, z0, t0);
	fb_add(z4, z4, z6);
	fb_mul(z5, z0, t1);
	fb_add(z5, z5, z1);
	fb_mul(z8, u1, t1);
	fb_add(z8, z8, z2);
	fb_add_dig(t2, z0, 1);
	fb_copy(f[0], z3);
	fb_add(f[1], z4, z5);
	fb_add(f[2], z0, z4);
	fb_add(f[2], f[2], u0);
	fb_add(f[2], f[2], t0);
	fb_copy(f[3], t2);
	fb_copy(f[4], u0);
	fb_set_dig(f[5], 1);
	fb_set_dig(f[6], 0);
	fb_add(f[7], z8, t2);
	fb_add(f[8], u1, t1);
	fb_copy(f[9], u1);

	/* l1,l0 = l_8,R2(x,y). */
	pb_map_l8(l1, l0, f, tab, q);
	/* r0 = l_4,R2(x,y)^2 * l_8,R2(x,y). */
	pb_map_res(f4, l1, l0, q);
	fb12_mul(r0, r0, f4);

	fb_copy(t0, u0);
	fb_copy(t1, u1);
	fb_sqr(u1, u1);
	fb_sqr(u0, u0);
	fb_sqr(z0, z0);
	fb_sqr(z3, z3);
	fb_sqr(z4, z4);
	fb_sqr(z5, z5);
	fb_sqr(z8, z8);
	fb_add(t2, u0, u1);
	fb_add(t3, u1, t1);
	fb_add(t4, z0, z8);
	fb_add(t4, t4, t1);
	fb_add_dig(t4, t4, 1);
	fb_add(t5, t0, t4);
	fb_add(t6, z0, t3);
	fb_add_dig(t7, t2, 1);

	fb_add(f[0], z3, z5);
	fb_add(f[0], f[0], t5);
	fb_add(f[1], z4, z5);
	fb_add(f[1], f[1], t6);
	fb_add(f[2], z4, t5);
	fb_add(f[2], f[2], t7);
	fb_add_dig(f[3], t6, 1);
	fb_copy(f[4], t7);
	fb_set_dig(f[5], 1);
	fb_set_dig(f[6], 0);
	fb_copy(f[7], t4);
	fb_copy(f[8], t3);
	fb_copy(f[9], u1);

	/* l1,l0 = l_8,R1(x,y). */
	pb_map_l8(l1, l0, f, tab, q);
	/* r1 = l_4,R1(x,y)^2 * l_8,R1(x,y). */
	pb_map_res(f8, l1, l0, q);
	fb12_mul(r1, r1, f8);

	/* u0 = u0^2^6, u1 = u1^2^6. */
	fb_sqr(u0, u0);
	fb_sqr(u1, u1);
	for (int i = 0; i < 4; i++) {
		fb_sqr(v0, v0);
		fb_sqr(v1, v1);
	}
	for (int i = 0; i < 2; i++) {
		fb_sqr(z0, z0);
		fb_sqr(z3, z3);
		fb_sqr(z4, z4);
		fb_sqr(z5, z5);
		fb_sqr(z8, z8);
	}
	for (int i = 0; i < 3; i++) {
		fb_sqr(z1, z1);
		fb_sqr(z2, z2);
		fb_sqr(z7, z7);
		fb_sqr(z6, z6);
	}

	for (int j = 1; j <= ((FB_BITS - 1)/6 - 1)/4; j++) {
		/* r0 = r0^8, r1 = r1^8. */
		for (int i = 0; i < 3; i++) {
			fb12_sqr(r0, r0);
			fb12_sqr(r1, r1);
		}
		/* j = 2 mod 4, mu(i) = 0, vu(i) = 1. */
		fb_sqr(u0, u0);
		fb_sqr(u1, u1);
		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z1, z1);
		fb_sqr(z2, z2);
		fb_sqr(z6, z6);
		fb_sqr(z7, z7);
		fb_add_dig(t2, z2, 1);
		fb_add(t3, u1, t1);
		fb_add(f[0], t2, z7);
		fb_add(f[1], z1, t1);
		fb_add(f[2], z1, z6);
		fb_add(f[2], f[2], t0);
		fb_add(f[2], f[2], t3);
		fb_add(f[3], z0, t2);
		fb_add(f[4], u0, z0);
		fb_add(f[4], f[4], t0);
		fb_add(f[4], f[4], t1);
		fb_add_dig(f[5], t3, 1);
		fb_set_dig(f[6], 1);
		fb_copy(f[7], t2);
		fb_copy(f[8], t1);
		fb_copy(f[9], t3);

		pb_map_l4(l1, l0, f, tab, q);
		pb_map_res(f4, l1, l0, q);
		fb12_sqr(f4, f4);
		fb12_mul(r0, r0, f4);

		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z1, z1);
		fb_sqr(z2, z2);
		fb_sqr(z6, z6);
		fb_sqr(z7, z7);
		fb_add_dig(t2, u1, 1);
		fb_add(t3, z0, t1);
		fb_add(t4, z2, t2);
		fb_add(t5, z2, t3);
		fb_add(f[0], z6, z7);
		fb_add(f[0], f[0], u0);
		fb_add(f[0], f[0], t4);
		/* f1 = u1 * v0 + u1^2 + u1 * v1 + u0 * u1 + u1. */
		fb_add(f[1], z1, u1);
		fb_add(f[1], f[1], t5);
		/* f2 = u0 * u1 + u1 * v0 + u0 * u1 * v1 + u1^2 * v0 + u0 + u1^2 + 1. */
		fb_add(f[2], z0, z1);
		fb_add(f[2], f[2], z6);
		fb_add(f[2], f[2], t0);
		fb_add(f[2], f[2], t2);
		/* f3 = u1 * v1 + u0 * u1 + u1 + 1. */
		fb_add_dig(f[3], t5, 1);
		/* f4 = u0^2 + u0 + u0 * u1 + u1. */
		fb_add(f[4], u0, t0);
		fb_add(f[4], f[4], t3);
		/* f5 = u1^2 + 1 + u1. */
		fb_add(f[5], t1, t2);
		/* f6 = 1, f7 = u1 * v1 + u1^2 + 1, f8 = u1, f9 = u1^2 + u1. */
		fb_set_dig(f[6], 1);
		fb_copy(f[7], t4);
		fb_copy(f[8], t1);
		fb_add(f[9], u1, t1);

		pb_map_l4(l1, l0, f, tab, q);
		pb_map_res(f4, l1, l0, q);
		fb12_sqr(f4, f4);
		fb12_mul(r1, r1, f4);

		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z3, z3);
		fb_sqr(z3, z3);
		fb_sqr(z3, z3);
		fb_sqr(z4, z4);
		fb_sqr(z4, z4);
		fb_sqr(z4, z4);
		fb_sqr(z5, z5);
		fb_sqr(z5, z5);
		fb_sqr(z5, z5);
		fb_sqr(z8, z8);
		fb_sqr(z8, z8);
		fb_sqr(z8, z8);
		fb_add(t2, z0, t1);
		fb_add(t3, z8, t2);
		fb_add_dig(t4, u1, 1);

		fb_add(f[0], z3, z5);
		fb_add(f[0], f[0], t0);
		fb_add(f[1], z4, z5);
		fb_add(f[1], f[1], z0);
		fb_add(f[2], z4, u0);
		fb_add(f[2], f[2], t0);
		fb_add(f[2], f[2], t3);
		fb_add(f[3], t2, t4);
		fb_add(f[4], u0, t4);
		fb_set_dig(f[5], 1);
		fb_set_dig(f[6], 0);
		fb_add_dig(f[7], t3, 1);
		fb_add(f[8], u1, t1);
		fb_copy(f[9], u1);

		pb_map_l8(l1, l0, f, tab, q);
		pb_map_res(f8, l1, l0, q);
		fb12_mul(r0, r0, f8);

		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z3, z3);
		fb_sqr(z4, z4);
		fb_sqr(z5, z5);
		fb_sqr(z8, z8);
		fb_add_dig(t2, z0, 1);

		fb_copy(f[0], z3);
		fb_add(f[1], z4, z5);
		fb_add(f[2], z0, z4);
		fb_add(f[2], f[2], u0);
		fb_add(f[2], f[2], t0);
		fb_copy(f[3], t2);
		fb_copy(f[4], u0);
		fb_set_dig(f[5], 1);
		fb_set_dig(f[6], 0);
		fb_add(f[7], z8, t2);
		fb_add(f[8], u1, t1);
		fb_copy(f[9], u1);

		pb_map_l8(l1, l0, f, tab, q);
		pb_map_res(f8, l1, l0, q);
		fb12_mul(r1, r1, f8);

		/* r0 = r0^8, r1 = r1^8. */
		for (int i = 0; i < 3; i++) {
			fb12_sqr(r0, r0);
			fb12_sqr(r1, r1);
		}

		/* j = 3 mod 4, mu(i) = 1, vu(i) = 0. */
		fb_sqr(u0, u0);
		fb_sqr(u0, u0);
		fb_sqr(u1, u1);
		fb_sqr(u1, u1);
		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z0, z0);
		fb_sqr(z0, z0);
		for (int i = 0; i < 5; i++) {
			fb_sqr(z1, z1);
			fb_sqr(z2, z2);
			fb_sqr(z6, z6);
			fb_sqr(z7, z7);
		}
		fb_add(t2, z0, t1);
		fb_add(t3, z2, u1);
		fb_add(t4, t0, t2);
		fb_add(t5, u1, t1);

		fb_add(f[0], z6, z7);
		fb_add(f[0], f[0], u0);
		fb_add(f[1], z0, z1);
		fb_add(f[1], f[1], t3);
		fb_add(f[2], z1, z6);
		fb_add(f[2], f[2], t4);
		fb_add_dig(f[2], f[2], 1);
		fb_add(f[3], z2, t2);
		fb_add_dig(f[3], f[3], 1);
		fb_add(f[4], u0, t4);
		fb_add_dig(f[5], t5, 1);
		fb_set_dig(f[6], 1);
		fb_add_dig(f[7], t3, 1);
		fb_copy(f[8], t1);
		fb_copy(f[9], t5);

		pb_map_l4(l1, l0, f, tab, q);
		pb_map_res(f4, l1, l0, q);
		fb12_sqr(f4, f4);
		fb12_mul(r0, r0, f4);

		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z1, z1);
		fb_sqr(z2, z2);
		fb_sqr(z6, z6);
		fb_sqr(z7, z7);

		fb_add_dig(t2, z2, 1);
		fb_add(t3, u1, t1);
		fb_add(f[0], z7, t2);
		fb_add(f[1], z1, t1);
		fb_add(f[2], z1, z6);
		fb_add(f[2], f[2], t0);
		fb_add(f[2], f[2], t3);
		fb_add(f[3], z0, t2);
		fb_add(f[4], z0, u0);
		fb_add(f[4], f[4], t0);
		fb_add(f[4], f[4], t1);
		fb_add_dig(f[5], t3, 1);
		fb_set_dig(f[6], 1);
		fb_copy(f[7], t2);
		fb_copy(f[8], t1);
		fb_copy(f[9], t3);

		pb_map_l4(l1, l0, f, tab, q);
		pb_map_res(f4, l1, l0, q);
		fb12_sqr(f4, f4);
		fb12_mul(r1, r1, f4);

		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		for (int i = 0; i < 5; i++) {
			fb_sqr(z3, z3);
			fb_sqr(z4, z4);
			fb_sqr(z5, z5);
			fb_sqr(z8, z8);
		}
		fb_add_dig(t2, z0, 1);
		fb_add(t3, z8, t2);
		fb_add(t4, z4, u1);

		fb_add(f[0], z3, t3);
		fb_add(f[1], z5, t1);
		fb_add(f[1], f[1], t4);
		fb_add(f[2], z0, u0);
		fb_add(f[2], f[2], t0);
		fb_add(f[2], f[2], t4);
		fb_copy(f[3], t2);
		fb_copy(f[4], u0);
		fb_set_dig(f[5], 1);
		fb_set_dig(f[6], 0);
		fb_copy(f[7], t3);
		fb_add(f[8], u1, t1);
		fb_copy(f[9], u1);

		pb_map_l8(l1, l0, f, tab, q);
		pb_map_res(f8, l1, l0, q);
		fb12_mul(r0, r0, f8);

		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z3, z3);
		fb_sqr(z4, z4);
		fb_sqr(z5, z5);
		fb_sqr(z8, z8);
		fb_add(t2, z0, t1);
		fb_add(t3, z8, t2);
		fb_add_dig(t4, u1, 1);

		fb_add(f[0], z3, z5);
		fb_add(f[0], f[0], t0);
		fb_add(f[1], z4, z5);
		fb_add(f[1], f[1], z0);
		fb_add(f[2], z4, u0);
		fb_add(f[2], f[2], t0);
		fb_add(f[2], f[2], t3);
		fb_add(f[3], t2, t4);
		fb_add(f[4], u0, t4);
		fb_set_dig(f[5], 1);
		fb_set_dig(f[6], 0);
		fb_add_dig(f[7], t3, 1);
		fb_add(f[8], u1, t1);
		fb_copy(f[9], u1);

		pb_map_l8(l1, l0, f, tab, q);
		pb_map_res(f8, l1, l0, q);
		fb12_mul(r1, r1, f8);

		/* r0 = r0^8, r1 = r1^8. */
		for (int i = 0; i < 3; i++) {
			fb12_sqr(r0, r0);
			fb12_sqr(r1, r1);
		}

		/* j = 0 mod 4, mu(i) = 0, vu(i) = 0. */
		fb_sqr(u0, u0);
		fb_sqr(u0, u0);
		fb_sqr(u1, u1);
		fb_sqr(u1, u1);
		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z0, z0);
		fb_sqr(z0, z0);
		for (int i = 0; i < 5; i++) {
			fb_sqr(z1, z1);
			fb_sqr(z2, z2);
			fb_sqr(z6, z6);
			fb_sqr(z7, z7);
		}
		fb_add_dig(t2, z2, 1);
		fb_add(t3, u1, t1);

		fb_copy(f[0], z7);
		fb_copy(f[1], z1);
		fb_add(f[2], z1, z6);
		fb_add(f[2], f[2], t0);
		fb_add(f[3], z0, t2);
		fb_add(f[4], z0, u0);
		fb_add(f[4], f[4], t0);
		fb_add(f[4], f[4], t1);
		fb_add_dig(f[5], t3, 1);
		fb_set_dig(f[6], 1);
		fb_copy(f[7], t2);
		fb_copy(f[8], t1);
		fb_copy(f[9], t3);

		pb_map_l4(l1, l0, f, tab, q);
		pb_map_res(f4, l1, l0, q);
		fb12_sqr(f4, f4);
		fb12_mul(r0, r0, f4);

		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z1, z1);
		fb_sqr(z2, z2);
		fb_sqr(z6, z6);
		fb_sqr(z7, z7);
		fb_add(t2, z0, t1);
		fb_add(t3, z2, u1);
		fb_add(t4, t0, t2);
		fb_add(t5, u1, t1);

		fb_add(f[0], z6, z7);
		fb_add(f[0], f[0], u0);
		fb_add(f[1], z0, z1);
		fb_add(f[1], f[1], t3);
		fb_add(f[2], z1, z6);
		fb_add(f[2], f[2], t4);
		fb_add_dig(f[2], f[2], 1);
		fb_add(f[3], z2, t2);
		fb_add_dig(f[3], f[3], 1);
		fb_add(f[4], u0, t4);
		fb_add_dig(f[5], t5, 1);
		fb_set_dig(f[6], 1);
		fb_add_dig(f[7], t3, 1);
		fb_copy(f[8], t1);
		fb_copy(f[9], t5);

		pb_map_l4(l1, l0, f, tab, q);
		pb_map_res(f4, l1, l0, q);
		fb12_sqr(f4, f4);
		fb12_mul(r1, r1, f4);

		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		for (int i = 0; i < 5; i++) {
			fb_sqr(z3, z3);
			fb_sqr(z4, z4);
			fb_sqr(z5, z5);
			fb_sqr(z8, z8);
		}
		fb_add(t2, u0, u1);
		fb_add(t3, u1, t1);
		fb_add(t4, z0, z8);
		fb_add(t4, t4, t1);
		fb_add_dig(t4, t4, 1);
		fb_add(t5, t0, t4);
		fb_add(t6, z0, t3);
		fb_add_dig(t7, t2, 1);

		fb_add(f[0], z3, z5);
		fb_add(f[0], f[0], t5);
		fb_add(f[1], z4, z5);
		fb_add(f[1], f[1], t6);
		fb_add(f[2], z4, t5);
		fb_add(f[2], f[2], t7);
		fb_add_dig(f[3], t6, 1);
		fb_copy(f[4], t7);
		fb_set_dig(f[5], 1);
		fb_set_dig(f[6], 0);
		fb_copy(f[7], t4);
		fb_copy(f[8], t3);
		fb_copy(f[9], u1);

		pb_map_l8(l1, l0, f, tab, q);
		pb_map_res(f8, l1, l0, q);
		fb12_mul(r0, r0, f8);

		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z3, z3);
		fb_sqr(z4, z4);
		fb_sqr(z5, z5);
		fb_sqr(z8, z8);
		fb_add_dig(t2, z0, 1);
		fb_add(t3, z8, t2);
		fb_add(t4, z4, u1);

		fb_add(f[0], z3, t3);
		fb_add(f[1], z5, t1);
		fb_add(f[1], f[1], t4);
		fb_add(f[2], z0, u0);
		fb_add(f[2], f[2], t0);
		fb_add(f[2], f[2], t4);
		fb_copy(f[3], t2);
		fb_copy(f[4], u0);
		fb_set_dig(f[5], 1);
		fb_set_dig(f[6], 0);
		fb_copy(f[7], t3);
		fb_add(f[8], u1, t1);
		fb_copy(f[9], u1);

		pb_map_l8(l1, l0, f, tab, q);
		pb_map_res(f8, l1, l0, q);
		fb12_mul(r1, r1, f8);

		/* r0 = r0^8, r1 = r1^8. */
		for (int i = 0; i < 3; i++) {
			fb12_sqr(r0, r0);
			fb12_sqr(r1, r1);
		}
		/* j = 1 mod 4, mu(i) = 1, vu(i) = 1. */
		fb_sqr(u0, u0);
		fb_sqr(u0, u0);
		fb_sqr(u1, u1);
		fb_sqr(u1, u1);
		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z0, z0);
		fb_sqr(z0, z0);
		for (int i = 0; i < 5; i++) {
			fb_sqr(z1, z1);
			fb_sqr(z2, z2);
			fb_sqr(z6, z6);
			fb_sqr(z7, z7);
		}
		fb_add_dig(t2, u1, 1);
		fb_add(t3, z0, t1);
		fb_add(t4, z2, t2);
		fb_add(t5, z2, t3);

		fb_add(f[0], z6, z7);
		fb_add(f[0], f[0], u0);
		fb_add(f[0], f[0], t4);
		fb_add(f[1], z1, u1);
		fb_add(f[1], f[1], t5);
		fb_add(f[2], z1, z6);
		fb_add(f[2], f[2], z0);
		fb_add(f[2], f[2], t0);
		fb_add(f[2], f[2], t2);
		fb_add_dig(f[3], t5, 1);
		fb_add(f[4], u0, t0);
		fb_add(f[4], f[4], t3);
		fb_add(f[5], t1, t2);
		fb_set_dig(f[6], 1);
		fb_copy(f[7], t4);
		fb_copy(f[8], t1);
		fb_add(f[9], u1, t1);

		pb_map_l4(l1, l0, f, tab, q);
		pb_map_res(f4, l1, l0, q);
		fb12_sqr(f4, f4);
		fb12_mul(r0, r0, f4);

		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z1, z1);
		fb_sqr(z2, z2);
		fb_sqr(z6, z6);
		fb_sqr(z7, z7);
		fb_add_dig(t2, z2, 1);
		fb_add(t3, u1, t1);
		fb_copy(f[0], z7);
		fb_copy(f[1], z1);
		fb_add(f[2], z1, z6);
		fb_add(f[2], f[2], t0);
		fb_add(f[3], z0, t2);
		fb_add(f[4], u0, z0);
		fb_add(f[4], f[4], t0);
		fb_add(f[4], f[4], t1);
		fb_add_dig(f[5], t3, 1);
		fb_set_dig(f[6], 1);
		fb_copy(f[7], t2);
		fb_copy(f[8], t1);
		fb_copy(f[9], t3);

		pb_map_l4(l1, l0, f, tab, q);
		pb_map_res(f4, l1, l0, q);
		fb12_sqr(f4, f4);
		fb12_mul(r1, r1, f4);

		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		for (int i = 0; i < 5; i++) {
			fb_sqr(z3, z3);
			fb_sqr(z4, z4);
			fb_sqr(z5, z5);
			fb_sqr(z8, z8);
		}
		fb_add_dig(t2, z0, 1);

		fb_copy(f[0], z3);
		fb_add(f[1], z4, z5);
		fb_add(f[2], z0, z4);
		fb_add(f[2], f[2], u0);
		fb_add(f[2], f[2], t0);
		fb_copy(f[3], t2);
		fb_copy(f[4], u0);
		fb_set_dig(f[5], 1);
		fb_set_dig(f[6], 0);
		fb_add(f[7], z8, t2);
		fb_add(f[8], u1, t1);
		fb_copy(f[9], u1);

		pb_map_l8(l1, l0, f, tab, q);
		pb_map_res(f8, l1, l0, q);
		fb12_mul(r0, r0, f8);

		fb_copy(t0, u0);
		fb_copy(t1, u1);
		fb_sqr(u1, u1);
		fb_sqr(u0, u0);
		fb_sqr(z0, z0);
		fb_sqr(z3, z3);
		fb_sqr(z4, z4);
		fb_sqr(z5, z5);
		fb_sqr(z8, z8);
		fb_add(t2, u0, u1);
		fb_add(t3, u1, t1);
		fb_add(t4, z0, z8);
		fb_add(t4, t4, t1);
		fb_add_dig(t4, t4, 1);
		fb_add(t5, t0, t4);
		fb_add(t6, z0, t3);
		fb_add_dig(t7, t2, 1);

		fb_add(f[0], z3, z5);
		fb_add(f[0], f[0], t5);
		fb_add(f[1], z4, z5);
		fb_add(f[1], f[1], t6);
		fb_add(f[2], z4, t5);
		fb_add(f[2], f[2], t7);
		fb_add_dig(f[3], t6, 1);
		fb_copy(f[4], t7);
		fb_set_dig(f[5], 1);
		fb_set_dig(f[6], 0);
		fb_copy(f[7], t4);
		fb_copy(f[8], t3);
		fb_copy(f[9], u1);

		pb_map_l8(l1, l0, f, tab, q);
		pb_map_res(f8, l1, l0, q);
		fb12_mul(r1, r1, f8);

		fb_sqr(u0, u0);
		fb_sqr(u1, u1);
		for (int i = 0; i < 24; i++) {
			fb_sqr(v0, v0);
			fb_sqr(v1, v1);
		}
		for (int i = 0; i < 2; i++) {
			fb_sqr(z0, z0);
			fb_sqr(z3, z3);
			fb_sqr(z4, z4);
			fb_sqr(z5, z5);
			fb_sqr(z8, z8);
		}
		for (int i = 0; i < 4; i++) {
			fb_sqr(z1, z1);
			fb_sqr(z2, z2);
			fb_sqr(z6, z6);
			fb_sqr(z7, z7);
		}
	}

	/* r1 = r1^(m-1)/2. */
	int to = ((FB_BITS - 1) / 2) / 6;
	to = to * 6;
	fb12_copy(l1, r1);
	for (i = 0; i < to; i++) {
		/* This is faster than calling fb12_sqr(alpha, alpha) (no field additions). */
		fb_sqr(l1[0][0], l1[0][0]);
		fb_sqr(l1[0][1], l1[0][1]);
		fb_sqr(l1[0][2], l1[0][2]);
		fb_sqr(l1[0][3], l1[0][3]);
		fb_sqr(l1[0][4], l1[0][4]);
		fb_sqr(l1[0][5], l1[0][5]);
		fb_sqr(l1[1][0], l1[1][0]);
		fb_sqr(l1[1][1], l1[1][1]);
		fb_sqr(l1[1][2], l1[1][2]);
		fb_sqr(l1[1][3], l1[1][3]);
		fb_sqr(l1[1][4], l1[1][4]);
		fb_sqr(l1[1][5], l1[1][5]);
	}
	if ((to / 6) % 2 == 1) {
		fb6_add(l1[0], l1[0], l1[1]);
	}
	for (; i < (FB_BITS - 1) / 2; i++) {
		fb12_sqr(l1, l1);
	}

	fb12_mul(r0, r0, l1);
	fb12_sqr(r0, r0);
	fb12_mul(r0, r0, r1);

	fb_add(tab[2], u0, u1);
	fb_add_dig(tab[2], tab[2], 1);
	fb_copy(tab[3], u1);
	fb_add(tab[4], v0, v1);
	fb_add(tab[4], tab[4], tab[2]);
	fb_add(tab[5], v1, u1);

	fb_sqr(tab[6], u0);
	fb_sqr(tab[7], u1);
	fb_sqr(tab[8], v0);
	fb_sqr(tab[9], v1);

	fb_add(t0, tab[2], tab[6]);
	fb_add(t1, tab[3], tab[7]);
	fb_add(t2, tab[4], tab[8]);
	fb_add(t3, tab[5], tab[9]);
	fb_sqr(t4, tab[2]);
	fb_sqr(t5, tab[6]);
	fb_copy(t6, tab[7]);
	fb_sqr(t7, tab[7]);
	fb_mul(t8, tab[2], tab[6]);
	fb_mul(t9, tab[3], tab[7]);
	fb_add(t10, t7, t9);
	fb_mul(t10, t10, tab[2]);
	fb_add(t11, t6, t9);
	fb_mul(t11, t11, tab[6]);
	fb_add(t12, t4, t8);
	fb_add(t12, t12, t11);
	fb_add(t13, t5, t8);
	fb_add(t13, t13, t10);
	fb_add(t14, z0, u1);
	fb_sqr(t15, t14);
	fb_add(t15, t15, t14);
	fb_mul(f[0], t1, t3);
	fb_add(f[0], f[0], t2);
	fb_mul(f[0], f[0], t8);
	fb_add(t, t5, t11);
	fb_mul(t, t, tab[4]);
	fb_add(f[0], f[0], t);
	fb_add(t, t4, t10);
	fb_mul(t, t, tab[8]);
	fb_add(f[0], f[0], t);
	fb_add(t, t0, t9);
	fb_mul(t, t, t1);
	fb_add(t, t, t15);
	fb_mul(f[1], t2, t);
	fb_mul(t, tab[5], t13);
	fb_add(f[1], f[1], t);
	fb_mul(t, tab[9], t12);
	fb_add(f[1], f[1], t);
	fb_mul(f[2], t3, t15);
	fb_sqr(t, t1);
	fb_add(t, t, t0);
	fb_mul(t, t, t2);
	fb_add(f[2], f[2], t);
	fb_mul(f[3], t3, t0);
	fb_mul(t, t2, t1);
	fb_add(f[3], f[3], t);
	fb_zero(f[4]);
	fb_zero(f[5]);
	fb_zero(f[6]);
	fb_add(f[7], t12, t13);
	fb_zero(f[8]);
	fb_zero(f[9]);

	pb_map_add(l1, l0, f, tab, q);
	pb_map_res2(f8, l1, l0, q);
	fb12_mul(r1, r1, f8);

	fb_add(z0, t9, u1);
	fb_add_dig(z1, u1, 1);
	fb_sqr(z6, z1);
	fb_add(z3, z0, z6);
	fb_sqr(z4, z6);
	fb_add(z5, z0, tab[9]);

	fb_add(t0, z2, v1);
	fb_add_dig(t0, t0, 1);
	fb_mul(tab[1], z6, tab[3]);
	fb_mul(tab[0], z6, tab[2]);
	fb_mul(tab[4], tab[4], z3);
	fb_mul(tab[5], tab[5], z3);
	fb_copy(tab[6], z5);
	fb_mul(t, z3, v0);
	fb_mul(tab[8], z5, t0);
	fb_add(tab[8], tab[8], t);
	fb_mul(tab[9], z3, u0);
	fb_mul(t, z5, z6);
	fb_add(tab[9], tab[9], t);
	fb_add(tab[9], tab[9], t0);
	fb_add(t0, tab[0], tab[6]);
	fb_add_dig(t1, tab[1], 1);
	fb_add(t2, tab[4], tab[8]);
	fb_add(t3, tab[5], tab[9]);
	fb_sqr(t4, tab[0]);
	fb_sqr(t5, tab[6]);
	fb_mul(t6, tab[1], tab[3]);
	fb_mul(t8, tab[0], tab[6]);
	fb_mul(t9, tab[2], tab[6]);
	fb_add_dig(t10, tab[1], 1);
	fb_mul(t10, t10, tab[2]);
	fb_add(t11, tab[3], t6);
	fb_mul(t11, t11, tab[6]);
	fb_add(t12, t4, t8);
	fb_add(t12, t12, t11);
	fb_add(t13, t5, t8);
	fb_add(t13, t13, t10);
	fb_mul(t14, tab[0], tab[1]);
	fb_add(t14, t14, tab[6]);
	fb_mul(t15, z6, t0);

	fb_mul(f[0], t1, t3);
	fb_mul(t, t2, z6);
	fb_add(f[0], f[0], t);
	fb_mul(f[0], f[0], t9);
	fb_add(t, t5, t11);
	fb_mul(t, t, tab[4]);
	fb_add(f[0], f[0], t);
	fb_add(t, t4, t10);
	fb_mul(t, t, tab[8]);
	fb_add(f[0], f[0], t);

	fb_add(t, t0, tab[3]);
	fb_mul(t, t, t1);
	fb_add(t, t, t14);
	fb_mul(f[1], t, t2);
	fb_mul(t, tab[5], t13);
	fb_add(f[1], f[1], t);
	fb_mul(t, tab[9], t12);
	fb_add(f[1], f[1], t);

	fb_sqr(f[2], t1);
	fb_add(f[2], f[2], t15);
	fb_mul(f[2], f[2], t2);
	fb_mul(t, t3, t14);
	fb_add(f[2], f[2], t);
	fb_mul(f[3], t3, t15);
	fb_mul(t, z6, t1);
	fb_mul(t, t, t2);
	fb_add(f[3], f[3], t);
	fb_zero(f[4]);
	fb_zero(f[5]);
	fb_zero(f[6]);
	fb_add(f[7], t12, t13);
	fb_mul(f[7], f[7], z3);
	fb_zero(f[8]);
	fb_zero(f[9]);

	pb_map_add(l1, l0, f, tab, q);
	pb_map_res2(f8, l1, l0, q);
	fb12_mul(r0, r0, f8);

	fb_add_dig(f[0], v0, 1);
	fb_copy(f[1], u0);
	fb_copy(f[2], v1);
	fb_add_dig(f[3], u1, 1);
	fb_zero(f[4]);
	fb_zero(f[5]);
	fb_zero(f[6]);
	fb_set_dig(f[7], 1);
	fb_zero(f[8]);
	fb_zero(f[9]);

	pb_map_l2(l1, l0, f, tab, q);
	pb_map_res3(f8, l1, l0, q);
	fb12_mul(r0, r0, f8);

	fb12_frb(r0, r0);
	fb12_frb(r0, r0);
	fb12_frb(r0, r0);
	fb12_mul(r, r0, r1);
}

/*============================================================================*/
/* Public definitions                                                        */
/*============================================================================*/

void pb_map_oate2(fb12_t r, hb_t p, hb_t q) {
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
			pb_map_oate2_gxg(r, p, q);
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
