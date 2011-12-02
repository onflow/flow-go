/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
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
 * Implementation of the quadratic extension binary field arithmetic module.
 *
 * @version $Id$
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

void fb2_mul(fb2_t c, fb2_t a, fb2_t b) {
	fb_t t0, t1, t2;

	fb_null(t0);
	fb_null(t1);
	fb_null(t2);

	TRY {
		fb_new(t0);
		fb_new(t1);
		fb_new(t2);

		fb_add(t0, a[0], a[1]);
		fb_add(t1, b[0], b[1]);

		fb_mul(t0, t0, t1);
		fb_mul(t1, a[0], b[0]);
		fb_mul(t2, a[1], b[1]);

		fb_add(c[0], t1, t2);
		fb_add(c[1], t0, t1);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(t0);
		fb_free(t1);
		fb_free(t2);
	}
}

void fb2_mul_nor(fb2_t c, fb2_t a) {
	fb_t t;

	fb_null(t);

	TRY {
		fb_new(t);

		fb_copy(t, a[1]);
		fb_add(c[1], a[0], a[1]);
		fb_copy(c[0], t);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(t);
	}
}

void fb2_sqr(fb2_t c, fb2_t a) {
	fb_sqr(c[1], a[1]);
	fb_sqr(c[0], a[0]);
	fb_add(c[0], c[0], c[1]);
}

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
