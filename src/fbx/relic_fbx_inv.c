/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2013 RELIC Authors
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

void fb6_inv(fb6_t c, fb6_t a) {
	fb2_t t0, t1, t2, t3, t4, t5;
	fb2_t q0, q1, q2;

	fb2_null(t0);
	fb2_null(t1);
	fb2_null(t2);
	fb2_null(t3);
	fb2_null(t4);
	fb2_null(t5);
	fb2_null(q0);
	fb2_null(q1);
	fb2_null(q2);

	TRY {
		fb2_new(t0);
		fb2_new(t1);
		fb2_new(t2);
		fb2_new(t3);
		fb2_new(t4);
		fb2_new(t5);
		fb2_new(q0);
		fb2_new(q1);
		fb2_new(q2);

		/* t0 = a_0 + a_1, t1 = a_0 + a_1 + a_2 */
		fb2_add(t0, a, (a + 2));
		fb2_add(t1, t0, (a + 4));

		/* q0 = a_0^2, q1 = a_1^2, q2 = a_2^2. */
		fb2_sqr(q0, a);
		fb2_sqr(q1, (a + 2));
		fb2_sqr(q2, (a + 4));

		fb2_sqr(t2, t0);
		fb2_mul(t2, t2, t0);
		fb2_mul(t3, a, (a + 2));
		fb2_mul(t4, t3, (a + 4));
		fb2_add(t0, t2, t4);

		fb2_sqr(t2, t1);
		fb2_mul(t2, t2, t1);
		fb2_mul(t4, q0, a);

		fb2_add(t1, t4, t2);
		fb2_add(t0, t0, t1);
		fb2_mul_nor(t1, t1);
		fb2_add(t0, t0, t1);

		fb2_inv(t0, t0);

		fb2_mul(t4, a, (a + 4));
		fb2_mul(t5, (a + 2), (a + 4));

		fb2_add((c + 4), t4, q1);
		fb2_add(c, t3, (c + 4));
		fb2_mul_nor(c, c);
		fb2_add(c, c, t4);
		fb2_add(c, c, t5);
		fb2_add(c, c, q0);

		fb2_add(t4, t5, q2);
		fb2_add((c + 2), t5, q1);
		fb2_mul_nor((c + 2), (c + 2));
		fb2_add((c + 2), (c + 2), t3);
		fb2_add((c + 2), (c + 2), t4);

		fb2_mul_nor(t4, t4);
		fb2_add((c + 4), (c + 4), t4);

		fb2_mul(c, c, t0);
		fb2_mul((c + 2), (c + 2), t0);
		fb2_mul((c + 4), (c + 4), t0);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb2_free(t0);
		fb2_free(t1);
		fb2_free(t2);
		fb2_free(t3);
		fb2_free(t4);
		fb2_free(t5);
		fb2_free(q0);
		fb2_free(q1);
		fb2_free(q2);
	}
}

void fb12_inv(fb12_t c, fb12_t a) {
	fb6_t t0, t1, t2;

	fb6_null(t0);
	fb6_null(t1);
	fb6_null(t2);

	TRY {
		fb6_new(t0);
		fb6_new(t1);
		fb6_new(t2);

		fb6_add(t0, a[0], a[1]);
		fb6_mul(t1, t0, a[0]);
		fb6_sqr(t2, a[1]);
		fb6_mul_nor(t2, t2);
		fb6_add(t1, t1, t2);
		fb6_inv(t1, t1);
		fb6_mul(c[0], t0, t1);
		fb6_mul(c[1], a[1], t1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb6_free(t0);
		fb6_free(t1);
		fb6_free(t2);
	}
}
