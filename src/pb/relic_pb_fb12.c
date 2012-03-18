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
