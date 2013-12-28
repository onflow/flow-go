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
 * Implementation of multiplication in quartic extensions of binary fields.
 *
 * The implementations of fb4_mul() and fb4_mul_dxs() are based on:
 *
 * Beuchat et al., A comparison between hardware accelerators for the modified
 * Tate pairing over F_2^m and F_3^m, 2008.
 *
 * The implementation of fb4_mul_dxd() is based on:
 *
 * Shirase et al., Efficient computation of Eta pairing over binary field with
 * Vandermonde matrix, 2008.
 *
 * and optimal 4-way Toom-Cook from:
 *
 * Bodrato, Towards optimal Toom-Cook multiplication for univariate and
 * multivariate polynomials in characteristic 2 and 0, 2007.
 *
 * @version $Id$
 * @ingroup fbx
 */

#include "relic_core.h"
#include "relic_fbx.h"
#include "relic_fb_low.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

void fb_mul_beta(dig_t *c, dig_t *a) {
	int j, k;
	dig_t b1, b2;
	dv_t t;

	dv_null(t);

	TRY {
		dv_new(t);

#if WORD == 8
		fb_lshd_low(t, a, 1);
		t[FB_DIGS] = a[FB_DIGS - 1];
#else
		t[FB_DIGS] = fb_lshb_low(t, a, 8);
#endif

		j = FB_DIGIT - 6;
		b1 = a[0];
		t[0] ^= (b1 << 6);
		for (k = 1; k < FB_DIGS; k++) {
			b2 = a[k];
			t[k] ^= ((b2 << 6) | (b1 >> j));
			b1 = b2;
		}
		t[FB_DIGS] ^= (b1 >> j);

		j = FB_DIGIT - 5;
		b1 = a[0];
		t[0] ^= (b1 << 5);
		for (k = 1; k < FB_DIGS; k++) {
			b2 = a[k];
			t[k] ^= ((b2 << 5) | (b1 >> j));
			b1 = b2;
		}
		t[FB_DIGS] ^= (b1 >> j);

		j = FB_DIGIT - 3;
		b1 = a[0];
		t[0] ^= (b1 << 3);
		for (k = 1; k < FB_DIGS; k++) {
			b2 = a[k];
			t[k] ^= ((b2 << 3) | (b1 >> j));
			b1 = b2;
		}
		t[FB_DIGS] ^= (b1 >> j);

		fb_rdc1_low(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

void fb_mul_21(dig_t *c, dig_t *a) {
	int j, k;
	dig_t b1, b2;
	dv_t t;

	dv_null(t);

	TRY {
		dv_new(t);

		t[FB_DIGS] = fb_lshb_low(t, a, 2);

		j = FB_DIGIT - 1;
		b1 = a[0];
		t[0] ^= (b1 << 1);
		for (k = 1; k < FB_DIGS; k++) {
			b2 = a[k];
			t[k] ^= ((b2 << 1) | (b1 >> j));
			b1 = b2;
		}
		t[FB_DIGS] ^= (b1 >> j);
		fb_rdc1_low(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

void fb_mul_420(dig_t *c, dig_t *a) {
	int j, k;
	dig_t b1, b2;
	dv_t t;

	dv_null(t);

	TRY {
		dv_new(t);

		t[FB_DIGS] = fb_lshb_low(t, a, 4);

		j = FB_DIGIT - 2;
		b1 = a[0];
		t[0] ^= (b1 << 2);
		for (k = 1; k < FB_DIGS; k++) {
			b2 = a[k];
			t[k] ^= ((b2 << 2) | (b1 >> j));
			b1 = b2;
		}
		t[FB_DIGS] ^= (b1 >> j);
		fb_addn_low(t, t, a);
		fb_rdc1_low(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

void fb_mul_41(dig_t *c, dig_t *a) {
	int j, k;
	dig_t b1, b2;
	dv_t t;

	dv_null(t);

	TRY {
		dv_new(t);

		t[FB_DIGS] = fb_lshb_low(t, a, 4);

		j = FB_DIGIT - 1;
		b1 = a[0];
		t[0] ^= (b1 << 1);
		for (k = 1; k < FB_DIGS; k++) {
			b2 = a[k];
			t[k] ^= ((b2 << 1) | (b1 >> j));
			b1 = b2;
		}
		t[FB_DIGS] ^= (b1 >> j);
		fb_rdc1_low(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

void fb_mul_42(dig_t *c, dig_t *a) {
	int j, k;
	dig_t b1, b2;
	dv_t t;

	dv_null(t);

	TRY {
		dv_new(t);

		t[FB_DIGS] = fb_lshb_low(t, a, 4);

		j = FB_DIGIT - 2;
		b1 = a[0];
		t[0] ^= (b1 << 2);
		for (k = 1; k < FB_DIGS; k++) {
			b2 = a[k];
			t[k] ^= ((b2 << 2) | (b1 >> j));
			b1 = b2;
		}
		t[FB_DIGS] ^= (b1 >> j);
		fb_rdc1_low(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

void fb_mul_51(dig_t *c, dig_t *a) {
	int j, k;
	dig_t b1, b2;
	dv_t t;

	dv_null(t);

	TRY {
		dv_new(t);

		t[FB_DIGS] = fb_lshb_low(t, a, 5);

		j = FB_DIGIT - 1;
		b1 = a[0];
		t[0] ^= (b1 << 1);
		for (k = 1; k < FB_DIGS; k++) {
			b2 = a[k];
			t[k] ^= ((b2 << 1) | (b1 >> j));
			b1 = b2;
		}
		t[FB_DIGS] ^= (b1 >> j);
		fb_rdc1_low(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb4_mul(fb4_t c, fb4_t a, fb4_t b) {
	fb_t t0, t1, t2, t3, t4, t5, t6, t7, t8, t9;

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

	TRY {
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

		fb_add(t0, a[0], a[1]);
		fb_add(t1, b[0], b[1]);
		fb_add(t2, a[0], a[2]);
		fb_add(t3, b[0], b[2]);
		fb_add(t4, a[1], a[3]);
		fb_add(t5, b[1], b[3]);
		fb_add(t6, a[2], a[3]);
		fb_add(t7, b[2], b[3]);

		fb_add(t8, t0, t6);
		fb_add(t9, t1, t7);

		fb_mul(t0, t0, t1);
		fb_mul(t2, t2, t3);
		fb_mul(t4, t4, t5);
		fb_mul(t6, t6, t7);
		fb_mul(t8, t8, t9);

		fb_mul(t1, a[0], b[0]);
		fb_mul(t3, a[1], b[1]);
		fb_mul(t5, a[2], b[2]);
		fb_mul(t7, a[3], b[3]);

		fb_add(t3, t1, t3);
		fb_add(t0, t1, t0);

		fb_add(c[0], t3, t5);
		fb_add(c[0], c[0], t6);
		fb_add(c[1], t0, t7);
		fb_add(c[1], c[1], t6);
		fb_add(c[2], t3, t2);
		fb_add(c[2], c[2], t4);
		fb_add(c[3], t0, t2);
		fb_add(c[3], c[3], t8);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
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
	}
}

void fb4_mul_dxd(fb4_t c, fb4_t a, fb4_t b) {
	fb4_t _a, _b;
	fb_t t0, t1, t2, t3, t4, t5, t6, t7, t8, t9;
	dv_t t;

	fb4_null(_a);
	fb4_null(_b);

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
	dv_null(t);

	TRY {
		fb4_new(_a);
		fb4_new(_b);
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
		dv_new(t);

		/* First, convert to quartic polynomial basis. */
		fb_add(_a[1], a[1], a[2]);
		fb_add(_a[2], a[1], a[3]);
		fb_add(_b[1], b[1], b[2]);
		fb_add(_b[2], b[1], b[3]);

		/* w1 = u0 + u1 + u2 + u3. */
		fb_add(t1, a[0], _a[1]);
		fb_add(t1, t1, _a[2]);
		fb_add(t1, t1, a[3]);

		/* w2 = v0 + v1 + v2 + v3. */
		fb_add(t2, b[0], _b[1]);
		fb_add(t2, t2, _b[2]);
		fb_add(t2, t2, b[3]);

		/* w3 = w1 * w2. */
		fb_mul(t3, t1, t2);

		/* w0 = u1 + x * (u2 + x * u3). */
		fb_lsh1_low(t, a[3]);
		fb_add(t, t, _a[2]);
		t[FB_DIGS] = fb_lsh1_low(t, t);
		fb_rdc1_low(t0, t);
		fb_add(t0, t0, _a[1]);

		/* w6 = v1 + x * (v2 + x * v3). */
		fb_lsh1_low(t, b[3]);
		fb_add(t, t, _b[2]);
		t[FB_DIGS] = fb_lsh1_low(t, t);
		fb_rdc1_low(t6, t);
		fb_add(t6, t6, _b[1]);

		/* w4 = (w0 + u3 * (x + 1)) * x + w1. */
		fb_lsh1_low(t, a[3]);
		fb_add(t, t, a[3]);
		fb_add(t, t, t0);
		t[FB_DIGS] = fb_lsh1_low(t, t);
		fb_rdc1_low(t4, t);
		fb_add(t4, t4, t1);

		/* w5 = (w6 + v3 * (x + 1)) * x + w2. */
		fb_lsh1_low(t, b[3]);
		fb_add(t, t, b[3]);
		fb_add(t, t, t6);
		t[FB_DIGS] = fb_lsh1_low(t, t);
		fb_rdc1_low(t5, t);
		fb_add(t5, t5, t2);

		/* w0 = w0 * x + u0. */
		fb_lsh1_low(t, t0);
		fb_rdc1_low(t0, t);
		fb_add(t0, t0, a[0]);

		/* w6 = w6 * x + v0. */
		fb_lsh1_low(t, t6);
		fb_rdc1_low(t6, t);
		fb_add(t6, t6, b[0]);

		/* w5 = w5 * w4, w4 = w0 * w6. */
		fb_mul(t5, t5, t4);
		fb_mul(t4, t0, t6);

		/* w0 = u0 * x^3 + u1 * x^2 + u2 * x. */
		fb_lsh1_low(t, _a[2]);
		t[FB_DIGS] = fb_lshb_low(t7, _a[1], 2);
		t[FB_DIGS] ^= fb_lshb_low(t8, a[0], 3);
		fb_add(t, t, t7);
		fb_add(t, t, t8);
		fb_rdc1_low(t0, t);

		/* w6 = v0 * x^3 + v1 * x^2 + v2 * x. */
		fb_lsh1_low(t, _b[2]);
		t[FB_DIGS] = fb_lshb_low(t7, _b[1], 2);
		t[FB_DIGS] ^= fb_lshb_low(t8, b[0], 3);
		fb_add(t, t, t7);
		fb_add(t, t, t8);
		fb_rdc1_low(t6, t);

		/* w1 = w1 + w0 + u0 * (x^2 + x). */
		fb_mul_21(t7, a[0]);
		fb_add(t1, t1, t0);
		fb_add(t1, t1, t7);

		/* w2 = w2 + w6 + v0 * (x^2 + x). */
		fb_mul_21(t7, b[0]);
		fb_add(t2, t2, t6);
		fb_add(t2, t2, t7);

		/* w0 = w0 + u3, w6 = w6 + v3, w1 = w1 * w2, w2 = w0 * w6, w6 = u3 * v3, w0 = u0 * v0. */
		fb_add(t0, t0, a[3]);
		fb_add(t6, t6, b[3]);
		fb_mul(t1, t1, t2);
		fb_mul(t2, t0, t6);
		fb_mul(t6, a[3], b[3]);
		fb_mul(t0, a[0], b[0]);

		/* w3 = w3 + w0 + w6. */
		fb_add(t3, t3, t0);
		fb_add(t3, t3, t6);

		/* w1 = w1 + w2 + w0 * (x^4 + x^2 + 1). */
		fb_mul_420(t7, t0);
		fb_add(t1, t1, t2);
		fb_add(t1, t1, t7);

		/* w2 = w2 + w6 + w0 * x^6. */
		t[FB_DIGS] = fb_lshb_low(t, t0, 6);
		fb_rdc1_low(t7, t);
		fb_add(t2, t2, t6);
		fb_add(t2, t2, t7);

		fb_copy(t8, t4);
		/* w4 = w4 + w2 + w6 * x^6 + w0. */
		t[FB_DIGS] = fb_lshb_low(t, t6, 6);
		fb_rdc1_low(t7, t);
		fb_add(t4, t4, t2);
		fb_add(t4, t4, t0);
		fb_add(t4, t4, t7);

		fb_copy(t9, t1);
		/* w1 = w1 + w3. */
		fb_add(t1, t1, t3);

		/* w2 = w2 + w1 * x + w3 * x^2. */
		fb_lsh1_low(t, t1);
		fb_add(t, t, t2);
		t[FB_DIGS] = fb_lshb_low(t7, t3, 2);
		fb_add(t, t, t7);
		fb_rdc1_low(t2, t);

		/* (x^4 + x) * w5 = (x^4 + x) * (w5 + w4 + w6 * (x^4 + x^2 + 1) + w1) =
		 * w5 + w4 + w6 * (x^4 + x^2 + 1) + w1. */
		fb_mul_420(t7, t6);
		fb_add(t5, t5, t8);
		fb_add(t5, t5, t7);
		fb_add(t5, t5, t9);

		/* b * w4 = b * (w4 + w5 * (x^5 + x))/(x^4 + x^2) =
		 * (x^4 + x) * (w4 + w5 * (x^5 + x)). */
		fb_mul_51(t7, t5);
		fb_mul_41(t4, t4);
		fb_add(t4, t4, t7);

		/* b * w3 = b * (w3 + w4 + w5) = b * w3 + b * w4 + b * w5 =
		 * b * w3 + b * w4 + (x^4 + x^2) * (x^4 + x) * w5 */
		fb_mul_42(t5, t5);
		fb_mul_beta(t3, t3);
		fb_add(t3, t3, t4);
		fb_add(t3, t3, t5);

		/* b * (x^4 + x) * w1 = b * (w1 + w3 * (x^2 + x)) =
		 * b * w1 + b * w3 * (x^2 + x). */
		fb_mul_beta(t1, t1);
		fb_mul_21(t7, t3);
		fb_add(t8, t1, t7);

		/* b^2 * w1 = b^2 * (w1 + w3 * (x^2 + x))/(x^4 + x) =
		 * (x^4 + x^2) * (w1 + w3 * (x^2 + x)). */
		fb_mul_21(t7, t3);
		fb_add(t1, t1, t7);
		fb_mul_42(t1, t1);

		/* b * (x^4 + x) * w5 = b * (x^4 + x) * (w5 + w1). */
		fb_mul_41(t9, t5);
		fb_add(t9, t9, t8);

		/* b^2 * w5 = b^2 * (w5 + w1) = b^2 * w5 + b^2 * w1. */
		fb_mul_beta(t5, t5);
		fb_add(t5, t5, t1);

		/* b^2 * w2 = b^2 * (w2 + w5 * (x^2 + x))/(x^4 + x^2) =
		 * b * (x^4 + x) * (w2 + w5 * (x^2 + x)). */
		fb_mul_beta(t2, t2);
		fb_mul_41(t2, t2);
		fb_mul_21(t7, t9);
		fb_add(t2, t2, t7);

		/* b^2 * w4 = b^2 * w4 + b^2 * w2 = b * (b * w4) + b^2 * w2. */
		fb_mul_beta(t4, t4);
		fb_add(t4, t4, t2);

		/* b^2 * w0, b^2 * w6, b^2 * w3. */
		fb_mul_beta(t0, t0);
		fb_mul_beta(t6, t6);
		fb_mul_beta(t0, t0);
		fb_mul_beta(t6, t6);
		fb_mul_beta(t3, t3);

		/* Reduce modulo z^4 + z + 1 and convert to st-basis representation. */
		fb_add(c[0], t0, t4);
		fb_add(c[1], t2, t5);
		fb_add(c[1], c[1], t3);
		fb_add(c[2], t1, t4);
		fb_add(c[2], c[2], t2);
		fb_add(c[2], c[2], t3);
		fb_add(c[3], t3, t6);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb4_free(_a);
		fb4_free(_b);
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
		dv_free(t);
	}
}

void fb4_mul_dxs(fb4_t c, fb4_t a, fb4_t b) {
	fb_t t0, t1, t2, t3, t4, t5, t6;

	fb_null(t0);
	fb_null(t1);
	fb_null(t2);
	fb_null(t3);
	fb_null(t4);
	fb_null(t5);
	fb_null(t6);

	TRY {
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
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(t0);
		fb_free(t1);
		fb_free(t2);
		fb_free(t3);
		fb_free(t4);
		fb_free(t5);
		fb_free(t6);
	}
}

void fb4_mul_sxs(fb4_t c, fb4_t a, fb4_t b) {
	fb_t t0, t1, t3, t4, t5;

	fb_null(t0);
	fb_null(t1);
	fb_null(t3);
	fb_null(t4);
	fb_null(t5);

	TRY {
		fb_new(t0);
		fb_new(t1);
		fb_new(t3);
		fb_new(t4);
		fb_new(t5);

		fb_add(t0, a[0], a[1]);
		fb_add(t1, b[0], b[1]);

		fb_mul(t3, a[0], b[0]);
		fb_mul(t4, a[1], b[1]);
		fb_mul(t5, t0, t1);

		fb_add(c[3], a[1], b[1]);
		fb_add(c[2], a[0], b[0]);
		fb_add_dig(c[2], c[2], 1);
		fb_add(c[0], t3, t4);
		fb_add(c[1], t3, t5);
		fb_add_dig(c[1], c[1], 1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(t0);
		fb_free(t1);
		fb_free(t3);
		fb_free(t4);
		fb_free(t5);
	}
}
