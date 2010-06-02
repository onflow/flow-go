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
 * Implementation of the ternary field modular reduction.
 *
 * @version $Id$
 * @ingroup ft
 */

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_bn_low.h"
#include "relic_dv.h"
#include "relic_ft.h"
#include "relic_ft_low.h"
#include "relic_util.h"
#include "relic_error.h"

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if FT_RDC == BASIC || !defined(STRIP)

void ft_rdc_mul_basic(ft_t c, dv_t a) {
	int i, j, k, l;
	dig_t *tmpa;
	dv_t r;

	dv_null(r);

	TRY {
		dv_new(r);

		tmpa = a + FT_DIGS;

		for (i = MAX(ft_bits(tmpa), ft_bits(a)) - 1; i >= FT_TRITS; i--) {
			dv_zero(r, 2 * FT_DIGS);
			j = (ft_get_bit(tmpa, i) << 1) | ft_get_bit(a, i);
			SPLIT(k, l, i - FT_TRITS, FT_DIG_LOG);
			r[l + FT_DIGS / 2] = ft_lshb_low(r + l, ft_poly_get(), k);
			r[l + FT_DIGS + FT_DIGS / 2] = ft_lshb_low(r + l + FT_DIGS,
					ft_poly_get() + FT_DIGS / 2, k);
			if (j == 1) {
				ft_subd_low(a, a, r, 2 * FT_DIGS);
			}
			if (j == 2) {
				ft_addd_low(a, a, r, 2 * FT_DIGS);
			}
		}
		dv_copy(c, a, FT_DIGS / 2);
		dv_copy(c + FT_DIGS / 2, a + FT_DIGS, FT_DIGS / 2);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ft_free(r);
	}
}

void ft_rdc_cub_basic(ft_t c, dv_t a) {
	int i, j, k, l;
	dig_t *tmpa;
	dv_t r;

	dv_null(r);

	TRY {
		dv_new(r);
		dv_zero(r, 3 * FT_DIGS);

		tmpa = a + 3 * (FT_DIGS / 2);

		for (i = 3 * FT_TRITS; i >= FT_TRITS; i--) {
			j = (ft_get_bit(tmpa, i) << 1) | ft_get_bit(a, i);
			if (j > 0) {
				SPLIT(k, l, i - FT_TRITS, FT_DIG_LOG);
				r[l + FT_DIGS / 2] = ft_lshb_low(r + l, ft_poly_get(), k);
				r[l + 3 * (FT_DIGS / 2) + FT_DIGS / 2] =
						ft_lshb_low(r + l + 3 * FT_DIGS / 2,
						ft_poly_get() + FT_DIGS / 2, k);
				if (j == 1) {
					ft_subd_low(a, a, r, 3 * FT_DIGS);
				}
				if (j == 2) {
					ft_addd_low(a, a, r, 3 * FT_DIGS);
				}
			}
		}
		dv_copy(c, a, FT_DIGS / 2);
		dv_copy(c + FT_DIGS / 2, a + 3 * FT_DIGS / 2, FT_DIGS / 2);
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		ft_free(r);
	}
}

#endif

#if FT_RDC == QUICK || !defined(STRIP)

void ft_rdc_mul_quick(ft_t c, dv_t a) {
	ft_rdcm_low(c, a);
}

void ft_rdc_cub_quick(ft_t c, dv_t a) {
	ft_rdcc_low(c, a);
}

#endif
