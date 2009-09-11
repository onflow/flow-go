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
 * Implementation of the point negation on binary elliptic curves.
 *
 * @version $Id$
 * @ingroup ep
 */

#include "string.h"

#include "relic_core.h"
#include "relic_ep.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EP_ADD == BASIC || defined(EP_MIXED) || !defined(STRIP)

void ep_neg_basic(ep_t r, ep_t p) {
	if (ep_is_infty(p)) {
		ep_set_infty(r);
		return;
	}

	if (r != p) {
		fp_copy(r->x, p->x);
		fp_copy(r->z, p->z);
	}

#if defined(EP_SUPER)
	if (ep_curve_is_super()) {
		switch (ep_curve_opt_c()) {
			case EP_OPT_ZERO:
				fb_copy(r->y, p->y);
				break;
			case EP_OPT_ONE:
				fb_add_dig(r->y, p->y, (dig_t)1);
				break;
			case EP_OPT_DIGIT:
				fb_add_dig(r->y, p->y, ep_curve_get_c()[0]);
				break;
			default:
				fb_add(r->y, p->y, ep_curve_get_c());
				break;
		}

		r->norm = 1;
		return;
	}
#endif

	fp_sub(r->y, fp_prime_get(), p->y);

	r->norm = 1;
}

#endif
