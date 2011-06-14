/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
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
/*
 * void ft_mul1_low(dig_t *c, dig_t *a, dig_t digit) {
 * int j, k;
 * dig_t b1, b2;
 *
 * if (digit == 0) {
 * dv_zero(c, FT_DIGS + 1);
 * return;
 * }
 * if (digit == 1) {
 * ft_copy(c, a);
 * return;
 * }
 * c[FT_DIGS] = ft_lshb_low(c, a, util_bits_dig(digit) - 1);
 * for (int i = util_bits_dig(digit) - 2; i > 0; i--) {
 * if (digit & ((dig_t)1 << i)) {
 * j = FT_DIGIT - i;
 * b1 = a[0];
 * c[0] ^= (b1 << i);
 * for (k = 1; k < FT_DIGS; k++) {
 * b2 = a[k];
 * c[k] ^= ((b2 << i) | (b1 >> j));
 * b1 = b2;
 * }
 * c[FT_DIGS] ^= (b1 >> j);
 * }
 * }
 * if (digit & (dig_t)1) {
 * ft_add(c, c, a);
 * }
 * }
 */

void ft_muln_low(dig_t *c, dig_t *a, dig_t *b) {
	dv_t table[81];
	dig_t r0, r1, r2, r4, r8, u, carry;
	unsigned char k[FT_TRITS] = { 0 };
	int i, l;

	for (i = 0; i < 81; i++) {
		dv_null(table[i]);
	}
	for (i = 0; i < 2 * FT_DIGS; i++) {
		c[i] = 0;
	}
	for (i = 0; i < 81; i++) {
		dv_new(table[i]);
		dv_zero(table[i], 2 * FT_DIGS);
	}

	u = 0;
	for (i = 0; i < FT_DIGS / 2; i++) {
		r1 = r0 = b[i];
		r2 = (r0 << 1) | (u >> (FT_DIGIT - 1));
		r4 = (r0 << 2) | (u >> (FT_DIGIT - 2));
		r8 = (r0 << 3) | (u >> (FT_DIGIT - 3));
		table[0][i] = 0;
		table[1][i] = r1;
		table[3][i] = r2;
		table[9][i] = r4;
		u = r1;
	}
	if (u > 0) {
		r2 = u >> (FT_DIGIT - 1);
		r4 = u >> (FT_DIGIT - 2);
		r8 = u >> (FT_DIGIT - 3);
		table[0][FT_DIGS / 2] = table[1][FT_DIGS / 2] = 0;
		table[3][FT_DIGS / 2] = r2;
		table[9][FT_DIGS / 2] = r4;
	}

	for (i = FT_DIGS / 2; i < FT_DIGS; i++) {
		r1 = r0 = b[i];
		r2 = (r0 << 1) | (u >> (FT_DIGIT - 1));
		r4 = (r0 << 2) | (u >> (FT_DIGIT - 2));
		r8 = (r0 << 3) | (u >> (FT_DIGIT - 3));
		table[0][i + FT_DIGS/2] = 0;
		table[1][i + FT_DIGS/2] = r1;
		table[3][i + FT_DIGS/2] = r2;
		table[9][i + FT_DIGS/2] = r4;
		u = r1;
	}
	if (u > 0) {
		r2 = u >> (FT_DIGIT - 1);
		r4 = u >> (FT_DIGIT - 2);
		r8 = u >> (FT_DIGIT - 3);
		table[0][FT_DIGS + FT_DIGS/2] = table[1][FT_DIGS + FT_DIGS/2] = 0;
		table[3][FT_DIGS + FT_DIGS/2] = r2;
		table[9][FT_DIGS + FT_DIGS/2] = r4;
	}

	dv_copy(table[2] + FT_DIGS, table[1], FT_DIGS);
	dv_copy(table[2], table[1] + FT_DIGS, FT_DIGS);
	ft_addd_low(table[4], table[3], table[1], 2 * FT_DIGS, 2 * FT_DIGS);
	ft_addd_low(table[5], table[3], table[2], 2 * FT_DIGS, 2 * FT_DIGS);
	dv_copy(table[6] + FT_DIGS, table[3], FT_DIGS);
	dv_copy(table[6], table[3] + FT_DIGS, FT_DIGS);
	dv_copy(table[7] + FT_DIGS, table[5], FT_DIGS);
	dv_copy(table[7], table[5] + FT_DIGS, FT_DIGS);
	dv_copy(table[8] + FT_DIGS, table[4], FT_DIGS);
	dv_copy(table[8], table[4] + FT_DIGS, FT_DIGS);
	for (i = 1; i < 9; i++) {
		ft_addd_low(table[9 + i], table[9], table[i], 2 * FT_DIGS, 2 * FT_DIGS);
	}
	dv_copy(table[18] + FT_DIGS, table[9], FT_DIGS);
	dv_copy(table[18], table[9] + FT_DIGS, FT_DIGS);
	dv_copy(table[19] + FT_DIGS, table[11], FT_DIGS);
	dv_copy(table[19], table[11] + FT_DIGS, FT_DIGS);
	dv_copy(table[20] + FT_DIGS, table[10], FT_DIGS);
	dv_copy(table[20], table[10] + FT_DIGS, FT_DIGS);
	dv_copy(table[21] + FT_DIGS, table[15], FT_DIGS);
	dv_copy(table[21], table[15] + FT_DIGS, FT_DIGS);
	dv_copy(table[22] + FT_DIGS, table[17], FT_DIGS);
	dv_copy(table[22], table[17] + FT_DIGS, FT_DIGS);
	dv_copy(table[23] + FT_DIGS, table[16], FT_DIGS);
	dv_copy(table[23], table[16] + FT_DIGS, FT_DIGS);
	dv_copy(table[24] + FT_DIGS, table[12], FT_DIGS);
	dv_copy(table[24], table[12] + FT_DIGS, FT_DIGS);
	dv_copy(table[25] + FT_DIGS, table[14], FT_DIGS);
	dv_copy(table[25], table[14] + FT_DIGS, FT_DIGS);
	dv_copy(table[26] + FT_DIGS, table[13], FT_DIGS);
	dv_copy(table[26], table[13] + FT_DIGS, FT_DIGS);

	i = l = 0;
	while (i < ft_trits(a)) {
		k[l++] = ft_get_trit(a, i + 2) * 9 + ft_get_trit(a, i + 1) * 3 + ft_get_trit(a, i);
		i += 3;
	}

	for (i = l - 1; i > 0; i--) {
		ft_addd_low(c, c, table[k[i]], 2 * FT_DIGS, 2 * FT_DIGS);

		carry = ft_lshb_low(c, c, 3);
		ft_lshb_low(c + FT_DIGS / 2, c + FT_DIGS / 2, 3);
		c[FT_DIGS / 2] |= carry;

		carry = ft_lshb_low(c + FT_DIGS, c + FT_DIGS, 3);
		ft_lshb_low(c + FT_DIGS + FT_DIGS / 2, c + FT_DIGS + FT_DIGS / 2, 3);
		c[FT_DIGS + FT_DIGS / 2] |= carry;
	}
	ft_addd_low(c, c, table[k[0]], 2 * FT_DIGS, 2 * FT_DIGS);

	for (i = 0; i < 81; i++) {
		dv_free(table[i]);
	}
}

//void ft_muld_low(dig_t *c, dig_t *a, dig_t *b, int size) {
//  dv_t table[16];
//  dig_t u, *tmpa, *tmpc, r0, r1, r2, r4, r8;
//  int i, j;
//
//  for (i = 0; i < 16; i++) {
//      dv_null(table[i]);
//  }
//
//  dv_zero(c, 2 * size);
//
//  for (i = 0; i < 16; i++) {
//      dv_new(table[i]);
//      dv_zero(table[i], size + 1);
//  }
//
//  u = 0;
//  for (i = 0; i < size; i++) {
//      r1 = r0 = b[i];
//      r2 = (r0 << 1) | (u >> (FT_DIGIT - 1));
//      r4 = (r0 << 2) | (u >> (FT_DIGIT - 2));
//      r8 = (r0 << 3) | (u >> (FT_DIGIT - 3));
//      table[0][i] = 0;
//      table[1][i] = r1;
//      table[2][i] = r2;
//      table[3][i] = r1 ^ r2;
//      table[4][i] = r4;
//      table[5][i] = r1 ^ r4;
//      table[6][i] = r2 ^ r4;
//      table[7][i] = r1 ^ r2 ^ r4;
//      table[8][i] = r8;
//      table[9][i] = r1 ^ r8;
//      table[10][i] = r2 ^ r8;
//      table[11][i] = r1 ^ r2 ^ r8;
//      table[12][i] = r4 ^ r8;
//      table[13][i] = r1 ^ r4 ^ r8;
//      table[14][i] = r2 ^ r4 ^ r8;
//      table[15][i] = r1 ^ r2 ^ r4 ^ r8;
//      u = r1;
//  }
//
//  if (u > 0) {
//      r2 = u >> (FT_DIGIT - 1);
//      r4 = u >> (FT_DIGIT - 2);
//      r8 = u >> (FT_DIGIT - 3);
//      table[0][size] = table[1][size] = 0;
//      table[2][size] = table[3][size] = r2;
//      table[4][size] = table[5][size] = r4;
//      table[6][size] = table[7][size] = r2 ^ r4;
//      table[8][size] = table[9][size] = r8;
//      table[10][size] = table[11][size] = r2 ^ r8;
//      table[12][size] = table[13][size] = r4 ^ r8;
//      table[14][size] = table[15][size] = r2 ^ r4 ^ r8;
//  }
//
//  for (i = FT_DIGIT - 4; i > 0; i -= 4) {
//      tmpa = a;
//      tmpc = c;
//      for (j = 0; j < size; j++, tmpa++, tmpc++) {
//          u = (*tmpa >> i) & 0x0F;
//          ft_addd_low(tmpc, tmpc, table[u], size + 1);
//      }
//      bn_lshb_low(c, c, 2 * size, 4);
//  }
//  for (j = 0; j < size; j++, a++, c++) {
//      u = *a & 0x0F;
//      ft_addd_low(c, c, table[u], size + 1);
//  }
//  for (i = 0; i < 16; i++) {
//      dv_free(table[i]);
//  }
//}

void ft_mulm_low(dig_t *c, dig_t *a, dig_t *b) {
	dig_t align t[2 * FT_DIGS];

	ft_muln_low(t, a, b);
	ft_rdcm_low(c, t);
}
