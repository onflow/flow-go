/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2013 RELIC Authors
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
 * Implementation of the low-level binary field bit shifting functions.
 *
 * @version $Id$
 * @ingroup fb
 */

#include <stdlib.h>

#include "relic_ft.h"
#include "relic_ft_low.h"
#include "relic_bn_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ft_muln_low(dig_t *c, dig_t *a, dig_t *b) {
	align dig_t t[81][2 * FT_DIGS + 1];
	dig_t r0, r1, r2, r4, u, carry;
	unsigned char k[FT_TRITS] = { 0 };
	int i, l;

	for (i = 0; i < 81; i++) {
		dv_zero(t[i], 2 * FT_DIGS + 1);
	}
	for (i = 0; i < 2 * FT_DIGS; i++) {
		c[i] = 0;
	}

	u = 0;
	for (i = 0; i < FT_DIGS / 2; i++) {
		r1 = r0 = b[i];
		r2 = (r0 << 1) | (u >> (FT_DIGIT - 1));
		r4 = (r0 << 2) | (u >> (FT_DIGIT - 2));
		t[0][i] = 0;
		t[1][i] = r1;
		t[3][i] = r2;
		t[9][i] = r4;
		u = r1;
	}
	if (u > 0) {
		r2 = u >> (FT_DIGIT - 1);
		r4 = u >> (FT_DIGIT - 2);
		t[0][FT_DIGS / 2] = t[1][FT_DIGS / 2] = 0;
		t[3][FT_DIGS / 2] = r2;
		t[9][FT_DIGS / 2] = r4;
	}

	for (i = FT_DIGS / 2; i < FT_DIGS; i++) {
		r1 = r0 = b[i];
		r2 = (r0 << 1) | (u >> (FT_DIGIT - 1));
		r4 = (r0 << 2) | (u >> (FT_DIGIT - 2));
		t[0][i + FT_DIGS/2] = 0;
		t[1][i + FT_DIGS/2] = r1;
		t[3][i + FT_DIGS/2] = r2;
		t[9][i + FT_DIGS/2] = r4;
		u = r1;
	}
	if (u > 0) {
		r2 = u >> (FT_DIGIT - 1);
		r4 = u >> (FT_DIGIT - 2);
		t[0][FT_DIGS + FT_DIGS/2] = t[1][FT_DIGS + FT_DIGS/2] = 0;
		t[3][FT_DIGS + FT_DIGS/2] = r2;
		t[9][FT_DIGS + FT_DIGS/2] = r4;
	}

	dv_copy(t[2] + FT_DIGS, t[1], FT_DIGS);
	dv_copy(t[2], t[1] + FT_DIGS, FT_DIGS);
	ft_addd_low(t[4], t[3], t[1], 2 * FT_DIGS, 2 * FT_DIGS);
	ft_addd_low(t[5], t[3], t[2], 2 * FT_DIGS, 2 * FT_DIGS);
	dv_copy(t[6] + FT_DIGS, t[3], FT_DIGS);
	dv_copy(t[6], t[3] + FT_DIGS, FT_DIGS);
	dv_copy(t[7] + FT_DIGS, t[5], FT_DIGS);
	dv_copy(t[7], t[5] + FT_DIGS, FT_DIGS);
	dv_copy(t[8] + FT_DIGS, t[4], FT_DIGS);
	dv_copy(t[8], t[4] + FT_DIGS, FT_DIGS);
	for (i = 1; i < 9; i++) {
		ft_addd_low(t[9 + i], t[9], t[i], 2 * FT_DIGS, 2 * FT_DIGS);
	}
	dv_copy(t[18] + FT_DIGS, t[9], FT_DIGS);
	dv_copy(t[18], t[9] + FT_DIGS, FT_DIGS);
	dv_copy(t[19] + FT_DIGS, t[11], FT_DIGS);
	dv_copy(t[19], t[11] + FT_DIGS, FT_DIGS);
	dv_copy(t[20] + FT_DIGS, t[10], FT_DIGS);
	dv_copy(t[20], t[10] + FT_DIGS, FT_DIGS);
	dv_copy(t[21] + FT_DIGS, t[15], FT_DIGS);
	dv_copy(t[21], t[15] + FT_DIGS, FT_DIGS);
	dv_copy(t[22] + FT_DIGS, t[17], FT_DIGS);
	dv_copy(t[22], t[17] + FT_DIGS, FT_DIGS);
	dv_copy(t[23] + FT_DIGS, t[16], FT_DIGS);
	dv_copy(t[23], t[16] + FT_DIGS, FT_DIGS);
	dv_copy(t[24] + FT_DIGS, t[12], FT_DIGS);
	dv_copy(t[24], t[12] + FT_DIGS, FT_DIGS);
	dv_copy(t[25] + FT_DIGS, t[14], FT_DIGS);
	dv_copy(t[25], t[14] + FT_DIGS, FT_DIGS);
	dv_copy(t[26] + FT_DIGS, t[13], FT_DIGS);
	dv_copy(t[26], t[13] + FT_DIGS, FT_DIGS);

	i = l = 0;
	while (i < ft_trits(a)) {
		k[l++] = ft_get_trit(a, i + 2) * 9 + ft_get_trit(a, i + 1) * 3 + ft_get_trit(a, i);
		i += 3;
	}

	for (i = l - 1; i > 0; i--) {
		ft_addd_low(c, c, t[k[i]], 2 * FT_DIGS, 2 * FT_DIGS);

		carry = ft_lshb_low(c, c, 3);
		ft_lshb_low(c + FT_DIGS / 2, c + FT_DIGS / 2, 3);
		c[FT_DIGS / 2] |= carry;

		carry = ft_lshb_low(c + FT_DIGS, c + FT_DIGS, 3);
		ft_lshb_low(c + FT_DIGS + FT_DIGS / 2, c + FT_DIGS + FT_DIGS / 2, 3);
		c[FT_DIGS + FT_DIGS / 2] |= carry;
	}
	ft_addd_low(c, c, t[k[0]], 2 * FT_DIGS, 2 * FT_DIGS);
}

void ft_mulm_low(dig_t *c, dig_t *a, dig_t *b) {
	dig_t align t[2 * FT_DIGS];

	ft_muln_low(t, a, b);
	ft_rdcm_low(c, t);
}
