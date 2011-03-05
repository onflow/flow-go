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
 * Implementation of the low-level modular reduction functions.
 *
 * @version $Id$
 * @ingroup ft
 */

#include <stdlib.h>

#include "relic_ft.h"
#include "relic_ft_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#define FT_ADD(A_H, A_L, B_H, B_L, T, F)									\
		if (F < 0) {														\
			T = ((A_L) | (A_H)) & ((B_L) | (B_H));							\
			A_L = (T) ^ ((A_L) | (B_L));									\
			A_H = (T) ^ ((A_H) | (B_H));									\
		} else {															\
			T = ((A_L) | (A_H)) & ((B_H) | (B_L));							\
			A_L = (T) ^ ((A_L) | (B_H));									\
			A_H = (T) ^ ((A_H) | (B_L));									\
		}																	\

static void ft_rdct_low(dig_t *c, dig_t *a, int fa, int fb, int size) {
	int i, sh, lh, rh, sa, la, ra;
	dig_t d0, d1, t;

	SPLIT(rh, sh, FT_TRITS, FT_DIG_LOG);
	sh++;
	lh = FT_DIGIT - rh;

	SPLIT(ra, sa, FT_TRITS - abs(fa), FT_DIG_LOG);
	sa++;
	la = FT_DIGIT - ra;

	for (i = size - 1; i >= FT_DIGS/2; i--) {
		d0 = a[i];
		d1 = a[i + size];
		a[i] = a[i + size] = 0;

		if (rh == 0) {
			FT_ADD(a[i - sh + 1 + size], a[i - sh + 1], d1, d0, t, fb)
		} else {
			FT_ADD(a[i - sh + 1 + size], a[i - sh + 1], d1 >> rh, d0 >> rh, t, fb)
			FT_ADD(a[i - sh + size], a[i - sh], d1 << lh, d0 << lh, t, fb)
		}
		if (ra == 0) {
			FT_ADD(a[i - sa + 1 + size], a[i - sa + 1], d1, d0, t, fa)
		} else {
			FT_ADD(a[i - sa + 1 + size], a[i - sa + 1], d1 >> ra, d0 >> ra, t, fa)
			FT_ADD(a[i - sa + size], a[i - sa], d1 << la, d0 << la, t, fa)
		}
	}

	d0 = a[i] >> rh;
	d1 = a[i + size] >> rh;
	d0 <<= rh;
	d1 <<= rh;

	if (rh == 0) {
		FT_ADD(a[i - sh + 1 + size], a[i - sh + 1], d1, d0, t, fb)
	} else {
		FT_ADD(a[i - sh + 1 + size], a[i - sh + 1], d1 >> rh, d0 >> rh, t, fb)
		FT_ADD(a[i - sh + size], a[i - sh], d1 << lh, d0 << lh, t, fb)
	}
	if (ra == 0) {
		FT_ADD(a[i - sa + 1 + size], a[i - sa + 1], d1, d0, t, fa)
	} else {
		FT_ADD(a[i - sa + 1 + size], a[i - sa + 1], d1 >> ra, d0 >> ra, t, fa)
		FT_ADD(a[i - sa + size], a[i - sa], d1 << la, d0 << la, t, fa)
	}
	a[i] ^= d0;
	a[i + size] ^= d1;

	dv_copy(c, a, FT_DIGS/2);
	dv_copy(c + FT_DIGS/2, a + size, FT_DIGS/2);
}

static void ft_rdcp_low(dig_t *c, dig_t *a, int fa, int fb, int fc, int fd, int size) {
	int i, sh, lh, rh, sa, la, ra, sb, lb, rb, sc, rc, lc;
	dig_t d0, d1, t;

	SPLIT(rh, sh, FT_TRITS, FT_DIG_LOG);
	sh++;
	lh = FT_DIGIT - rh;

	SPLIT(ra, sa, FT_TRITS - abs(fa), FT_DIG_LOG);
	sa++;
	la = FT_DIGIT - ra;

	SPLIT(rb, sb, FT_TRITS - abs(fb), FT_DIG_LOG);
	sb++;
	lb = FT_DIGIT - rb;

	SPLIT(rc, sc, FT_TRITS - abs(fc), FT_DIG_LOG);
	sc++;
	lc = FT_DIGIT - rc;

	for (i = size - 1; i >= FT_DIGS/2; i--) {
		d0 = a[i];
		d1 = a[i + size];
		a[i] = a[i + size] = 0;

		if (rh == 0) {
			FT_ADD(a[i - sh + 1 + size], a[i - sh + 1], d1, d0, t, fd)
		} else {
			FT_ADD(a[i - sh + 1 + size], a[i - sh + 1], d1 >> rh, d0 >> rh, t, fd)
			FT_ADD(a[i - sh + size], a[i - sh], d1 << lh, d0 << lh, t, fd)
		}
		if (ra == 0) {
			FT_ADD(a[i - sa + 1 + size], a[i - sa + 1], d1, d0, t, fa)
		} else {
			FT_ADD(a[i - sa + 1 + size], a[i - sa + 1], d1 >> ra, d0 >> ra, t, fa)
			FT_ADD(a[i - sa + size], a[i - sa], d1 << la, d0 << la, t, fa)
		}
		if (rb == 0) {
			FT_ADD(a[i - sb + 1 + size], a[i - sb + 1], d1, d0, t, fb)
		} else {
			FT_ADD(a[i - sb + 1 + size], a[i - sb + 1], d1 >> rb, d0 >> rb, t, fb)
			FT_ADD(a[i - sb + size], a[i - sb], d1 << lb, d0 << lb, t, fb)
		}
		if (rc == 0) {
			FT_ADD(a[i - sc + 1 + size], a[i - sc + 1], d1, d0, t, fc)
		} else {
			FT_ADD(a[i - sc + 1 + size], a[i - sc + 1], d1 >> rc, d0 >> rc, t, fc)
			FT_ADD(a[i - sc + size], a[i - sc], d1 << lc, d0 << lc, t, fc)
		}
	}

	d0 = a[i] >> rh;
	d1 = a[i + size] >> rh;
	d0 <<= rh;
	d1 <<= rh;

	if (rh == 0) {
		FT_ADD(a[i - sh + 1 + size], a[i - sh + 1], d1, d0, t, fd)
	} else {
		FT_ADD(a[i - sh + 1 + size], a[i - sh + 1], d1 >> rh, d0 >> rh, t, fd)
		FT_ADD(a[i - sh + size], a[i - sh], d1 << lh, d0 << lh, t, fd)
	}
	if (ra == 0) {
		FT_ADD(a[i - sa + 1 + size], a[i - sa + 1], d1, d0, t, fa)
	} else {
		FT_ADD(a[i - sa + 1 + size], a[i - sa + 1], d1 >> ra, d0 >> ra, t, fa)
		FT_ADD(a[i - sa + size], a[i - sa], d1 << la, d0 << la, t, fa)
	}
	if (rb == 0) {
		FT_ADD(a[i - sb + 1 + size], a[i - sb + 1], d1, d0, t, fb)
	} else {
		FT_ADD(a[i - sb + 1 + size], a[i - sb + 1], d1 >> rb, d0 >> rb, t, fb)
		FT_ADD(a[i - sb + size], a[i - sb], d1 << lb, d0 << lb, t, fb)
	}
	if (rc == 0) {
		FT_ADD(a[i - sc + 1 + size], a[i - sc + 1], d1, d0, t, fc)
	} else {
		FT_ADD(a[i - sc + 1 + size], a[i - sc + 1], d1 >> rc, d0 >> rc, t, fc)
		FT_ADD(a[i - sc + size], a[i - sc], d1 << lc, d0 << lc, t, fc)
	}
	a[i] ^= d0;
	a[i + size] ^= d1;

	dv_copy(c, a, FT_DIGS/2);
	dv_copy(c + FT_DIGS/2, a + size, FT_DIGS/2);
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ft_rdcc_low(dig_t *c, dig_t *a) {
	int fa, fb, fc, fd;

	ft_poly_get_rdc(&fa, &fb, &fc, &fd);

	if (fc == 0) {
		ft_rdct_low(c, a, fa, fb, 3 * FT_DIGS / 2);
	} else {
		ft_rdcp_low(c, a, fa, fb, fc, fd, 3 * FT_DIGS / 2);
	}
}

void ft_rdcm_low(dig_t *c, dig_t *a) {
	int fa, fb, fc, fd;

	ft_poly_get_rdc(&fa, &fb, &fc, &fd);

	if (fc == 0) {
		ft_rdct_low(c, a, fa, fb, FT_DIGS);
	} else {
		ft_rdcp_low(c, a, fa, fb, fc, fd, FT_DIGS);
	}
}
