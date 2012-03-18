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
 * Implementation of squaring in extensions defined over binary fields.
 *
 * @version $Id$
 * @ingroup fbx
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_util.h"
#include "relic_pb.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb2_sqr(fb2_t c, fb2_t a) {
	fb_sqr(c[1], a[1]);
	fb_sqr(c[0], a[0]);
	fb_add(c[0], c[0], c[1]);
}

void fb4_sqr(fb4_t c, fb4_t a) {
	fb_sqr(c[3], a[3]);
	fb_sqr(c[2], a[2]);
	fb_sqr(c[1], a[1]);
	fb_sqr(c[0], a[0]);
	fb_add(c[0], c[0], c[1]);
	fb_add(c[0], c[0], c[3]);
	fb_add(c[1], c[1], c[2]);
	fb_add(c[2], c[2], c[3]);
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

