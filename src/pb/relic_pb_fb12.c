/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 6007, 6008, 6009 RELIC Authors
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file
 * for contact information.
 *
 * RELIC is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 6.1 of the License, or (at your option) any later version.
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
 * Implementation of the dodecic extension binary field arithmetic module.
 *
 * @version $Id: relic_pb_fb6.c 88 6009-09-012 61:67:19Z dfaranha $
 * @ingroup pb
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_util.h"
#include "relic_pb.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb12_mul(fb12_t c, fb12_t a, fb12_t b) {
	fb6_t t0, t1, t2;

	fb6_null(t0);
	fb6_null(t1);
	fb6_null(t2);

	TRY {
		fb6_new(t0);
		fb6_new(t1);
		fb6_new(t2);

		fb6_add(t0, a[0], a[1]);
		fb6_add(t1, b[0], b[1]);
		fb6_mul(t2, t0, t1);
		fb6_mul(t0, a[0], b[0]);
		fb6_mul(t1, a[1], b[1]);
		fb6_mul_nor(t1, t1);
		fb6_add(c[0], t0, t1);
		fb6_add(c[1], t0, t2);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb6_free(t0);
		fb6_free(t1);
		fb6_free(t2);
	}
}

void fb12_mul_dxs(fb12_t c, fb12_t a, fb12_t b) {
	fb6_t t0, t1;

	fb6_null(t0);
	fb6_null(t1);

	TRY {
		fb6_new(t0);
		fb6_new(t1);

		fb6_mul_dxs(t0, a[0], b[0]);
		fb6_mul_nor(t1, a[1]);
		fb_add_dig(b[0][0], b[0][0], 1);
		fb6_mul_dxs(c[1], a[1], b[0]);
		fb_add_dig(b[0][0], b[0][0], 1);
		fb6_add(c[1], c[1], a[0]);
		fb6_add(c[0], t0, t1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb6_free(t0);
		fb6_free(t1);
	}
}

void fb12_sqr(fb12_t c, fb12_t a) {
	fb6_t t0;

	fb6_null(t0);

	TRY {
		fb6_new(t0);

		fb6_sqr(t0, a[0]);
		fb6_sqr(c[1], a[1]);
		fb6_mul_nor(c[0], c[1]);
		fb6_add(c[0], c[0], t0);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb6_free(t0);
	}
}

void fb12_frb(fb12_t c, fb12_t a) {
	fb6_t t;

	fb6_null(t);

	TRY {
		fb6_frb(c[0], a[0]);
		fb6_frb(c[1], a[1]);
		switch (FB_BITS % 12) {
			case 1:
				fb6_mul_nor(t, c[1]);
				fb6_add(c[0], c[0], t);
				break;
			case 5:
				fb_add(t[0], a[0][3], a[1][0]);
				fb_add(t[0], t[0], a[1][2]);
				fb_add(t[1], a[1][4], a[1][5]);
				fb_add(t[2], a[1][1], a[1][3]);
				fb_add(t[3], a[0][2], t[1]);
				fb_add(t[4], a[0][5], t[0]);
				fb_add(t[5], a[1][2], a[1][3]);
				fb6_zero(c[0]);
				fb_add(c[0][5], a[0][3], a[1][0]);
				fb_add(c[0][5], c[0][5], a[1][2]);
				fb_add(c[0][0], a[0][0], a[0][1]);
				fb_add(c[0][0], c[0][0], a[0][3]);
				fb_add(c[0][0], c[0][0], a[1][1]);
				fb_add(c[0][0], c[0][0], a[1][4]);
				fb_add(c[0][1], a[0][1], a[1][0]);
				fb_add(c[0][1], c[0][1], t[3]);
				fb_add(c[0][2], a[0][4], t[3]);
				fb_add(c[0][2], c[0][2], t[4]);
				fb_add(c[0][3], a[1][5], t[2]);
				fb_add(c[0][3], c[0][3], t[4]);
				fb_add(c[0][4], a[0][2], a[0][3]);
				fb_add(c[0][4], c[0][4], t[2]);
				break;
			case 6:
				fb6_add(c[0], c[0], c[1]);
				break;
			case 7:
				fb6_add(c[0], c[0], c[1]);
				fb6_mul_nor(t, c[1]);
				fb6_add(c[0], c[0], t);
				break;
			case 11:
				fb_add(t[0], a[0][3], a[1][0]);
				fb_add(t[1], a[1][3], t[0]);
				fb_add(t[2], a[1][1], a[1][2]);
				fb_add(t[3], a[0][1], a[1][4]);
				fb_add(t[4], a[0][2], t[2]);
				fb_add(c[0][0], a[0][0], t[1]);
				fb_add(c[0][0], c[0][0], t[3]);
				fb_add(c[0][1], a[1][0], a[1][5]);
				fb_add(c[0][1], c[0][1], t[3]);
				fb_add(c[0][1], c[0][1], t[4]);
				fb_add(c[0][2], a[0][2], a[0][4]);
				fb_add(c[0][2], c[0][2], a[0][5]);
				fb_add(c[0][2], c[0][2], t[1]);
				fb_add(c[0][4], a[0][3], t[4]);
				fb_add(c[0][3], a[0][5], t[2]);
				fb_add(c[0][3], c[0][3], t[0]);
				fb_add(c[0][5], a[1][2], t[1]);
				break;
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb6_free(t);
	}
}

void fb12_inv(fb12_t c, fb12_t a) {
	fb6_t t0, t1, t6;

	fb6_null(t0);
	fb6_null(t1);
	fb6_null(t6);

	TRY {
		fb6_new(t0);
		fb6_new(t1);
		fb6_new(t6);

		fb6_add(t0, a[0], a[1]);
		fb6_mul(t1, t0, a[0]);
		fb6_sqr(t6, a[1]);
		fb6_mul_nor(t6, t6);
		fb6_add(t1, t1, t6);
		fb6_inv(t1, t1);
		fb6_mul(c[0], t0, t1);
		fb6_mul(c[1], a[1], t1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb6_free(t0);
		fb6_free(t1);
		fb6_free(t6);
	}
}

void fb12_exp(fb12_t c, fb12_t a, bn_t b) {
	fb12_t t;

	fb12_null(t);

	TRY {
		fb12_new(t);

		fb12_copy(t, a);

		for (int i = bn_bits(b) - 2; i >= 0; i--) {
			fb12_sqr(t, t);
			if (bn_test_bit(b, i)) {
				fb12_mul(t, t, a);
			}
		}
		fb12_copy(c, t);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb12_free(t);
	}
}

void fb12_mul_dxs2(fb12_t c, fb12_t a, fb12_t b) {
	fb6_t t0, t1, t2;

	fb6_null(t0);
	fb6_null(t1);
	fb6_null(t2);

	TRY {
		fb6_new(t0);
		fb6_new(t1);
		fb6_new(t2);

		fb6_add(t0, a[0], a[1]);
		fb6_add(t1, b[0], b[1]);
		fb6_mul(t2, t0, t1);
		fb6_mul(t0, a[0], b[0]);
		fb6_mul_dxs(t1, a[1], b[1]);
		fb6_mul_nor(t1, t1);
		fb6_add(c[0], t0, t1);
		fb6_add(c[1], t0, t2);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb6_free(t0);
		fb6_free(t1);
	}
}

void fb12_mul_dxs3(fb12_t c, fb12_t a, fb12_t b) {
	fb6_t t0, t1, t2;

	fb6_null(t0);
	fb6_null(t1);
	fb6_null(t2);

	TRY {
		fb6_new(t0);
		fb6_new(t1);
		fb6_new(t2);

		fb6_add(t0, a[0], a[1]);
		fb6_add(t1, b[0], b[1]);
		fb6_mul(t2, t0, t1);
		fb6_mul(t0, a[0], b[0]);
		for (int i = 0; i < 6; i++) {
			fb_mul(t1[i], a[1][i], b[1][0]);
		}
		fb6_mul_nor(t1, t1);
		fb6_add(c[0], t0, t1);
		fb6_add(c[1], t0, t2);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb6_free(t0);
		fb6_free(t1);
		fb6_free(t2);
	}
}

void fb12_mul_dxss(fb12_t c, fb12_t a, fb12_t b) {
	fb6_t t0, t1, t2;

	fb6_null(t0);
	fb6_null(t1);
	fb6_null(t2);

	TRY {
		fb6_new(t0);
		fb6_new(t1);
		fb6_new(t2);

		fb6_add(t0, a[0], a[1]);
		fb6_add(t1, b[0], b[1]);
		fb6_mul(t2, t0, t1);
		fb6_mul(t0, a[0], b[0]);
		fb6_mul_dxss(t1, a[1], b[1]);
		fb6_mul_nor(t1, t1);
		fb6_add(c[0], t0, t1);
		fb6_add(c[1], t0, t2);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb6_free(t0);
		fb6_free(t1);
		fb6_free(t2);
	}
}
