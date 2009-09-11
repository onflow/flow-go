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
 * Implementation of the point multiplication on prime elliptic curves.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "string.h"

#include "relic_core.h"
#include "relic_ep.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if EP_MUL == BASIC || !defined(STRIP)

void ep_mul_basic(ep_t r, ep_t p, bn_t k) {
	int i, l;
	ep_t t;

	ep_new(t);
	l = bn_bits(k);

	if (bn_test_bit(k, l - 1)) {
		ep_copy(t, p);
	} else {
		ep_set_infty(t);
	}

	for (i = l - 2; i >= 0; i--) {
		ep_dbl(t, t);
		if (bn_test_bit(k, i)) {
			ep_add(t, t, p);
		}
	}

	ep_copy(r, t);
	ep_norm(r, r);

	ep_free(t);
}

#endif
