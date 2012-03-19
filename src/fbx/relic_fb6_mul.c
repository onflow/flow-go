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
 * Implementation of multiplication in sextic extensions of binary fields.
 *
 * @version $Id: relic_fbx_mul.c 1137 2012-03-18 13:35:48Z dfaranha $
 * @ingroup fbx
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_util.h"
#include "relic_fbx.h"
#include "relic_fb_low.h"

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
		for (i = 0; i < 15; i++) {
			fb_free(t[i]);
			fb_free(p[i]);
			fb_free(u[i]);
			fb_free(v[i]);
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
		for (i = 0; i < 15; i++) {
			fb_free(t[i]);
			fb_free(p[i]);
			fb_free(u[i]);
			fb_free(v[i]);
		}
	}
}

void fb6_mul_dxss(fb6_t c, fb6_t a, fb6_t b) {
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

		fb_add(t[0], b[2], b[4]);

		fb_copy(v[0], b[0]);
		fb_copy(v[2], b[0]);
		fb_copy(v[3], b[4]);
		fb_copy(v[5], v[3]);
		fb_add(v[6], b[0], t[0]);
		fb_copy(v[8], v[6]);
		fb_add(v[9], b[4], b[0]);
		fb_copy(v[10], t[0]);
		fb_add(v[11], b[2], b[0]);
		fb_copy(v[12], v[11]);
		fb_copy(v[13], t[0]);
		fb_copy(v[14], v[9]);

		fb_mul(p[0], u[0], v[0]);
		fb_mul(p[2], u[2], v[2]);
		fb_mul(p[3], u[3], v[3]);
		fb_mul(p[5], u[5], v[5]);
		fb_mul(p[6], u[6], v[6]);
		fb_mul(p[8], u[8], v[8]);
		fb_mul(p[9], u[9], v[9]);
		fb_mul(p[10], u[10], v[10]);
		fb_mul(p[11], u[11], v[11]);
		fb_mul(p[12], u[12], v[12]);
		fb_mul(p[13], u[13], v[13]);
		fb_mul(p[14], u[14], v[14]);

		fb_copy(t[8], p[14]);
		fb_add(t[9], p[2], p[12]);
		fb_copy(t[10], p[3]);
		fb_add(t[12], p[5], p[6]);
		fb_add(t[12], t[12], t[8]);
		fb_add(t[12], t[12], t[9]);
		fb_add(t[13], p[0], p[13]);
		fb_add(t[13], t[13], t[10]);
		fb_add(t[13], t[13], p[8]);
		fb_add(t[14], p[2], p[9]);

		fb_add(c[0], p[9], p[11]);
		fb_add(c[0], c[0], p[8]);
		fb_add(c[0], c[0], t[12]);
		fb_add(c[1], p[10], p[11]);
		fb_add(c[1], c[1], t[8]);
		fb_add(c[1], c[1], t[13]);
		fb_add(c[2], p[0], p[8]);
		fb_add(c[2], c[2], p[10]);
		fb_add(c[2], c[2], t[14]);
		fb_add(c[3], p[11], p[6]);
		fb_add(c[3], c[3], t[14]);
		fb_add(c[4], t[9], t[13]);
		fb_add(c[5], t[10], t[12]);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		for (i = 0; i < 15; i++) {
			fb_free(t[i]);
			fb_free(p[i]);
			fb_free(u[i]);
			fb_free(v[i]);
		}
	}
}

void fb6_mul_nor(fb6_t c, fb6_t a) {
	fb_t t[3];
	int i;

	for (i = 0; i < 3; i++) {
		fb_null(t[i]);
	}

	TRY {
		for (i = 0; i < 3; i++) {
			fb_new(t[i]);
		}
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
		for (i = 0; i < 3; i++) {
			fb_free(t[i]);
		}
	}
}
