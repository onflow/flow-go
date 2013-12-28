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
 * Implementation of inversion in binary fields extensions.
 *
 * @version $Id$
 * @ingroup fbx
 */

#include "relic_core.h"
#include "relic_fbx.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb2_inv(fb2_t c, fb2_t a) {
	fb_t a0, a1, m0, m1;

	fb_null(a0);
	fb_null(a1);
	fb_null(m0);
	fb_null(m1);

	TRY {
		fb_new(a0);
		fb_new(a1);
		fb_new(m0);
		fb_new(m1);

		fb_add(a0, a[0], a[1]);
		fb_sqr(m0, a[0]);
		fb_mul(m1, a0, a[1]);
		fb_add(a1, m0, m1);
		fb_inv(a1, a1);
		fb_mul(c[0], a0, a1);
		fb_mul(c[1], a[1], a1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(a0);
		fb_free(a1);
		fb_free(m0);
		fb_free(m1);
	}
}

void fb4_inv(fb4_t c, fb4_t a) {
	fb2_t a0, a1, m0, m1;

	fb2_null(a0);
	fb2_null(a1);
	fb2_null(m0);
	fb2_null(m1);

	TRY {
		fb2_new(a0);
		fb2_new(a1);
		fb2_new(m0);
		fb2_new(m1);

		fb2_add(a0, a, (a + 2));
		fb2_mul(m1, a0, a);
		fb2_sqr(m0, a + 2);
		fb2_copy(a1, m0);
		fb_add(m0[1], a1[0], a1[1]);
		fb_copy(m0[0], a1[1]);
		fb2_add(m1, m0, m1);
		fb2_inv(m1, m1);
		fb2_mul(c, a0, m1);
		fb2_mul(c + 2, a + 2, m1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb2_free(a0);
		fb2_free(a1);
		fb2_free(m0);
		fb2_free(m1);
	}
}
