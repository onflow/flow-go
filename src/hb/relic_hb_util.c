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
 * Implementation of the binary hyperelliptic curve utilities.
 *
 * @version $Id: relic_hb_util.c 390 2010-06-05 22:15:02Z dfaranha $
 * @ingroup hb
 */

#include "relic_core.h"
#include "relic_hb.h"
#include "relic_error.h"
#include "relic_conf.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

int hb_is_infty(hb_t p) {
	return (fb_is_zero(p->u1) && (fb_cmp_dig(p->u0, 1) == CMP_EQ) &&
			fb_is_zero(p->v1) && fb_is_zero(p->v0));
}

void hb_set_infty(hb_t p) {
	fb_zero(p->u1);
	fb_set_dig(p->u0, 1);
	fb_zero(p->v1);
	fb_zero(p->v0);
	p->deg = 1;
}

void hb_copy(hb_t r, hb_t p) {
	fb_copy(r->u1, p->u1);
	fb_copy(r->u0, p->u0);
	fb_copy(r->v1, p->v1);
	fb_copy(r->v0, p->v0);
	r->deg = p->deg;
}

int hb_cmp(hb_t p, hb_t q) {
	if (fb_cmp(p->u1, q->u1) != CMP_EQ) {
		return CMP_NE;
	}

	if (fb_cmp(p->u0, q->u0) != CMP_EQ) {
		return CMP_NE;
	}

	if (fb_cmp(p->v1, q->v1) != CMP_EQ) {
		return CMP_NE;
	}

	if (fb_cmp(p->v0, q->v0) != CMP_EQ) {
		return CMP_NE;
	}

	return CMP_EQ;
}

void hb_print(hb_t p) {
	fb_print(p->u1);
	fb_print(p->u0);
	fb_print(p->v1);
	fb_print(p->v0);
	printf("%d\n", p->deg);
}
