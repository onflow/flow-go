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
 * Implementation of quadratic equation solution on binary extension fields.
 *
 * @version $Id$
 * @ingroup fb
 */

#include "relic_core.h"
#include "relic_fbx.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb2_slv(fb2_t c, fb2_t a) {
	fb_t t;

	fb_null(t);

	TRY {
		fb_new(t);

		/* Compute t^2 + t = a_1. */
		fb_slv(t, a[1]);
		/* Compute c_0 = a_0 + a_1 + t. */
		fb_add(c[0], t, a[0]);
		fb_add(c[0], c[0], a[1]);
		fb_add_dig(c[0], c[0], fb_trc(t));
		/* Make Trc(c_0) = 0. */
		fb_slv(c[0], c[0]);
		/* Compute c_0^2 + c_0 = c_0. */
		fb_add_dig(c[1], t, fb_trc(t));
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fb_free(t);
	}
}
