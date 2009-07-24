/*
 * Copyright 2007 Project RELIC
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
 * Implementation of the Frobenius map on binary elliptic curves.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_eb.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if defined(EB_KBLTZ)

#if EB_ADD == BASIC || defined(EB_MIXED) || !defined(STRIP)

void eb_frb_basic(eb_t r, eb_t p) {
	if (eb_is_infty(p)) {
		return;
	}

	fb_sqr(r->x, p->x);
	fb_sqr(r->y, p->y);

	r->norm = 1;
}

#endif

#if EB_ADD == PROJC || defined(EB_MIXED) || !defined(STRIP)

void eb_frb_projc(eb_t r, eb_t p) {
	if (eb_is_infty(p)) {
		return;
	}

	fb_sqr(r->x, p->x);
	fb_sqr(r->y, p->y);
	if (!p->norm) {
		fb_sqr(r->z, p->z);
	} else {
		if (r != p) {
			fb_copy(r->z, p->z);
		}
	}

	r->norm = p->norm;
}

#endif

#endif
