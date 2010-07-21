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
 * Implementation of frobenius action on prime elliptic curves over
 * quadratic extensions.
 *
 * @version $Id: relic_pp_ep2.c 463 2010-07-13 21:12:13Z conradoplg $
 * @ingroup pp
 */

#include "relic_core.h"
#include "relic_md.h"
#include "relic_pp.h"
#include "relic_error.h"
#include "relic_conf.h"
#include "relic_fp_low.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ep2_frb(ep2_t r, ep2_t p) {
	fp2_t t;

	fp2_null(t);

	TRY {
		fp2_new(t);

		fp2_const_get(t);

		fp2_frb(r->x, p->x);
		fp2_frb(r->y, p->y);
		fp2_mul(r->y, r->y, t);
		fp2_sqr(t, t);
		fp2_mul(r->x, r->x, t);
		fp2_mul(r->y, r->y, t);
		r->norm = 1;
		fp_set_dig(r->z[0], 1);
		fp_zero(r->z[1]);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	} FINALLY {
		fp2_free(t);
	}
}
