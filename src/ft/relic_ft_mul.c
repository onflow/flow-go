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
 * Implementation of the ternary field multiplication functions.
 *
 * @version $Id$
 * @ingroup ft
 */

#include <string.h>

#include "relic_core.h"
#include "relic_conf.h"
#include "relic_ft.h"
#include "relic_ft_low.h"
#include "relic_bn_low.h"
#include "relic_util.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

#if FT_MUL == BASIC || !defined(STRIP)

void ft_mul_basic(ft_t c, ft_t a, ft_t b) {
	int i, j;
	dv_t s;
	ft_t t, u;

	dv_null(s);
	ft_null(t);
	ft_null(u);

	TRY {
		/* We need a temporary variable so that c can be a or b. */
		dv_new(s);
		ft_new(t);
		ft_new(u);

		ft_zero(t);
		ft_copy(u, b);
		dv_zero(s + 2 * FT_DIGS, FT_DIGS);

		j = ft_get_trit(a, 0);
		if (j == 1) {
			ft_copy(t, b);
		}
		if (j == 2) {
			ft_neg(t, b);
		}
		for (i = 1; i < FT_TRITS; i++) {
			dv_copy(s, u, FT_DIGS / 2);
			dv_zero(s + FT_DIGS / 2, FT_DIGS / 2);
			dv_copy(s + FT_DIGS, u + FT_DIGS / 2, FT_DIGS / 2);
			dv_zero(s + FT_DIGS / 2 + FT_DIGS, FT_DIGS / 2);

			/* We are already shifting a temporary value, so this is more efficient
			 * than calling ft_lsh(). */
			ft_lsh1_low(s, s);
			ft_lsh1_low(s + FT_DIGS, s + FT_DIGS);
			ft_rdc_mul(u, s);
			j = ft_get_trit(a, i);
			if (j == 1) {
				ft_add(t, t, u);
			}
			if (j == 2) {
				ft_sub(t, t, u);
			}
		}

		if (ft_trits(t) > FT_TRITS) {
			j = ft_get_trit(t, FT_TRITS);
			if (j == 1) {
				ft_poly_sub(c, t);
			}
			if (j == 2) {
				ft_poly_add(c, t);
			}
		} else {
			ft_copy(c, t);
		}
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(s);
		ft_free(t);
		ft_free(u);
	}
}

#endif

#if FT_MUL == LODAH || !defined(STRIP)

void ft_mul_lodah(ft_t c, ft_t a, ft_t b) {
	dv_t t;

	dv_null(t);

	TRY {
		dv_new(t);

		ft_muln_low(t, a, b);

		ft_rdc_mul(c, t);
	} CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		dv_free(t);
	}
}

#endif

#if FT_MUL == INTEG || !defined(STRIP)

void ft_mul_integ(ft_t c, ft_t a, ft_t b) {
	ft_mulm_low(c, a, b);
}

#endif
