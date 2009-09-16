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
 * Implementation of the point normalization on prime elliptic curves.
 *
 * @version $Id$
 * @ingroup ep
 */

#include "string.h"

#include "relic_core.h"
#include "relic_ep.h"
#include "relic_ep2.h"
#include "relic_fp2.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#if EP_ADD == PROJC || defined(EP_MIXED)

#if defined(EP_ORDIN)

/**
 * Normalizes a point represented in projective coordinates.
 *
 * @param r			- the result.
 * @param p			- the point to normalize.
 */
void ep2_norm_ordin(ep2_t r, ep2_t p) {
	if (!p->norm) {
		fp2_t t0, t1;

		fp2_new(t0);
		fp2_new(t1);

		fp2_inv(t1, p->z);
		fp2_sqr(t0, t1);
		fp2_mul(r->x, p->x, t0);
		fp2_mul(t0, t0, t1);
		fp2_mul(r->y, p->y, t0);
		fp_zero(r->z[0]);
		fp_zero(r->z[1]);
		fp2_set_dig(r->z, 1);

		fp2_free(t0);
	}

	r->norm = 1;
}

#endif /* EP_ORDIN */

#endif /* EP_ADD == PROJC || EP_MIXED */

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ep2_norm(ep2_t r, ep2_t p) {
	if (ep2_is_infinity(p)) {
		ep2_set_infinity(r);
		return;
	}

	if (p->norm) {
		/* If the point is represented in affine coordinates, we just copy it. */
		ep2_copy(r, p);
	}

#if defined(EP_ORDIN)
	ep2_norm_ordin(r, p);
#endif /* EP_ORDIN */
}
