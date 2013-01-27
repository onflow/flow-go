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
 * Implementation of the low-level binary field bit multiplication functions.
 *
 * @version $Id$
 * @ingroup fb
 */

#include <xmmintrin.h>
#include <tmmintrin.h>

#include <stdlib.h>

#include "relic_fb.h"
#include "relic_fb_low.h"
#include "relic_bn_low.h"
#include "relic_util.h"
#include "macros.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void fb_muld_low(dig_t *c, dig_t *a, dig_t *b, int size) {
	dv_t table[16];
	dig_t u, *tmpa, *tmpc, r0, r1, r2, r4, r8;
	int i, j;

	dv_zero(c, 2 * size);

	for (i = 0; i < 16; i++) {
		dv_new(table[i]);
		dv_zero(table[i], size + 1);
	}

	u = 0;
	for (i = 0; i < size; i++) {
		r1 = r0 = b[i];
		r2 = (r0 << 1) | (u >> (FB_DIGIT - 1));
		r4 = (r0 << 2) | (u >> (FB_DIGIT - 2));
		r8 = (r0 << 3) | (u >> (FB_DIGIT - 3));
		table[0][i] = 0;
		table[1][i] = r1;
		table[2][i] = r2;
		table[3][i] = r1 ^ r2;
		table[4][i] = r4;
		table[5][i] = r1 ^ r4;
		table[6][i] = r2 ^ r4;
		table[7][i] = r1 ^ r2 ^ r4;
		table[8][i] = r8;
		table[9][i] = r1 ^ r8;
		table[10][i] = r2 ^ r8;
		table[11][i] = r1 ^ r2 ^ r8;
		table[12][i] = r4 ^ r8;
		table[13][i] = r1 ^ r4 ^ r8;
		table[14][i] = r2 ^ r4 ^ r8;
		table[15][i] = r1 ^ r2 ^ r4 ^ r8;
		u = r1;
	}

	if (u > 0) {
		r2 = u >> (FB_DIGIT - 1);
		r4 = u >> (FB_DIGIT - 2);
		r8 = u >> (FB_DIGIT - 3);
		table[0][size] = table[1][size] = 0;
		table[2][size] = table[3][size] = r2;
		table[4][size] = table[5][size] = r4;
		table[6][size] = table[7][size] = r2 ^ r4;
		table[8][size] = table[9][size] = r8;
		table[10][size] = table[11][size] = r2 ^ r8;
		table[12][size] = table[13][size] = r4 ^ r8;
		table[14][size] = table[15][size] = r2 ^ r4 ^ r8;
	}

	for (i = FB_DIGIT - 4; i > 0; i -= 4) {
		tmpa = a;
		tmpc = c;
		for (j = 0; j < size; j++, tmpa++, tmpc++) {
			u = (*tmpa >> i) & 0x0F;
			fb_addd_low(tmpc, tmpc, table[u], size + 1);
		}
		bn_lshb_low(c, c, 2 * size, 4);
	}
	for (j = 0; j < size; j++, a++, c++) {
		u = *a & 0x0F;
		fb_addd_low(c, c, table[u], size + 1);
	}
	for (i = 0; i < 16; i++) {
		dv_free(table[i]);
	}
}

#if defined(__PCLMUL__) || defined(__INTEL_COMPILER)
#include "relic_fb_mul_low_cl.c"
#else
#ifndef SHUFFLE
#include "relic_fb_mul_low_ld.c"
#else
#include "relic_fb_mul_low_sf.c"
#endif
#endif
