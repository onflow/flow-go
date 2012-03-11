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
 * Implementation of divisor class negation on binary hyperelliptic curves.
 *
 * @version $Id$
 * @ingroup hb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_hb.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void hb_neg_basic(hb_t r, hb_t p) {
	if (hb_is_infty(p)) {
		hb_set_infty(r);
		return;
	}

	if (r != p) {
		hb_copy(r, p);
	}
#if defined(HB_SUPER)
	if (hb_curve_is_super()) {
		fb_add_dig(r->v0, r->v0, 1);
		return;
	}
#endif
}
