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
 * Implementation of the low-level ternary field cube root.
 *
 * @version $Id$
 * @ingroup ft
 */

#include <stdlib.h>

#include "relic_ft.h"
#include "relic_ft_low.h"
#include "relic_core.h"
#include "relic_error.h"

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

static void ft_crtf_low(dig_t *c, dig_t *a, int *crz, int *srz, int clen,
		int slen) {
	int i, j, k, l;
	align dig_t t[2 * FT_DIGS], r[2 * FT_DIGS], t0[FT_DIGS], t1[FT_DIGS],
			t2[FT_DIGS];

	dv_zero(t0, FT_DIGS);
	dv_zero(t1, FT_DIGS);
	dv_zero(t2, FT_DIGS);
	dv_zero(t, 2 * FT_DIGS);

	for (i = 0; i < (FT_TRITS - (FT_TRITS % 3)) / 3; i++) {
		j = ft_get_trit(a, 3 * i);
		ft_set_trit(t0, i, j);
		j = ft_get_trit(a, 3 * i + 1);
		ft_set_trit(t1, i, j);
		j = ft_get_trit(a, 3 * i + 2);
		ft_set_trit(t2, i, j);
	}
	if (FT_TRITS % 3 >= 1) {
		j = ft_get_trit(a, 3 * i);
		ft_set_trit(t0, i, j);
	}
	if (FT_TRITS % 3 == 2) {
		j = ft_get_trit(a, 3 * i + 1);
		ft_set_trit(t1, i, j);
	}

	dv_copy(t, t0, FT_DIGS / 2);
	dv_copy(t + FT_DIGS, t0 + FT_DIGS / 2, FT_DIGS / 2);
	for (i = 0; i < clen; i++) {
		dv_zero(r, 2 * FT_DIGS);
		if (crz[i] < 0) {
			SPLIT(k, l, -crz[i], FT_DIG_LOG);
		} else {
			SPLIT(k, l, crz[i], FT_DIG_LOG);
		}
		r[l + FT_DIGS / 2] = ft_lshb_low(r + l, t1, k);
		r[l + FT_DIGS + FT_DIGS / 2] = ft_lshb_low(r + l + FT_DIGS,
				t1 + FT_DIGS / 2, k);
		if (crz[i] < 0) {
			ft_subd_low(t, t, r, 2 * FT_DIGS, 2 * FT_DIGS);
		} else {
			ft_addd_low(t, t, r, 2 * FT_DIGS, 2 * FT_DIGS);
		}
	}
	for (i = 0; i < slen; i++) {
		dv_zero(r, 2 * FT_DIGS);
		if (srz[i] < 0) {
			SPLIT(k, l, -srz[i], FT_DIG_LOG);
		} else {
			SPLIT(k, l, srz[i], FT_DIG_LOG);
		}
		r[l + FT_DIGS / 2] = ft_lshb_low(r + l, t2, k);
		r[l + FT_DIGS + FT_DIGS / 2] = ft_lshb_low(r + l + FT_DIGS,
				t2 + FT_DIGS / 2, k);
		if (srz[i] < 0) {
			ft_subd_low(t, t, r, 2 * FT_DIGS, 2 * FT_DIGS);
		} else {
			ft_addd_low(t, t, r, 2 * FT_DIGS, 2 * FT_DIGS);
		}
	}
	ft_rdcm_low(c, t);
}

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void ft_crtn_low(dig_t *c, dig_t *a) {
	int *crz, *srz, clen, slen;

	crz = ft_poly_get_crz_sps(&clen);
	srz = ft_poly_get_srz_sps(&slen);

	if (clen != 0 && slen != 0) {
		ft_crtf_low(c, a, crz, srz, clen, slen);
	} else {
		THROW(ERR_INVALID);
	}
}
