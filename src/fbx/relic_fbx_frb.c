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
 * Implementation of frobenius action in binary fields extensions.
 *
 * @version $Id$
 * @ingroup fbx
 */

#include "relic_core.h"
#include "relic_fbx.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb4_frb(fb4_t c, fb4_t a) {
	int alpha;

	if (FB_BITS % 4 == 3) {
		alpha = 0;
	} else {
		alpha = 1;
	}

	if (alpha == 1) {
		fb_add(c[0], a[0], a[1]);
		fb_add(c[0], c[0], a[3]);
	} else {
		fb_add(c[0], a[0], a[1]);
		fb_add(c[0], c[0], a[2]);
	}
	fb_add(c[1], a[1], a[2]);
	if (alpha == 0) {
		fb_add(c[1], c[1], a[3]);
	}
	fb_add(c[2], a[2], a[3]);
	fb_copy(c[3], a[3]);
}

void fb6_frb(fb6_t c, fb6_t a) {
	fb_t t0, t1;

	fb_null(t0);
	fb_null(t1);

	TRY {
		fb_new(t0);
		fb_new(t1);

		switch (FB_BITS % 6) {
			case 1:
				fb_add(t0, a[4], a[5]);
				fb_copy(t1, a[3]);
				fb_add(c[0], a[0], a[1]);
				fb_add(c[0], c[0], a[4]);
				fb_add(c[1], a[1], t0);
				fb_copy(c[3], a[5]);
				fb_add(c[4], a[2], t1);
				fb_add(c[4], c[4], t0);
				fb_add(c[5], t1, a[5]);
				fb_copy(c[2], t0);
				break;
			case 2:
				fb_add(t0, a[2], a[4]);
				fb_add(c[0], a[0], a[3]);
				fb_add(c[0], c[0], t0);
				fb_add(c[1], a[1], a[2]);
				fb_add(c[1], c[1], a[5]);
				fb_add(c[3], a[3], a[5]);
				fb_copy(c[4], a[2]);
				fb_copy(c[5], a[5]);
				fb_copy(c[2], t0);
				break;
			case 3:
				fb_add(t0, a[3], a[5]);
				fb_copy(t1, a[2]);
				fb_add(c[0], a[0], a[1]);
				fb_add(c[0], c[0], a[3]);
				fb_add(c[1], a[1], a[4]);
				fb_add(c[1], c[1], t0);
				fb_copy(c[2], a[4]);
				fb_copy(c[3], a[5]);
				fb_add(c[4], a[4], t1);
				fb_copy(c[5], t0);
				break;
			case 4:
				fb_add(t0, a[3], a[5]);
				fb_copy(t1, a[2]);
				fb_add(c[0], a[0], a[2]);
				fb_add(c[0], c[0], a[5]);
				fb_add(c[1], a[1], a[4]);
				fb_add(c[1], c[1], t0);
				fb_copy(c[2], a[4]);
				fb_copy(c[3], a[5]);
				fb_add(c[4], a[4], t1);
				fb_copy(c[5], t0);
				break;
			case 5:
				fb_add(t0, a[2], a[3]);
				fb_copy(t1, a[3]);
				fb_add(c[0], a[0], a[1]);
				fb_add(c[0], c[0], a[3]);
				fb_add(c[1], a[1], a[2]);
				fb_add(c[2], a[4], a[5]);
				fb_add(c[2], c[2], t0);
				fb_add(c[3], a[3], a[5]);
				fb_copy(c[4], t0);
				fb_copy(c[5], t1);
				break;
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t0);
		fb_free(t1);
	}
}

void fb12_frb(fb12_t c, fb12_t a) {
	fb6_t t;

	fb6_null(t);

	TRY {
		fb6_new(t);

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
