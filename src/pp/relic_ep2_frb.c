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
 * Implementation of frobenius action on prime elliptic curves over
 * quadratic extensions.
 *
 * @version $Id$
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
	fp2_frb(r->x, p->x);
	fp2_frb(r->y, p->y);
	fp2_mul_frb(r->x, r->x, 2);
	fp2_mul_frb(r->y, r->y, 3);
	r->norm = 1;
	fp_set_dig(r->z[0], 1);
	fp_zero(r->z[1]);
}

void ep2_frb_sqr(ep2_t r, ep2_t p) {
	fp2_mul_frb_sqr(r->x, p->x, 2);
	fp2_neg(r->y, p->y);
	r->norm = 1;
	fp_set_dig(r->z[0], 1);
	fp_zero(r->z[1]);
}
