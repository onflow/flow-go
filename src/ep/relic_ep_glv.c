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
 * Implementation of the GLV auxiliary functions.
 *
 * @version $Id$
 * @ingroup eb
 */

#include "relic_core.h"
#include "relic_ep.h"
#include "relic_error.h"

#if EP_MUL == GLV || EP_FIX == GLV || !defined(STRIP)

void ep_glv_end(ep_t q, ep_t p) {
	ep_copy(q, p);
	fp_mul(q->x, q->x, ep_curve_get_glvb());
}

void ep_glv_dec(bn_t k0, bn_t k1, bn_t k) {
	bn_t b1, b2, t1, t2;
	int r1, r2, bits;

	bn_null(b1);
	bn_null(b2);
	bn_null(t1);
	bn_null(t2);
	bn_new(b1);
	bn_new(b2);

	ep_curve_get_ord(t1);
	bits = bn_bits(t1);

	ep_curve_get_glv1(t1);
	ep_curve_get_glv2(t2);
	bn_neg(t2, t2);

	bn_mul(b1, k, t1);
	r1 = bn_get_bit(b1, bits);
	bn_rsh(b1, b1, bits + 1);
	if (r1) {
		bn_add_dig(b1, b1, 1);
	}

	bn_mul(b2, k, t2);
	r2 = bn_get_bit(b2, bits);
	bn_rsh(b2, b2, bits + 1);
	if (r2) {
		bn_add_dig(b2, b2, 1);
	}

	ep_curve_get_glv_v10(t1);
	bn_mul(k0, b1, t1);
	ep_curve_get_glv_v20(t1);
	bn_mul(t1, b2, t1);
	bn_add(k0, k0, t1);

	ep_curve_get_glv_v11(t1);
	bn_mul(k1, b1, t1);
	ep_curve_get_glv_v21(t1);
	bn_mul(t1, b2, t1);
	bn_add(k1, k1, t1);

	bn_sub(k0, k, k0);
	bn_neg(k1, k1);

	bn_free(b1);
	bn_free(b2);
	bn_free(t1);
	bn_free(t2);
}

#endif
