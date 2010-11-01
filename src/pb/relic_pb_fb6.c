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
 * Implementation of the sextic extension binary field arithmetic module.
 *
 * @version $Id: relic_pb_fb2.c 88 2009-09-06 21:27:19Z dfaranha $
 * @ingroup pb
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_util.h"
#include "relic_fb.h"
#include "relic_fb_low.h"
#include "relic_pb.h"

/*============================================================================*/
/* Private definitions                                                         */
/*============================================================================*/

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb6_mul(fb6_t c, fb6_t a, fb6_t b) {
	fb_t t[15], p[15], u[15], v[15];
	int i;

	for (i = 0; i < 15; i++) {
		fb_null(t[i]);
		fb_null(p[i]);
		fb_null(u[i]);
		fb_null(v[i]);
	}

	TRY {
		for (i = 0; i < 15; i++) {
			fb_new(t[i]);
			fb_new(p[i]);
			fb_new(u[i]);
			fb_new(v[i]);
		}

		fb_add(t[0], a[2], a[4]);
		fb_add(t[1], a[3], a[5]);
		fb_add(t[2], a[1], t[0]);
		fb_add(t[3], a[0], t[1]);

		fb_copy(u[0], a[0]);
		fb_copy(u[1], a[1]);
		fb_add(u[2], u[0], u[1]);
		fb_copy(u[3], a[4]);
		fb_copy(u[4], a[5]);
		fb_add(u[5], u[3], u[4]);
		fb_add(u[6], a[0], t[0]);
		fb_add(u[7], a[1], t[1]);
		fb_add(u[8], u[6], u[7]);
		fb_add(u[9], a[4], t[3]);
		fb_add(u[10], a[3], t[2]);
		fb_add(u[11], u[9], u[10]);
		fb_add(u[12], a[2], t[3]);
		fb_add(u[13], a[5], t[2]);
		fb_add(u[14], u[12], u[13]);

		fb_add(t[4], b[2], b[4]);
		fb_add(t[5], b[3], b[5]);
		fb_add(t[6], b[1], t[4]);
		fb_add(t[7], b[0], t[5]);

		fb_copy(v[0], b[0]);
		fb_copy(v[1], b[1]);
		fb_add(v[2], v[0], v[1]);
		fb_copy(v[3], b[4]);
		fb_copy(v[4], b[5]);
		fb_add(v[5], v[3], v[4]);
		fb_add(v[6], b[0], t[4]);
		fb_add(v[7], b[1], t[5]);
		fb_add(v[8], v[6], v[7]);
		fb_add(v[9], b[4], t[7]);
		fb_add(v[10], b[3], t[6]);
		fb_add(v[11], v[9], v[10]);
		fb_add(v[12], b[2], t[7]);
		fb_add(v[13], b[5], t[6]);
		fb_add(v[14], v[12], v[13]);

		for (i = 0; i < 15; i++) {
			fb_mul(p[i], u[i], v[i]);
		}

		fb_add(t[8], p[1], p[14]);
		fb_add(t[9], p[2], p[12]);
		fb_add(t[10], p[3], p[7]);
		fb_add(t[11], p[4], p[8]);
		fb_add(t[12], p[5], p[6]);
		fb_add(t[12], t[12], t[8]);
		fb_add(t[12], t[12], t[9]);
		fb_add(t[13], p[0], p[13]);
		fb_add(t[13], t[13], t[10]);
		fb_add(t[13], t[13], t[11]);
		fb_add(t[14], p[2], p[7]);
		fb_add(t[14], t[14], p[9]);

		fb_add(c[0], p[9], p[11]);
		fb_add(c[0], c[0], t[11]);
		fb_add(c[0], c[0], t[12]);
		fb_add(c[1], p[10], p[11]);
		fb_add(c[1], c[1], t[8]);
		fb_add(c[1], c[1], t[13]);
		fb_add(c[2], p[0], p[8]);
		fb_add(c[2], c[2], p[10]);
		fb_add(c[2], c[2], t[14]);
		fb_add(c[3], p[1], p[6]);
		fb_add(c[3], c[3], p[11]);
		fb_add(c[3], c[3], t[14]);
		fb_add(c[4], t[9], t[13]);
		fb_add(c[5], t[10], t[12]);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t2[0]);
		fb_free(t2[1]);
		fb_free(t2[2]);
		fb_free(t2[3]);
		fb_free(t2[4]);
		for (i = 0; i < 3; i++) {
			fb_free(t0[i]);
			fb_free(t1[i]);
		}
		for (i = 0; i < 12; i++) {
			fb_free(t[i]);
		}
	}
}

void fb6_mul_dxs(fb6_t c, fb6_t a, fb6_t b) {
	fb_t t[15], p[15], u[15], v[15];
	int i;

	for (i = 0; i < 15; i++) {
		fb_null(t[i]);
		fb_null(p[i]);
		fb_null(u[i]);
		fb_null(v[i]);
	}

	TRY {
		for (i = 0; i < 15; i++) {
			fb_new(t[i]);
			fb_new(p[i]);
			fb_new(u[i]);
			fb_new(v[i]);
		}

		fb_add(t[0], a[2], a[4]);
		fb_add(t[1], a[3], a[5]);
		fb_add(t[2], a[1], t[0]);
		fb_add(t[3], a[0], t[1]);

		fb_copy(u[0], a[0]);
		fb_copy(u[1], a[1]);
		fb_add(u[2], u[0], u[1]);
		fb_copy(u[3], a[4]);
		fb_copy(u[4], a[5]);
		fb_add(u[5], u[3], u[4]);
		fb_add(u[6], a[0], t[0]);
		fb_add(u[7], a[1], t[1]);
		fb_add(u[8], u[6], u[7]);
		fb_add(u[9], a[4], t[3]);
		fb_add(u[10], a[3], t[2]);
		fb_add(u[11], u[9], u[10]);
		fb_add(u[12], a[2], t[3]);
		fb_add(u[13], a[5], t[2]);
		fb_add(u[14], u[12], u[13]);

		fb_add(t[4], b[2], b[4]);
		fb_add(t[6], b[1], t[4]);

		fb_copy(v[0], b[0]);
		fb_copy(v[1], b[1]);
		fb_add(v[2], v[0], v[1]);
		fb_copy(v[3], b[4]);
		fb_zero(v[4]);
		fb_add(v[5], v[3], v[4]);
		fb_add(v[6], b[0], t[4]);
		fb_copy(v[7], b[1]);
		fb_add(v[8], v[6], v[7]);
		fb_add(v[9], b[4], b[0]);
		fb_copy(v[10], t[6]);
		fb_add(v[11], v[9], v[10]);
		fb_add(v[12], b[2], b[0]);
		fb_copy(v[13], t[6]);
		fb_add(v[14], v[12], v[13]);

		for (i = 0; i < 4; i++) {
			fb_mul(p[i], u[i], v[i]);
		}
		for (i = 5; i < 15; i++) {
			fb_mul(p[i], u[i], v[i]);
		}

		fb_add(t[8], p[1], p[14]);
		fb_add(t[9], p[2], p[12]);
		fb_add(t[10], p[3], p[7]);
		fb_add(t[12], p[5], p[6]);
		fb_add(t[12], t[12], t[8]);
		fb_add(t[12], t[12], t[9]);
		fb_add(t[13], p[0], p[13]);
		fb_add(t[13], t[13], t[10]);
		fb_add(t[13], t[13], p[8]);
		fb_add(t[14], p[2], p[7]);
		fb_add(t[14], t[14], p[9]);

		fb_add(c[0], p[9], p[11]);
		fb_add(c[0], c[0], p[8]);
		fb_add(c[0], c[0], t[12]);
		fb_add(c[1], p[10], p[11]);
		fb_add(c[1], c[1], t[8]);
		fb_add(c[1], c[1], t[13]);
		fb_add(c[2], p[0], p[8]);
		fb_add(c[2], c[2], p[10]);
		fb_add(c[2], c[2], t[14]);
		fb_add(c[3], p[1], p[6]);
		fb_add(c[3], c[3], p[11]);
		fb_add(c[3], c[3], t[14]);
		fb_add(c[4], t[9], t[13]);
		fb_add(c[5], t[10], t[12]);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t2[0]);
		fb_free(t2[1]);
		fb_free(t2[2]);
		fb_free(t2[3]);
		fb_free(t2[4]);
		for (i = 0; i < 3; i++) {
			fb_free(t0[i]);
			fb_free(t1[i]);
		}
		for (i = 0; i < 12; i++) {
			fb_free(t[i]);
		}
	}
}

void fb6_mul_nor(fb6_t c, fb6_t a) {
	fb_t t[12];
	int i;

	for (i = 0; i < 12; i++) {
		fb_null(t[i]);
	}

	TRY {
		fb_add(t[0], a[0], a[2]);
		fb_add(t[1], a[1], a[3]);
		fb_add(t[2], a[3], a[5]);
		fb_add(c[1], a[2], a[4]);
		fb_add(c[1], c[1], t[2]);
		fb_add(c[2], a[0], t[1]);
		fb_add(c[3], a[3], t[0]);
		fb_copy(c[4], t[0]);
		fb_copy(c[5], t[1]);
		fb_copy(c[0], t[2]);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		for (i = 0; i < 12; i++) {
			fb_free(t[i]);
		}
	}
}

void fb6_sqr(fb6_t c, fb6_t a) {
	fb_t t0, t1, t2, t3, t4, t5;

	fb_null(t0);
	fb_null(t1);
	fb_null(t2);
	fb_null(t3);
	fb_null(t4);
	fb_null(t5);

	TRY {
		fb_new(t0);
		fb_new(t1);
		fb_new(t2);
		fb_new(t3);
		fb_new(t4);
		fb_new(t5);

		fb_sqr(t0, a[0]);
		fb_sqr(t1, a[1]);
		fb_sqr(t2, a[2]);
		fb_sqr(t3, a[3]);
		fb_sqr(t4, a[4]);
		fb_sqr(t5, a[5]);

		fb_add(c[5], t4, t5);
		fb_add(c[0], t0, t1);
		fb_add(c[0], c[0], t4);
		fb_add(c[1], t1, c[5]);
		fb_copy(c[2], c[5]);
		fb_copy(c[3], t5);
		fb_add(c[4], t2, t3);
		fb_add(c[4], c[4], c[5]);
		fb_add(c[5], t3, t5);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(t0);
		fb_free(t1);
		fb_free(t2);
		fb_free(t3);
		fb_free(t4);
		fb_free(t5);
	}
}

void fb6_frb(fb6_t c, fb6_t a) {
	fb_t t0, t1;

	fb_null(t0);
	fb_null(t1);

	TRY {
		fb_new(t0);
		fb_new(t1);

		fb_copy(t0, a[1]);
		fb_copy(t1, a[5]);

		fb_add(c[5], a[4], a[5]);
		fb_add(c[0], a[0], a[1]);
		fb_add(c[0], c[0], a[4]);
		fb_add(c[1], a[1], c[5]);
		fb_add(c[4], a[2], a[3]);
		fb_add(c[4], c[4], c[5]);
		fb_copy(c[2], c[5]);
		fb_add(c[5], a[3], t1);
		fb_copy(c[3], t1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(t0);
		fb_free(t1);
	}
}

void fb6_inv(fb6_t c, fb6_t a) {
	fb2_t _a0, _a1, _a2, n0, n1, n2;
	fb2_t t0, t1, t2, t3, t4;
	fb2_t q0, q1, q2, q3, q4;
	fb2_t p0, p1, p2, p3, p4, p5, p6;

	TRY {
		fb_copy(_a0[0], a[0]);
		fb_copy(_a0[1], a[1]);
		fb_copy(_a1[0], a[2]);
		fb_copy(_a1[1], a[3]);
		fb_copy(_a2[0], a[4]);
		fb_copy(_a2[1], a[5]);

		fb2_add(t0, _a0, _a1);
		fb2_add(t1, _a2, t0);
		fb2_sqr(q0, _a0);
		fb2_sqr(q1, _a1);
		fb2_sqr(q2, _a2);
		fb2_sqr(q3, t0);
		fb2_sqr(q4, t1);
		fb2_mul(p0, q0, _a0);
		fb2_mul(p1, q3, t0);
		fb2_mul(p2, q4, t1);
		fb2_mul(p3, _a0, _a1);
		fb2_mul(p4, _a0, _a2);
		fb2_mul(p5, _a1, _a2);
		fb2_mul(p6, p3, _a2);
		fb_add(t1[0], p0[0], p2[0]);

		fb_add(t2[0], p0[1], p1[0]);
		fb_add(t2[0], t2[0], p2[1]);
		fb_add(t2[0], t2[0], p6[0]);
		fb_add(t2[0], t2[0], t1[0]);

		fb_add(t2[1], p1[1], p6[1]);
		fb_add(t2[1], t2[1], t1[0]);

		fb2_inv(t2, t2);

		fb_add(t3[0], p5[0], p5[1]);
		fb_add(t3[0], t3[0], q1[1]);
		fb_add(t3[0], t3[0], q2[0]);
		fb_add(t3[1], p3[1], q1[1]);
		fb_add(t4[0], p4[0], p5[1]);
		fb_add(t4[0], t4[0], q1[0]);
		fb_add(t4[1], p5[0], t3[1]);

		fb_add(n0[0], p4[0], p4[1]);
		fb_add(n0[0], n0[0], q0[0]);
		fb_add(n0[0], n0[0], t4[1]);
		fb_add(n0[1], p3[0], q0[1]);
		fb_add(n0[1], n0[1], t3[1]);
		fb_add(n0[1], n0[1], t4[0]);
		fb_add(n1[0], p3[0], t3[0]);
		fb_add(n1[1], q1[0], q2[1]);
		fb_add(n1[1], n1[1], t4[1]);
		fb_add(n2[0], q2[1], t4[0]);
		fb_add(n2[1], p4[1], q2[1]);
		fb_add(n2[1], n2[1], t3[0]);

		fb2_mul(n0, n0, t2);
		fb2_mul(n1, n1, t2);
		fb2_mul(n2, n2, t2);

		fb_copy(c[0], n0[0]);
		fb_copy(c[1], n0[1]);
		fb_copy(c[2], n1[0]);
		fb_copy(c[3], n1[1]);
		fb_copy(c[4], n2[0]);
		fb_copy(c[5], n2[1]);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
	}
}
