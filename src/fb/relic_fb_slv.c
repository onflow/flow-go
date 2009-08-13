/*
 * Copyright 2007-2009 RELIC Project
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Implementation of quadratic equation solution on binary fields.
 *
 * @version $Id$
 * @ingroup fb
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_fb.h"
#include "relic_fb_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int fb_slv(fb_t c, fb_t a) {
	int i, result = 0;
	fb_t t0 = NULL, t1 = NULL;

	TRY {
		fb_new(t0);
		fb_new(t1);

		fb_copy(t0, a);
		fb_copy(c, a);

		for (i = 0; i < (FB_BITS - 1) / 2; i++) {
			fb_sqr(c, c);
			fb_sqr(c, c);
			fb_add(c, c, t0);
		}

		fb_copy(t1, c);
		fb_sqr(t1, t1);
		fb_add(t1, t1, c);
		if (fb_cmp(t0, t1) == CMP_EQ) {
			result = 1;
		} else {
			result = 0;
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		fb_free(t0);
		fb_free(t1);
	}
	return result;
}
