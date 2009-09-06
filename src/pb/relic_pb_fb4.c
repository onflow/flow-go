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
 * Implementation of the quartic extension binary field arithmetic module.
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

void fb4_mul(fb4_t c, fb4_t a, fb4_t b) {
	fb_t t0, t1, t2, t3, t4, t5, t6, t7, t8, t9;

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

void fb4_exp_2m(fb4_t c, fb4_t a) {
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
